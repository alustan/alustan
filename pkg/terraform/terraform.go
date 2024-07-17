package terraform

import (
	"fmt"
	"encoding/json"
	"strings"
	"context"
	"time"
	
     
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubernetesPkg "github.com/alustan/pkg/kubernetes"
	"github.com/alustan/pkg/util"
	"github.com/alustan/pkg/containers"
	"github.com/alustan/api/v1alpha1"

)

func GetScriptContent(logger *zap.SugaredLogger, observed *v1alpha1.Terraform, finalizing bool) (string, v1alpha1.TerraformStatus) {
	var scriptContent string
	var status v1alpha1.TerraformStatus

	if finalizing {
		if observed.Spec.Scripts.Destroy != "" {
			scriptContent = observed.Spec.Scripts.Destroy
		} else {
			scriptContent = ""
		}
	} else {
		scriptContent = observed.Spec.Scripts.Deploy
		if scriptContent == "" {
			status = util.ErrorResponse(logger,"executing script", fmt.Errorf("deploy script is missing"))
			return "", status
		}
	}

	return scriptContent, status
}

func ExecuteTerraform(
	logger *zap.SugaredLogger,
	clientset kubernetes.Interface,
	dynamicClient dynamic.Interface,
	observed *v1alpha1.Terraform,
	scriptContent, taggedImageName, secretName string,
	envVars map[string]string,
	finalizing bool,
) v1alpha1.TerraformStatus {

	var status v1alpha1.TerraformStatus

	if finalizing {

		logger.Info("Attempting to destroy provisioned resources")
		
        status = runDestroy(logger, clientset, dynamicClient, observed, scriptContent, taggedImageName, secretName, envVars)

		return status
	}

	status = v1alpha1.TerraformStatus{
		State:   "Progressing",
		Message: "Running Terraform Apply",
	}

	status = runApply(logger, clientset, observed, scriptContent, taggedImageName, secretName, envVars)

	// Preserve any existing status fields in the TerraformStatus struct
	finalStatus := v1alpha1.TerraformStatus{
		State:       status.State,
		Message:     status.Message,
	}

	if status.State == "Failed" {
		return finalStatus
	}

	if observed.Spec.PostDeploy.Script != "" {
		finalStatus.State = "Progressing"
		finalStatus.Message = "Running postDeploy script"

		postDeployOutput, err := runPostDeploy(logger, clientset, observed.ObjectMeta.Name, observed.ObjectMeta.Namespace, observed.Spec.PostDeploy, envVars, taggedImageName, secretName)
		if err != nil {
			return util.ErrorResponse(logger, "executing postDeploy script", err)
		}

		finalStatus.State = "Completed"
		finalStatus.Message = "Processing completed successfully"
		finalStatus.PostDeployOutput = postDeployOutput
	} else {
		finalStatus.State = "Completed"
		finalStatus.Message = "Processing completed successfully"
	}

	return finalStatus
}

func runApply(
	logger *zap.SugaredLogger,
	clientset kubernetes.Interface,
	observed *v1alpha1.Terraform,
	scriptContent, taggedImageName, secretName string,
	envVars map[string]string,
) v1alpha1.TerraformStatus {
	var status v1alpha1.TerraformStatus
	var terraformErr error
	

	err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		logger.Infof("Error occurred: %v", err)
		return strings.Contains(err.Error(), "timeout")
	}, func() error {
		_, terraformErr = containers.CreateOrUpdateRunPod(logger, clientset, observed.ObjectMeta.Name, observed.ObjectMeta.Namespace, scriptContent, envVars, taggedImageName, secretName, "deploy")
		return terraformErr
	})

	status.State = "Success"
	status.Message = "Terraform applied successfully"

	if err != nil {
		status.State = "Failed"
		status.Message = terraformErr.Error()
		return status
	}

  return status
}

