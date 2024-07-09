package containers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// WaitForPodCompletion waits for the pod to complete and retrieves the Terraform output from the associated pod.
func WaitForPodCompletion(logger *zap.SugaredLogger, clientset kubernetes.Interface, namespace, podName string) (map[string]interface{}, error) {
	for {
		// Retrieve the current state of the pod
		pod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		// Log the current pod phase
		logger.Infof("Pod %s is in phase %s", podName, pod.Status.Phase)

		// Check if the pod has succeeded
		if pod.Status.Phase == v1.PodSucceeded {
			logger.Infof("Pod %s has succeeded", podName)
			break
		}

		// Check if the pod has failed
		if pod.Status.Phase == v1.PodFailed {
			logger.Infof("Pod %s has failed", podName)
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

	// Convert the logs to a string and remove ANSI escape codes
	logsString := removeANSIEscapeCodes(string(logsBytes))
	logger.Infof("Raw Pod Logs: %s", logsString) // Log the raw pod logs for debugging

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
			if line == "" {
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

				var parsedValue interface{}
				
				// Try to parse value as int, float, bool, or fallback to string
				if intValue, err := strconv.Atoi(value); err == nil {
					parsedValue = intValue
				} else if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
					parsedValue = floatValue
				} else if boolValue, err := strconv.ParseBool(value); err == nil {
					parsedValue = boolValue
				} else {
					parsedValue = value
				}

				// Log the type and value for debugging
				logger.Infof("Output Key: %s, Value: %v, Type: %T", key, parsedValue, parsedValue)

				outputs[key] = parsedValue
			}
		}
	}

	// Log the final outputs
	logger.Infof("Final Outputs: %+v", outputs)

	// Attempt to marshal the outputs to JSON to catch any errors
	outputsJSON, err := json.Marshal(outputs)
	if err != nil {
		logger.Errorf("Error marshaling outputs to JSON: %v", err)
		return nil, fmt.Errorf("error marshaling outputs to JSON: %v", err)
	}

	logger.Infof("Outputs JSON: %s", string(outputsJSON))

	// Return the extracted outputs
	return outputs, nil
}

// removeANSIEscapeCodes removes ANSI escape codes from a string
func removeANSIEscapeCodes(input string) string {
	ansiEscapeCodes := regexp.MustCompile(`\x1B\[[0-9;]*[a-zA-Z]`)
	return ansiEscapeCodes.ReplaceAllString(input, "")
}
