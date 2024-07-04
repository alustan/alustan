package terraform

import (
	"fmt"
	"log"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"

	kubernetesPkg "github.com/alustan/pkg/kubernetes"
	"github.com/alustan/pkg/util"
	"github.com/alustan/pkg/containers"
	"github.com/alustan/pkg/schematypes"
)


func GetScriptContent(
	observed schematypes.SyncRequest,
	updateStatus func(observed schematypes.SyncRequest, status schematypes.ParentResourceStatus) error,
) (string, schematypes.ParentResourceStatus) {
	var scriptContent string
	var status schematypes.ParentResourceStatus

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
			}
			return "", status
		}
	}

	return scriptContent, status
}

func ExecuteTerraform(
	clientset kubernetes.Interface,
	observed schematypes.SyncRequest,
	scriptContent, taggedImageName, secretName string,
	envVars map[string]string,
	updateStatus func(observed schematypes.SyncRequest, status schematypes.ParentResourceStatus) error,
) schematypes.ParentResourceStatus {

	var status schematypes.ParentResourceStatus

	if observed.Finalizing {
		err := updateStatus(observed, schematypes.ParentResourceStatus{
			State:   "Progressing",
			Message: "Running Terraform Destroy",
		})
		if err != nil {
			log.Printf("Failed to update status: %v", err)
		}

		status = runDestroy(clientset, observed, scriptContent, taggedImageName, secretName, envVars)

		err = updateStatus(observed, status)
		if err != nil {
			log.Printf("Failed to update status: %v", err)
		}

		if status.State == "Success" {
			finalStatus := schematypes.ParentResourceStatus{
				State:     "Completed",
				Message:   "Destroy process completed successfully",
				Finalized: true,
			}

			err = updateStatus(observed, finalStatus)
			if err != nil {
				log.Printf("Failed to update status: %v", err)
			}
			return finalStatus
		}

		return status
	}

	err := updateStatus(observed, schematypes.ParentResourceStatus{
		State:   "Progressing",
		Message: "Running Terraform Apply",
	})
	if err != nil {
		log.Printf("Failed to update status: %v", err)
	}

	status = runApply(clientset, observed, scriptContent, taggedImageName, secretName, envVars)

	err = updateStatus(observed, status)
	if err != nil {
		log.Printf("Failed to update status: %v", err)
	}

	if status.State == "Failed" {
		return status
	}

	if observed.Parent.Spec.PostDeploy.Script != "" {
		err := updateStatus(observed, schematypes.ParentResourceStatus{
			State:   "Progressing",
			Message: "Running postDeploy script",
		})
		if err != nil {
			log.Printf("Failed to update status: %v", err)
		}

		postDeployOutput, err := runPostDeploy(clientset, observed.Parent.Metadata.Name, observed.Parent.Metadata.Namespace, observed.Parent.Spec.PostDeploy, envVars, taggedImageName, secretName)
		if err != nil {
			status := util.ErrorResponse("executing postDeploy script", err)

			err := updateStatus(observed, status)
			if err != nil {
				log.Printf("Failed to update status: %v", err)
			}
			return status
		}

		finalStatus := schematypes.ParentResourceStatus{
			State:            "Completed",
			Message:          "Processing completed successfully",
			PostDeployOutput: postDeployOutput,
		}

		err = updateStatus(observed, finalStatus)
		if err != nil {
			log.Printf("Failed to update status: %v", err)
		}

		return finalStatus
	}

	finalStatus := schematypes.ParentResourceStatus{
		State:   "Completed",
		Message: "Processing completed successfully",
	}

	err = updateStatus(observed, finalStatus)
	if err != nil {
		log.Printf("Failed to update status: %v", err)
	}

	return finalStatus
}

func runApply(
	clientset kubernetes.Interface,
	observed schematypes.SyncRequest,
	scriptContent, taggedImageName, secretName string,
	envVars map[string]string,
) schematypes.ParentResourceStatus {
	var status schematypes.ParentResourceStatus
	var terraformErr error
	var podName string

	err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		log.Printf("Error occurred: %v", err)
		return strings.Contains(err.Error(), "timeout")
	}, func() error {
		podName, terraformErr = containers.CreateRunPod(clientset, observed.Parent.Metadata.Name, observed.Parent.Metadata.Namespace, scriptContent, envVars, taggedImageName, secretName, "deploy")
		return terraformErr
	})

	status.State = "Success"
	status.Message = "Terraform applied successfully"

	if err != nil {
		status.State = "Failed"
		status.Message = terraformErr.Error()
		return status
	}

	outputs, err := containers.WaitForPodCompletion(clientset, observed.Parent.Metadata.Namespace, podName)
	if err != nil {
		status.State = "Failed"
		status.Message = fmt.Sprintf("Error retrieving Terraform output: %v", err)
		return status
	}
	
	output := make(map[string]interface{})
	for key, value := range outputs {
		output[key] = value
	}
	status.Output = output

	

	ingressURLs, err := kubernetesPkg.GetAllIngressURLs(clientset)
	if err != nil {
		status.State = "Failed"
		status.Message = fmt.Sprintf("Error retrieving Ingress URLs: %v", err)
		return status
	}
	ingress := make(map[string]interface{})
		for key, value := range ingressURLs {
			ingress[key] = value
		}
     status.IngressURLs = ingress
	

	credentials, err := kubernetesPkg.FetchCredentials(clientset)
	if err != nil {
		status.State = "Failed"
		status.Message = fmt.Sprintf("Error retrieving credentials: %v", err)
		return status
	}
	creds := map[string]interface{}{
		"ArgoCDUsername":  credentials.ArgoCDUsername,
		"ArgoCDPassword":  credentials.ArgoCDPassword,
		"GrafanaUsername": credentials.GrafanaUsername,
		"GrafanaPassword": credentials.GrafanaPassword,
	}
	status.Credentials = creds
	

	return status
}

func runDestroy(
	clientset kubernetes.Interface,
	observed schematypes.SyncRequest,
	scriptContent, taggedImageName, secretName string,
	envVars map[string]string,
) schematypes.ParentResourceStatus {
	var status schematypes.ParentResourceStatus

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
		_, terraformErr = containers.CreateRunPod(clientset, observed.Parent.Metadata.Name, observed.Parent.Metadata.Namespace, scriptContent, envVars, taggedImageName, secretName, "destroy")
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
	postDeploy schematypes.PostDeploy,
	envVars map[string]string,
	image, secretName string,
) (map[string]interface{}, error) {
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

	podName, err := containers.CreateRunPod(clientset, name, namespace, command, envVars, image, secretName, "postdeploy")
	if err != nil {
		return nil, fmt.Errorf("failed to create post-deploy pod: %v", err)
	}

	output, err := containers.ExtractPostDeployOutput(clientset, namespace, podName)
	if err != nil {
		return nil, fmt.Errorf("error executing postDeploy script: %v", err)
	}

	return output, nil
}
