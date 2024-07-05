package containers

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/client-go/kubernetes"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// CreateOrUpdateRunPod creates or updates a Kubernetes Pod that runs a script with specified environment variables and image.
func CreateOrUpdateRunPod(clientset kubernetes.Interface, name, namespace, scriptName string, envVars map[string]string, taggedImageName, imagePullSecretName, service string) (string, error) {
	identifier := fmt.Sprintf("%s-%s", name, service)
	

	// Generate a consistent pod name
	podName := fmt.Sprintf("%s-%s-docker-run-pod", name, service)

	// Generate the environment variables
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
	newPodSpec := v1.PodSpec{
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

	// Check for existing pods with the same name
	existingPod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err == nil {
		// If the existing pod spec differs from the new spec, delete and recreate the pod
		if !equality.Semantic.DeepEqual(existingPod.Spec, newPodSpec) {
			log.Printf("Existing Pod with name %s found. Pod spec has changed. Deleting and recreating.", podName)

			// Delete the existing pod
			err := clientset.CoreV1().Pods(namespace).Delete(context.Background(), existingPod.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Printf("Failed to delete Pod: %v", err)
				return "", err
			}
			log.Printf("Deleted Pod: %s", existingPod.Name)
		} else {
			log.Printf("Existing Pod with name %s found. Pod spec has not changed.", podName)
			return podName, nil
		}
	} else if !apierrors.IsNotFound(err) {
		return "", err
	}

	log.Printf("Creating Pod in namespace: %s with image: %s", namespace, taggedImageName)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"apprun": identifier,
			},
		},
		Spec: newPodSpec,
	}

	log.Println("Creating new Pod...")
	_, err = clientset.CoreV1().Pods(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create Pod: %v", err)
		return "", err
	}
	log.Println("Pod created successfully.")
	return podName, nil
}

// WaitForPodCompletion waits for the pod to complete and retrieves the Terraform output from the associated pod.
func WaitForPodCompletion(clientset kubernetes.Interface, namespace, podName string) (map[string]string, error) {
	for {
		// Retrieve the current state of the pod
		pod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		// Log the current pod phase
		log.Printf("Pod %s is in phase %s", podName, pod.Status.Phase)

		// Check if the pod has succeeded
		if pod.Status.Phase == v1.PodSucceeded {
			log.Printf("Pod %s has succeeded", podName)
			break
		}

		// Check if the pod has failed
		if pod.Status.Phase == v1.PodFailed {
			log.Printf("Pod %s has failed", podName)
			return nil, fmt.Errorf("pod %s failed", podName)
		}

		// Sleep for 1 minute before checking again
		time.Sleep(1 * time.Minute)
	}

	// Fetch the logs from the pod
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{})
	logs, err := req.Stream(context.Background())
	if err != nil {
		return nil, err
	}
	defer logs.Close()

	// Read the logs into a byte array
	logsBytes, err := io.ReadAll(logs)
	if err != nil {
		return nil, err
	}

	// Convert the logs to a string and split into lines
	logsString := string(logsBytes)
	lines := strings.Split(logsString, "\n")

	outputSection := false
	outputs := make(map[string]string)

	// Parse the logs to extract the "Outputs:" section
	for _, line := range lines {
		if strings.HasPrefix(line, "Outputs:") {
			outputSection = true
			continue
		}

		if outputSection {
			if strings.TrimSpace(line) == "" {
				break
			}

			// Extract key-value pairs from the "Outputs:" section
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				// Remove quotes from value if present
				if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
					value = value[1 : len(value)-1]
				}
				outputs[key] = value
			}
		}
	}

	// Return the extracted outputs
	return outputs, nil
}