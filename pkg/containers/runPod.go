package containers

import (
	"context"
	"fmt"

	"log"
	"strings"


	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)


// CreateOrUpdateRunPod creates or updates a Kubernetes Pod that runs a script with specified environment variables and image.
func CreateOrUpdateRunPod(clientset kubernetes.Interface, name, namespace, scriptName string, envVars map[string]string, taggedImageName, imagePullSecretName, service string) (string, error) {
	identifier := fmt.Sprintf("%s-%s", name, service)
	podName := fmt.Sprintf("%s-%s-docker-run-pod", name, service)

	// Generate environment variables
	env := []v1.EnvVar{}
	for key, value := range envVars {
		env = append(env, v1.EnvVar{
			Name:  key,
			Value: value,
		})
		log.Printf("Setting environment variable %s=%s", key, value)
	}

	// Ensure the scriptName starts with "./"
	if !strings.HasPrefix(scriptName, "./") {
		scriptName = "./" + scriptName
	}

	// Split the scriptName into script and args
	parts := strings.SplitN(scriptName, " ", 2)
	script := parts[0]
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	env = append(env, v1.EnvVar{
		Name:  "SCRIPT",
		Value: script,
	})

	if args != "" {
		env = append(env, v1.EnvVar{
			Name:  "ARGS",
			Value: args,
		})
	}

	// Define the pod spec
	podSpec := v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:            "terraform",
				Image:           taggedImageName,
				ImagePullPolicy: v1.PullAlways,
				Env:             env,
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      "workspace",
						MountPath: "/workspace",
					},
				},
			},
		},
		RestartPolicy: v1.RestartPolicyNever,
		Volumes: []v1.Volume{
			{
				Name: "workspace",
				VolumeSource: v1.VolumeSource{
					EmptyDir: &v1.EmptyDirVolumeSource{},
				},
			},
		},
		ImagePullSecrets: []v1.LocalObjectReference{
			{
				Name: imagePullSecretName,
			},
		},
	}

	// Define the pod object
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"apprun": identifier,
			},
		},
		Spec: podSpec,
	}

	// Try to get the existing pod
	existingPod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err == nil {
		// Pod already exists, remove finalizers and delete it
		log.Printf("Pod %s already exists. Removing finalizers and recreating.", podName)

		// Remove finalizers if any
		if len(existingPod.ObjectMeta.Finalizers) > 0 {
			existingPod.ObjectMeta.Finalizers = nil
			_, err = clientset.CoreV1().Pods(namespace).Update(context.Background(), existingPod, metav1.UpdateOptions{})
			if err != nil {
				log.Printf("Failed to remove finalizers from Pod: %v", err)
				return "", err
			}
			log.Printf("Removed finalizers from Pod: %s", existingPod.Name)
		}

		// Delete the existing pod
		err = clientset.CoreV1().Pods(namespace).Delete(context.Background(), existingPod.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Printf("Failed to delete Pod: %v", err)
			return "", err
		}
		log.Printf("Deleted Pod: %s", existingPod.Name)
	} else if !apierrors.IsNotFound(err) {
		// If the error is something other than NotFound, log and return it
		log.Printf("Failed to get existing Pod: %v", err)
		return "", err
	}

	// Create the pod with the new spec
	log.Printf("Creating Pod in namespace: %s with image: %s", namespace, taggedImageName)
	_, err = clientset.CoreV1().Pods(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create Pod: %v", err)
		return "", err
	}

	log.Println("Pod created successfully.")
	return podName, nil
}



