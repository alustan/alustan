package containers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	CoreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ExtractPostDeployOutput retrieves and parses the outputs from a pod's log
func ExtractPostDeployOutput(clientset kubernetes.Interface, namespace, podName string) (map[string]interface{}, error) {
	for {
		// Retrieve the current state of the pod
		pod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		// Log the current pod phase
		log.Printf("Pod %s is in phase %s", podName, pod.Status.Phase)

		// Check if the pod has succeeded
		if pod.Status.Phase == CoreV1.PodSucceeded {
			log.Printf("Pod %s has succeeded", podName)
			break
		}

		// Check if the pod has failed
		if pod.Status.Phase == CoreV1.PodFailed {
			log.Printf("Pod %s has failed", podName)
			return nil, fmt.Errorf("pod %s failed", podName)
		}

		// Sleep for 1 minute before checking again
		time.Sleep(1 * time.Minute)
	}

	// Retrieve the pod logs
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &CoreV1.PodLogOptions{})
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

	// Log the final outputs
	log.Printf("Final Outputs: %+v", outputs)

	return outputs, nil
}
