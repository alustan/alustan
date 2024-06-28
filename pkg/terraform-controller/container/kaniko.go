package container

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	containers "github.com/alustan/pkg/containers"
)


// CreateBuildPod creates a Kubernetes Pod to run a Kaniko build.
func CreateBuildPod(clientset *kubernetes.Clientset, name, namespace, configMapName, imageName, dockerSecretName, repoDir, gitRepo, branch, sshKey, pvcName string) (string, string, error) {

	labelSelector := fmt.Sprintf("appbuild=%s", name)
	
	// Check for existing pods with the same label
	exists, err := containers.CheckExistingPods(clientset, namespace, labelSelector)
	if err != nil {
		log.Printf("Error checking existing pods: %v", err)
		return "", "", err
	}

	if exists {
		log.Printf("Existing pods with label %s found, not creating new pod.", labelSelector)
		return "", "", fmt.Errorf("existing build pod already running")
	}

	// Generate a unique pod name using the current timestamp
	timestamp := time.Now().Format("20060102150405")
	podName := fmt.Sprintf("%s-docker-build-pod-%s", name, timestamp)

	// Generate a unique tag using the current timestamp
	taggedImageName := fmt.Sprintf("%s:%s", imageName, timestamp)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"appbuild": name,
			},
			Annotations: map[string]string{
				"kubectl.kubernetes.io/ttl": "7200", // TTL in seconds (2 hrs)
			},
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name:  "git-clone",
					Image: "docker.io/alustan/git-clone:0.4.0",
					Env: []corev1.EnvVar{
						{
							Name:  "REPO_URL",
							Value: gitRepo,
						},
						{
							Name:  "BRANCH",
							Value: branch,
						},
						{
							Name:  "REPO_DIR",
							Value: repoDir,
						},
						{
							Name:  "SSH_KEY",
							Value: sshKey,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "workspace",
							MountPath: "/workspace",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "kaniko",
					Image: "gcr.io/kaniko-project/executor:v1.23.1-debug",
					Args: []string{
						"--dockerfile=/workspace/tmp/" + name + "/Dockerfile",
						"--destination=" + taggedImageName,
						"--context=/workspace/tmp/" + name,
					},
					Env: []corev1.EnvVar{
						{
							Name:  "DOCKER_CONFIG",
							Value: "/root/.docker",
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "workspace",
							MountPath: "/workspace",
						},
						{
							Name:      "docker-credentials",
							MountPath: "/root/.docker",
						},
						{
							Name:      "dockerfile-config",
							MountPath: "/workspace/tmp/" + name + "/Dockerfile",
							SubPath:   "Dockerfile",
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes: []corev1.Volume{
				{
					Name: "workspace",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
				{
					Name: "docker-credentials",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: dockerSecretName,
							Items: []corev1.KeyToPath{
								{
									Key:  ".dockerconfigjson",
									Path: "config.json",
								},
							},
						},
					},
				},
				{
					Name: "dockerfile-config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: configMapName,
							},
							Items: []corev1.KeyToPath{
								{
									Key:  "Dockerfile",
									Path: "Dockerfile",
								},
							},
						},
					},
				},
			},
		},
	}

	// Create the pod
	_, err = clientset.CoreV1().Pods(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Failed to create Pod: %v", err)
		return "", "", err
	}

	log.Printf("Created Pod: %s", podName)
	log.Printf("Image will be pushed with tag: %s", taggedImageName)
	return taggedImageName, podName, nil
}
