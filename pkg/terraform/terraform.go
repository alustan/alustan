package terraform

import (
	"fmt"
	"log"
	"strings"
	"encoding/json"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"

	kubernetesPkg "github.com/alustan/pkg/kubernetes"
	"github.com/alustan/pkg/util"
	"github.com/alustan/pkg/containers"
	"github.com/alustan/api/v1alpha1"
)

func GetScriptContent(observed v1alpha1.SyncRequest) (string, v1alpha1.ParentResourceStatus) {
	var scriptContent string
	var status v1alpha1.ParentResourceStatus

	if observed.Finalizing {
		if observed.Parent.Spec.Scripts.Destroy != "" {
			scriptContent = observed.Parent.Spec.Scripts.Destroy
		} else {
			scriptContent = ""
		}
	} else {
		scriptContent = observed.Parent.Spec.Scripts.Deploy
		if scriptContent == "" {
			status = util.ErrorResponse("executing script", fmt.Errorf("deploy script is missing"))
			return "", status
		}
	}

	return scriptContent, status
}

func ExecuteTerraform(
	clientset kubernetes.Interface,
	observed v1alpha1.SyncRequest,
	scriptContent, taggedImageName, secretName string,
	envVars map[string]string,
) v1alpha1.ParentResourceStatus {

	var status v1alpha1.ParentResourceStatus

	if observed.Finalizing {
		status = v1alpha1.ParentResourceStatus{
			State:   "Progressing",
			Message: "Running Terraform Destroy",
		}

		status = runDestroy(clientset, observed, scriptContent, taggedImageName, secretName, envVars)

		if status.State == "Success" {
			status = v1alpha1.ParentResourceStatus{
				State:     "Completed",
				Message:   "Destroy process completed successfully",
				Finalized: true,
			}
		}

		return status
	}

	status = v1alpha1.ParentResourceStatus{
		State:   "Progressing",
		Message: "Running Terraform Apply",
	}

	status = runApply(clientset, observed, scriptContent, taggedImageName, secretName, envVars)

	if status.State == "Failed" {
		return status
	}

	if observed.Parent.Spec.PostDeploy.Script != "" {
		status = v1alpha1.ParentResourceStatus{
			State:   "Progressing",
			Message: "Running postDeploy script",
		}

		postDeployOutput, err := runPostDeploy(clientset, observed.Parent.ObjectMeta.Name, observed.Parent.ObjectMeta.Namespace, observed.Parent.Spec.PostDeploy, envVars, taggedImageName, secretName)
		if err != nil {
			status = util.ErrorResponse("executing postDeploy script", err)
			return status
		}

		postDeployOutputJson, err := json.Marshal(postDeployOutput)
		if err != nil {
			status = util.ErrorResponse("marshalling postDeploy output", err)
			return status
		}

		status = v1alpha1.ParentResourceStatus{
			State:            "Completed",
			Message:          "Processing completed successfully",
			PostDeployOutput: postDeployOutputJson,
		}
	} else {
		status = v1alpha1.ParentResourceStatus{
			State:   "Completed",
			Message: "Processing completed successfully",
		}
	}

	return status
}


func runApply(
	clientset kubernetes.Interface,
	observed v1alpha1.SyncRequest,
	scriptContent, taggedImageName, secretName string,
	envVars map[string]string,
) v1alpha1.ParentResourceStatus {
	var status v1alpha1.ParentResourceStatus
	var terraformErr error
	var podName string

	err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		log.Printf("Error occurred: %v", err)
		return strings.Contains(err.Error(), "timeout")
	}, func() error {
		podName, terraformErr = containers.CreateOrUpdateRunPod(clientset, observed.Parent.ObjectMeta.Name, observed.Parent.ObjectMeta.Namespace, scriptContent, envVars, taggedImageName, secretName, "deploy")
		return terraformErr
	})

	status.State = "Success"
	status.Message = "Terraform applied successfully"

	if err != nil {
		status.State = "Failed"
		status.Message = terraformErr.Error()
		return status
	}

	outputs, err := containers.WaitForPodCompletion(clientset, observed.Parent.ObjectMeta.Namespace, podName)
	if err != nil {
		status.State = "Failed"
		status.Message = fmt.Sprintf("Error retrieving Terraform output: %v", err)
		return status
	}

	outputsJson, err := json.Marshal(outputs)
	if err != nil {
		status.State = "Failed"
		status.Message = fmt.Sprintf("Error marshalling Terraform output: %v", err)
		return status
	}

	status.Output = outputsJson

	ingressURLs, err := kubernetesPkg.GetAllIngressURLs(clientset)
	if err != nil {
		status.State = "Failed"
		status.Message = fmt.Sprintf("Error retrieving Ingress URLs: %v", err)
		return status
	}

	ingressURLsJson, err := json.Marshal(ingressURLs)
	if err != nil {
		status.State = "Failed"
		status.Message = fmt.Sprintf("Error marshalling Ingress URLs: %v", err)
		return status
	}

	status.IngressURLs = ingressURLsJson

	credentials, err := kubernetesPkg.FetchCredentials(clientset)
	if err != nil {
		status.State = "Failed"
		status.Message = fmt.Sprintf("Error retrieving credentials: %v", err)
		return status
	}

	credentialsJson, err := json.Marshal(credentials)
	if err != nil {
		status.State = "Failed"
		status.Message = fmt.Sprintf("Error marshalling credentials: %v", err)
		return status
	}

	status.Credentials = credentialsJson

	return status
}

func runDestroy(
	clientset kubernetes.Interface,
	observed v1alpha1.SyncRequest,
	scriptContent, taggedImageName, secretName string,
	envVars map[string]string,
) v1alpha1.ParentResourceStatus {
	var status v1alpha1.ParentResourceStatus

	if scriptContent == "" {
		status.State = "Success"
		status.Message = "No destroy script specified"
		return status
	}

	var terraformErr error
	err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		log.Printf("Error occurred: %v", err)
		return strings.Contains(err.Error(), "timeout")
	}, func() error {
		_, terraformErr = containers.CreateOrUpdateRunPod(clientset, observed.Parent.ObjectMeta.Name, observed.Parent.ObjectMeta.Namespace, scriptContent, envVars, taggedImageName, secretName, "destroy")
		return terraformErr
	})

	status.State = "Success"
	status.Message = "Terraform destroyed successfully"

	if err != nil {
		status.State = "Failed"
		status.Message = terraformErr.Error()
	}

	return status
}

func runPostDeploy(
	clientset kubernetes.Interface,
	name, namespace string,
	postDeploy v1alpha1.PostDeploy,
	envVars map[string]string,
	image, secretName string,
) (map[string]interface{}, error) {
	scriptPath := postDeploy.Script
	if (!strings.HasPrefix(scriptPath, "./")) {
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

	podName, err := containers.CreateOrUpdateRunPod(clientset, name, namespace, command, envVars, image, secretName, "postdeploy")
	if err != nil {
		return nil, fmt.Errorf("failed to create post-deploy pod: %v", err)
	}

	output, err := containers.ExtractPostDeployOutput(clientset, namespace, podName)
	if err != nil {
		return nil, fmt.Errorf("error executing postDeploy script: %v", err)
	}

	return output, nil
}
