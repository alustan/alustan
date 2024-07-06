package containers

import (
	"context"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// WaitForPodCompletion waits for the pod to complete and retrieves the Terraform output from the associated pod.
func WaitForPodCompletion(clientset kubernetes.Interface, namespace, podName string) (map[string]interface{}, error) {
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
	outputs := make(map[string]interface{})

	// Parse the logs to extract the "Outputs:" section
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

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

				// Try to parse value as int, float, bool, or fallback to string
				if intValue, err := strconv.Atoi(value); err == nil {
					outputs[key] = intValue
				} else if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
					outputs[key] = floatValue
				} else if boolValue, err := strconv.ParseBool(value); err == nil {
					outputs[key] = boolValue
				} else {
					outputs[key] = value
				}
			}
		}
	}

	// Log the final outputs
	log.Printf("Final Outputs: %+v", outputs)

	// Return the extracted outputs
	return outputs, nil
}