func runDestroy(
	logger *zap.SugaredLogger,
	clientset kubernetes.Interface,
	dynamicClient dynamic.Interface,
	observed *v1alpha1.Terraform,
	scriptContent, taggedImageName, secretName string,
	envVars map[string]string,
) v1alpha1.TerraformStatus {
	var status v1alpha1.TerraformStatus
	var podName string

	if scriptContent == "" {
		status.State = "Success"
		status.Message = "No destroy script specified"
		return status
	}

	var terraformErr error
	retryErr := retry.OnError(retry.DefaultRetry, func(err error) bool {
		logger.Infof("Error occurred: %v", err)
		return strings.Contains(err.Error(), "timeout")
	}, func() error {
		podName, terraformErr = containers.CreateOrUpdateRunPod(
			logger, clientset, observed.ObjectMeta.Name, observed.ObjectMeta.Namespace, scriptContent, envVars, taggedImageName, secretName, "destroy",
		)
		return terraformErr
	})

	if retryErr != nil {
		status.State = "Failed"
		status.Message = terraformErr.Error()
		return status
	}

	// Wait for the destroy pod to complete and check its status
	for {
		// Retrieve the current state of the pod
		pod, err := clientset.CoreV1().Pods(observed.ObjectMeta.Namespace).Get(context.Background(), podName, metav1.GetOptions{})
		if err != nil {
			logger.Errorf("Failed to get pod %s/%s: %v", observed.ObjectMeta.Namespace, podName, err)
			status.State = "Failed"
			status.Message = fmt.Sprintf("Failed to get pod: %v", err)
			return status
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
			status.State = "Failed"
			status.Message = fmt.Sprintf("pod %s failed", podName)
			return status
		}

		// Sleep for 1 minute before checking again
		time.Sleep(1 * time.Minute)
	}

	logger.Info("Terraform Destroy successful")
	logger.Info("Removing finalizers")

	// If successful, remove finalizer
	err := kubernetesPkg.RemoveFinalizer(logger, dynamicClient, observed.ObjectMeta.Name, observed.ObjectMeta.Namespace)
	if err != nil {
		logger.Errorf("Failed to remove finalizer for %s/%s: %v", observed.ObjectMeta.Namespace, observed.ObjectMeta.Name, err)
		status.State = "Error"
		status.Message = fmt.Sprintf("Failed to remove finalizer: %v", err)
		return status
	}

	status.State = "Success"
	status.Message = "Destroy completed successfully"
	return status
}



func runPostDeploy(
	logger *zap.SugaredLogger,
	clientset kubernetes.Interface,
	name, namespace string,
	postDeploy v1alpha1.PostDeploy,
	envVars map[string]string,
	image, secretName string,
) (map[string]runtime.RawExtension, error) {
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

	command := fmt.Sprintf("%s %s", scriptPath, strings.Join(args, " "))

	fmt.Println("Command:", command)

	podName, err := containers.CreateOrUpdateRunPod(logger,clientset, name, namespace, command, envVars, image, secretName, "postdeploy")
	if err != nil {
		return nil, fmt.Errorf("failed to create post-deploy pod: %v", err)
	}

	output, err := containers.ExtractPostDeployOutput(logger,clientset, namespace, podName)
	if err != nil {
		return nil, fmt.Errorf("error executing postDeploy script: %v", err)
	}

	convertedOutput, err := convertToRawExtensionMap(output)
	if err != nil {
		return nil, fmt.Errorf("error converting postDeploy output: %v", err)
	}

	return convertedOutput, nil
}

// convertToRawExtensionMap converts a map[string]interface{} to map[string]runtime.RawExtension
func convertToRawExtensionMap(input map[string]interface{}) (map[string]runtime.RawExtension, error) {
    result := make(map[string]runtime.RawExtension)

	for key, value := range input {
        // Marshal each value to JSON
        raw, err := json.Marshal(value)
        if err != nil {
            return nil, fmt.Errorf("error marshaling value for key %s to JSON: %v", key, err)
        }
        
		// Encode to runtime.RawExtension
        result[key] = runtime.RawExtension{Raw: raw}
    }

    return result, nil
}
