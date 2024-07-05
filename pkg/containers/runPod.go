package containers

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateOrUpdateRunJob creates or updates a Kubernetes Job that runs a script with specified environment variables and image.
func CreateOrUpdateRunJob(clientset kubernetes.Interface, name, namespace, scriptName string, envVars map[string]string, taggedImageName, imagePullSecretName, service string) (string, error) {
    identifier := fmt.Sprintf("%s-%s", name, service)
    labelSelector := fmt.Sprintf("apprun=%s", identifier)

    // Check for existing jobs with the same label
    exists, existingJobName, err := CheckExistingJobs(clientset, namespace, labelSelector)
    if err != nil {
        log.Printf("Error checking existing jobs: %v", err)
        return "", err
    }

    // Generate a unique job name using the current timestamp if a new job is created
    timestamp := time.Now().Format("20060102150405")
    jobName := fmt.Sprintf("%s-%s-docker-run-job-%s", name, service, timestamp)

    log.Printf("Creating or updating Job in namespace: %s with image: %s", namespace, taggedImageName)

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

    // Define the job spec
    job := &batchv1.Job{
        ObjectMeta: metav1.ObjectMeta{
            Name: jobName,
            Labels: map[string]string{
                "apprun": identifier,
            },
            Annotations: map[string]string{
                "kubectl.kubernetes.io/ttl": "3600", // TTL in seconds (1 hour)
            },
        },
        Spec: batchv1.JobSpec{
            Template: v1.PodTemplateSpec{
                Spec: v1.PodSpec{
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
                },
            },
        },
    }

    if exists {
        log.Printf("Existing job with label %s found: %s. Updating Pods.", labelSelector, existingJobName)

        // Get the existing job
        existingJob, err := clientset.BatchV1().Jobs(namespace).Get(context.Background(), existingJobName, metav1.GetOptions{})
        if err != nil {
            log.Printf("Failed to get existing Job: %v", err)
            return "", err
        }

        // Update the template of the existing job
        existingJob.Spec.Template = job.Spec.Template

        // List the pods managed by the job
        podList, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
            LabelSelector: labelSelector,
        })
        if err != nil {
            log.Printf("Failed to list Pods for Job: %v", err)
            return "", err
        }

        // Delete the pods to force the job to recreate them with the updated template
        for _, pod := range podList.Items {
            err := clientset.CoreV1().Pods(namespace).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
            if err != nil {
                log.Printf("Failed to delete Pod: %v", err)
                return "", err
            }
            log.Printf("Deleted Pod: %s", pod.Name)
        }

        log.Println("Pods deleted successfully, Job will recreate them with the updated template.")
        return existingJobName, nil
    } else {
        log.Println("Creating new Job...")
        _, err := clientset.BatchV1().Jobs(namespace).Create(context.Background(), job, metav1.CreateOptions{})
        if err != nil {
            log.Printf("Failed to create Job: %v", err)
            return "", err
        }
        log.Println("Job created successfully.")
        return jobName, nil
    }
}

// WaitForJobCompletion waits for the job to complete and retrieves the Terraform output from the associated pod.
func WaitForJobCompletion(clientset kubernetes.Interface, namespace, jobName string) (map[string]string, error) {
    for {
        // Retrieve the current state of the job
        job, err := clientset.BatchV1().Jobs(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
        if err != nil {
            return nil, err
        }
        // Check if the job has succeeded
        if job.Status.Succeeded > 0 {
            break
        }
        // Check if the job has failed
        if job.Status.Failed > 0 {
            return nil, fmt.Errorf("job %s failed", jobName)
        }
        // Sleep for 1 minute before checking again
        time.Sleep(1 * time.Minute)
    }

    // List all pods with a label matching the job name
    podList, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
        LabelSelector: fmt.Sprintf("job-name=%s", jobName),
    })
    if err != nil {
        return nil, err
    }

    // Ensure there is at least one pod associated with the job
    if len(podList.Items) == 0 {
        return nil, fmt.Errorf("no pods found for job %s", jobName)
    }

    // Retrieve the name of the first pod in the list
    podName := podList.Items[0].Name
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

