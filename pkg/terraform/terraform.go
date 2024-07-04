package terraform

import (

	"fmt"
	"log"
	"strings"
	

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"

	kubernetespkg "github.com/alustan/pkg/kubernetes"
	"github.com/alustan/pkg/util"
	containers "github.com/alustan/pkg/containers"
	"github.com/alustan/pkg/schematypes"
)

const (
	maxRetries = 5
)

func GetScriptContent(
	observed schematypes.SyncRequest,
	updateStatus func(observed schematypes.SyncRequest, status map[string]interface{}) error,
) (string, map[string]interface{}) {
	var scriptContent string
	if observed.Finalizing {
		if observed.Parent.Spec.Scripts.Destroy != "" {
			scriptContent = observed.Parent.Spec.Scripts.Destroy
		} else {
			scriptContent = ""
		}
	} else {
		scriptContent = observed.Parent.Spec.Scripts.Deploy
		if scriptContent == "" {
			status := util.ErrorResponse("executing script", fmt.Errorf("deploy script is missing"))
			err := updateStatus(observed, status)
			if err != nil {
				log.Printf("Failed to update status: %v", err)
			} else {
				log.Printf("Successfully updated status: %v", status)
			}

			return "", status
		}
	}

	return scriptContent, nil
}

func ExecuteTerraform(
	clientset kubernetes.Interface,
	observed schematypes.SyncRequest,
	scriptContent, taggedImageName, secretName string,
	envVars map[string]string,
	updateStatus func(observed schematypes.SyncRequest, status map[string]interface{}) error,
) map[string]interface{} {

	if observed.Finalizing {
		err := updateStatus(observed, map[string]interface{}{
			"state":   "Progressing",
			"message": "Running Terraform Destroy",
		})
		if err != nil {
			log.Printf("Failed to update status: %v", err)
		} else {
			log.Printf("Successfully updated status: %v", map[string]interface{}{
				"state":   "Progressing",
				"message": "Running Terraform Destroy",
			})
		}

		status := runDestroy(clientset, observed, scriptContent, taggedImageName, secretName, envVars)

		err = updateStatus(observed, status)
		if err != nil {
			log.Printf("Failed to update status: %v", err)
		} else {
			log.Printf("Successfully updated status: %v", status)
		}

		if status["state"] == "Success" {
			finalStatus := map[string]interface{}{
				"state":     "Completed",
				"message":   "Destroy process completed successfully",
				"finalized": true,
			}

			err = updateStatus(observed, finalStatus)
			if err != nil {
				log.Printf("Failed to update status: %v", err)
			} else {
				log.Printf("Successfully updated status: %v", finalStatus)
			}
			return finalStatus
		}

		return status
	}

	err := updateStatus(observed, map[string]interface{}{
		"state":   "Progressing",
		"message": "Running Terraform Apply",
	})
	if err != nil {
		log.Printf("Failed to update status: %v", err)
	} else {
		log.Printf("Successfully updated status: %v", map[string]interface{}{
			"state":   "Progressing",
			"message": "Running Terraform Apply",
		})
	}

	status := runApply(clientset, observed, scriptContent, taggedImageName, secretName, envVars)

	err = updateStatus(observed, status)
	if err != nil {
		log.Printf("Failed to update status: %v", err)
	} else {
		log.Printf("Successfully updated status: %v", status)
	}

	if status["state"] == "Failed" {
		return status
	}

	if observed.Parent.Spec.PostDeploy.Script != "" {

		err := updateStatus(observed, map[string]interface{}{
			"state":   "Progressing",
			"message": "Running postDeploy script",
		})
		if err != nil {
			log.Printf("Failed to update status: %v", err)
		} else {
			log.Printf("Successfully updated status: %v", map[string]interface{}{
				"state":   "Progressing",
				"message": "Running postDeploy script",
			})
		}

		postDeployOutput, err := runPostDeploy(clientset, observed.Parent.Metadata.Name, observed.Parent.Metadata.Namespace, observed.Parent.Spec.PostDeploy, envVars, taggedImageName, secretName)
		if err != nil {
			status := util.ErrorResponse("executing postDeploy script", err)

			err := updateStatus(observed, status)
			if err != nil {
				log.Printf("Failed to update status: %v", err)
			} else {
				log.Printf("Successfully updated status: %v", status)
			}
			return status
		}

		finalStatus := map[string]interface{}{
			"state":            "Completed",
			"message":          "Processing completed successfully",
			"postDeployOutput": postDeployOutput,
		}

		err = updateStatus(observed, finalStatus)
		if err != nil {
			log.Printf("Failed to update status: %v", err)
		} else {
			log.Printf("Successfully updated status: %v", finalStatus)
		}

		return finalStatus
	}

	finalStatus := map[string]interface{}{
		"state":   "Completed",
		"message": "Processing completed successfully",
	}

	err = updateStatus(observed, finalStatus)
	if err != nil {
		log.Printf("Failed to update status: %v", err)
	} else {
		log.Printf("Successfully updated status: %v", finalStatus)
	}

	return finalStatus
}

