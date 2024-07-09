package terraform

import (
	"fmt"
	
	"strings"
	
     
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/dynamic"

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
		status = v1alpha1.TerraformStatus{
			State:   "Progressing",
			Message: "Running Terraform Destroy",
		}

		status = runDestroy(logger, clientset,dynamicClient, observed, scriptContent, taggedImageName, secretName, envVars)

	

		return status
	}

	status = v1alpha1.TerraformStatus{
		State:   "Progressing",
		Message: "Running Terraform Apply",
	}

	status = runApply(logger, clientset, observed, scriptContent, taggedImageName, secretName, envVars)

	status = v1alpha1.TerraformStatus{
		State:   status.State,
		Message: status.Message,
		Output: status.Output,
		IngressURLs: status.IngressURLs,
		Credentials: status.Credentials,
    }

	if status.State == "Failed" {
		return status
	}

	if observed.Spec.PostDeploy.Script != "" {
		status = v1alpha1.TerraformStatus{
			State:   "Progressing",
			Message: "Running postDeploy script",
		}

		postDeployOutput, err := runPostDeploy(logger,clientset, observed.ObjectMeta.Name, observed.ObjectMeta.Namespace, observed.Spec.PostDeploy, envVars, taggedImageName, secretName)
		if err != nil {
			status = util.ErrorResponse(logger,"executing postDeploy script", err)
			return status
		}


		status = v1alpha1.TerraformStatus{
			State:            "Completed",
			Message:          "Processing completed successfully",
			PostDeployOutput: postDeployOutput,
		}
	} else {
		status = v1alpha1.TerraformStatus{
			State:   "Completed",
			Message: "Processing completed successfully",
		}
	}

	return status
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
	var podName string

	err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		logger.Infof("Error occurred: %v", err)
		return strings.Contains(err.Error(), "timeout")
	}, func() error {
		podName, terraformErr = containers.CreateOrUpdateRunPod(logger,clientset, observed.ObjectMeta.Name, observed.ObjectMeta.Namespace, scriptContent, envVars, taggedImageName, secretName, "deploy")
		return terraformErr
	})

	status.State = "Success"
	status.Message = "Terraform applied successfully"

	if err != nil {
		status.State = "Failed"
		status.Message = terraformErr.Error()
		return status
	}

	outputs, err := containers.WaitForPodCompletion(logger,clientset, observed.ObjectMeta.Namespace, podName)
	if err != nil {
		status.State = "Failed"
		status.Message = fmt.Sprintf("Error retrieving Terraform output: %v", err)
		return status
	}

	convertedOutputs, err := convertToRawExtensionMap(outputs)
	if err != nil {
		status.State = "Failed"
		status.Message = fmt.Sprintf("Error converting outputs: %v", err)
		return status
	}
	status.Output = convertedOutputs

	ingressURLs, err := kubernetesPkg.GetAllIngressURLs(clientset)
	if err != nil {
		status.State = "Failed"
		status.Message = fmt.Sprintf("Error retrieving Ingress URLs: %v", err)
		return status
	}

	convertedIngressURLs, err := convertToRawExtensionMap(ingressURLs)
	if err != nil {
		status.State = "Failed"
		status.Message = fmt.Sprintf("Error converting ingress URLs: %v", err)
		return status
	}
	status.IngressURLs = convertedIngressURLs

	credentials, err := kubernetesPkg.FetchCredentials(clientset)
	if err != nil {
		status.State = "Failed"
		status.Message = fmt.Sprintf("Error retrieving credentials: %v", err)
		return status
	}

	convertedCredentials, err := convertToRawExtensionMap(credentials)
	if err != nil {
		status.State = "Failed"
		status.Message = fmt.Sprintf("Error converting credentials: %v", err)
		return status
	}
	status.Credentials = convertedCredentials

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
		_, terraformErr = containers.CreateOrUpdateRunPod(logger, clientset, observed.ObjectMeta.Name, observed.ObjectMeta.Namespace, scriptContent, envVars, taggedImageName, secretName, "destroy")
		return terraformErr
	})

	if retryErr != nil {
		status.State = "Failed"
		status.Message = terraformErr.Error()
		return status
	}

	// If successful, remove finalizer
	err := kubernetesPkg.RemoveFinalizer(logger, dynamicClient, observed.ObjectMeta.Name, observed.ObjectMeta.Namespace)
	if err != nil {
		logger.Errorf("Failed to remove finalizer for %s/%s: %v", observed.ObjectMeta.Namespace, observed.ObjectMeta.Name, err)
		status.State = "Error"
		status.Message = fmt.Sprintf("Failed to remove finalizer: %v", err)
		return status
	}

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


func convertToRawExtensionMap(input map[string]interface{}) (map[string]runtime.RawExtension, error) {
	result := make(map[string]runtime.RawExtension)
	encoder := json.NewSerializerWithOptions(json.DefaultMetaFactory, nil, nil, json.SerializerOptions{Yaml: false, Pretty: false, Strict: false})
	for key, value := range input {
		raw, err := runtime.Encode(encoder, &runtime.Unknown{Raw: []byte(fmt.Sprintf("%v", value))})
		if err != nil {
			return nil, err
		}
		result[key] = runtime.RawExtension{Raw: raw}
	}
	return result, nil
}
