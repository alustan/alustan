package containers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ExtractPostDeployOutput retrieves and parses the outputs from a pod's log
func ExtractPostDeployOutput(clientset kubernetes.Interface, namespace, jobName string) (map[string]interface{}, error) {
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


	// Retrieve the pod logs
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{})
	logs, err := req.Stream(context.Background())
	if err != nil {
		return nil, err
	}
	defer logs.Close()

	// Read the logs
	logsBytes, err := io.ReadAll(logs)
	if err != nil {
		return nil, err
	}

	// Unmarshal the logs into a generic map
	var logOutput map[string]interface{}
	if err := json.Unmarshal(logsBytes, &logOutput); err != nil {
		return nil, err
	}

	// Extract the "outputs" field
	outputs, ok := logOutput["outputs"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("outputs field not found or invalid format")
	}

	return outputs, nil
}