func runApply(
	clientset kubernetes.Interface,
	observed schematypes.SyncRequest,
	scriptContent, taggedImageName, secretName string,
	envVars map[string]string,
) map[string]interface{} {
	var terraformErr error
	var podName string

	err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		log.Printf("Error occurred: %v", err)
		return strings.Contains(err.Error(), "timeout") // Retry only on timeout errors
	}, func() error {
		podName, terraformErr = containers.CreateRunPod(clientset, observed.Parent.Metadata.Name, observed.Parent.Metadata.Namespace, scriptContent, envVars, taggedImageName, secretName)
		return terraformErr
	})

	status := map[string]interface{}{
		"state":   "Success",
		"message": "Terraform applied successfully",
	}
	if err != nil {
		status["state"] = "Failed"
		status["message"] = terraformErr.Error()
		return status
	}

	// Wait for the pod to complete and retrieve the logs
	output, err := containers.WaitForPodCompletion(clientset, observed.Parent.Metadata.Namespace, podName)
	if err != nil {
		status["state"] = "Failed"
		status["message"] = fmt.Sprintf("Error retrieving Terraform output: %v", err)
		return status
	}

	status["output"] = output

	// Retrieve ingress URLs and include them in the status
	ingressURLs, err := kubernetespkg.GetAllIngressURLs(clientset)
	if err != nil {
		status["state"] = "Failed"
		status["message"] = fmt.Sprintf("Error retrieving Ingress URLs: %v", err)
		return status
	}
	status["ingressURLs"] = ingressURLs

	// Retrieve credentials and include them in the status
	credentials, err := kubernetespkg.FetchCredentials(clientset)
	if err != nil {
		status["state"] = "Failed"
		status["message"] = fmt.Sprintf("Error retrieving credentials: %v", err)
		return status
	}
	status["credentials"] = credentials

	return status
}

func runDestroy(
	clientset kubernetes.Interface,
	observed schematypes.SyncRequest,
	scriptContent, taggedImageName, secretName string,
	envVars map[string]string,
) map[string]interface{} {
	// Check if scriptContent is empty
	if scriptContent == "" {
		return map[string]interface{}{
			"state":   "Success",
			"message": "No destroy script specified",
		}
	}

	// Call to run Terraform destroy
	var terraformErr error

	err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		
		log.Printf("Error occurred: %v", err)
		return strings.Contains(err.Error(), "timeout") // Retry only on timeout errors
	}, func() error {
		_, terraformErr = containers.CreateRunPod(clientset, observed.Parent.Metadata.Name, observed.Parent.Metadata.Namespace, scriptContent, envVars, taggedImageName, secretName)
		return terraformErr
	})

	status := map[string]interface{}{
		"state":   "Success",
		"message": "Terraform destroyed successfully",
	}
	if err != nil {
		status["state"] = "Failed"
		status["message"] = terraformErr.Error()
		return status
	}

	return status
}

func runPostDeploy(
	clientset kubernetes.Interface,
	name,
	namespace string,
	postDeploy schematypes.PostDeploy,
	envVars map[string]string,
	image, secretName string,
) (map[string]interface{}, error) { 
	// Ensure the script path starts with ./
	scriptPath := postDeploy.Script
	if !strings.HasPrefix(scriptPath, "./") {
		scriptPath = "./" + scriptPath
	}

	args := make([]string, 0, len(postDeploy.Args))
	for flag, envVarKey := range postDeploy.Args {
		value, ok := envVars[envVarKey]
		if !ok {
			return nil, fmt.Errorf("environment variable %s not found", envVarKey)
		}
		args = append(args, fmt.Sprintf("-%s=%s", flag, value))
	}

	// Concatenate the script and args into a single command string
	command := fmt.Sprintf("%s %s", scriptPath, strings.Join(args, " "))

	// Print the constructed command for debugging
	fmt.Println("Command:", command)

	// Create the pod to run the post-deploy script
	podName, err := containers.CreateRunPod(clientset, name, namespace, command, envVars, image, secretName)
	if err != nil {
		return nil, fmt.Errorf("failed to create post-deploy pod: %v", err)
	}

	// Wait for the pod to complete and retrieve the logs
	output, err := containers.ExtractPostDeployOutput(clientset, namespace, podName)
	if err != nil {
		return nil, fmt.Errorf("error executing postDeploy script: %v", err)
	}

	return output, nil
}

