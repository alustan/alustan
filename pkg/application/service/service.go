package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"encoding/json"
    "regexp"
    "bytes"
	"text/template"
    "os"
	
    "gopkg.in/yaml.v2"
	"k8s.io/client-go/dynamic"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime"
	corev1 "k8s.io/api/core/v1"
	appv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
    applicationset "github.com/argoproj/argo-cd/v2/pkg/apiclient/applicationset"
    application "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
    "go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	

	"github.com/alustan/alustan/pkg/application/errorstatus"
	kubernetespkg "github.com/alustan/alustan/pkg/application/kubernetes"
	"github.com/alustan/alustan/api/app/v1alpha1"
  
    
 
)


func RunService(
    logger *zap.SugaredLogger,
    clientset kubernetes.Interface,
    dynamicClient dynamic.Interface,
    appSetClient applicationset.ApplicationSetServiceClient,
    appClient application.ApplicationServiceClient,
    observed *v1alpha1.App,
    latestTag string,
    finalizing bool,
) (v1alpha1.AppStatus, error) {

    var status v1alpha1.AppStatus
    secretName := fmt.Sprintf("%s-container-secret", observed.ObjectMeta.Name)
    key := "pat"
    gitHubPATBase64 := os.Getenv("GITHUB_TOKEN")

    // Only create or update the secret if the GITHUB_TOKEN is set
    if gitHubPATBase64 != "" {
        err := kubernetespkg.CreateOrUpdateSecretWithGitHubPAT(logger, clientset, "argocd", secretName, key, gitHubPATBase64)
        if err != nil {
            logger.Errorf("Failed to create/update secret: %v", err)
            return status, fmt.Errorf("failed to create/update secret: %v", err)
        }
    } else {
        logger.Warn("GITHUB_TOKEN environment variable is not set, skipping secret creation")
    }

    if finalizing {
        logger.Info("Attempting to delete application")
        status, err := DeleteApplicationSet(logger, clientset, dynamicClient, appSetClient, observed)
        if err != nil {
            return status, fmt.Errorf("error deleting ApplicationSet: %v", err)
        }
        return status, nil
    }

    status = v1alpha1.AppStatus{
        State:   "Progressing",
        Message: "Running App",
    }

    // Extract dependencies
    dependencies := ExtractDependencies(observed)

    // Check if all dependent ApplicationSets are healthy and synced before proceeding
    retryInterval := 30 * time.Second
    timeout := 10 * time.Minute

    err := WaitForAllDependenciesHealthAndSyncStatus(logger, appClient, dependencies, retryInterval, timeout)
    if err != nil {
        return errorstatus.ErrorResponse(logger, "Waiting for dependencies to become healthy", err), err
    }

    // Proceed with creating the ApplicationSet
    conditions, err := CreateApplicationSet(logger, clientset, appSetClient, appClient, observed, secretName, key, latestTag)
    if err != nil {
        return errorstatus.ErrorResponse(logger, "Running App", err), err
    }

    // Ensure conditions is not nil before dereferencing
    if  len(conditions) == 0 {
        status.State = "Failed"
        status.Message = "ApplicationSet creation failed"
        return status, nil
    }

    // Fetch and validate Ingress URLs
    ingressURLs, err := kubernetespkg.GetAllIngressURLs(clientset)
    if err != nil {
        status.State = "Failed"
        status.Message = fmt.Sprintf("Error retrieving Ingress URLs: %v", err)
        return status, err
    }

    convertedIngressURLs, err := convertToRawExtensionMap(ingressURLs)
    if err != nil {
        status.State = "Failed"
        status.Message = fmt.Sprintf("Error converting ingress URLs: %v", err)
        return status, err
    }

    // Preserve any existing status fields in the AppStatus struct
    finalStatus := v1alpha1.AppStatus{
        State:        "Completed",
        Message:      "Successfully applied",
        HealthStatus: conditions,
        PreviewURLs:  convertedIngressURLs,
    }

    return finalStatus, nil
}




func fetchSecretAnnotations(
    clientset kubernetes.Interface, 
    secretTypeLabel, secretTypeValue, environmentLabel, environmentValue string,
) (map[string]string, error) {

    secrets, err := clientset.CoreV1().Secrets("alustan").List(context.TODO(), metav1.ListOptions{
        LabelSelector: fmt.Sprintf("%s=%s", secretTypeLabel, secretTypeValue),
    })
    if err != nil {
        return nil, err
    }

    var matchedSecret *corev1.Secret
    for _, secret := range secrets.Items {
        if value, ok := secret.Labels[environmentLabel]; ok && value == environmentValue {
            matchedSecret = &secret
            break
        }
    }

    if matchedSecret == nil {
        // Return an empty map instead of nil to avoid nil pointer dereference
        return map[string]string{}, fmt.Errorf("no secret found with label %s=%s and %s=%s", secretTypeLabel, secretTypeValue, environmentLabel, environmentValue)
    }

    return matchedSecret.Annotations, nil
}

// replaceWorkspaceValues replaces placeholders in the values map with corresponding values from the output map
func replaceWorkspaceValues(values map[string]interface{}, output map[string]string) (map[string]interface{}, error) {
    
    modifiedValues := make(map[string]interface{})

    for key, value := range values {
        switch v := value.(type) {
        case string:
            replacedValue, err := replacePlaceholder(v, output)
            if err != nil {
                return nil,  err
            }
            modifiedValues[key] = replacedValue
           
        case map[string]interface{}:
            nestedValues,  err := replaceWorkspaceValues(v, output)
            if err != nil {
                return nil,  err
            }
            modifiedValues[key] = nestedValues
           
        default:
            modifiedValues[key] = value
        }
    }

    return modifiedValues,  nil
}

func updateImageTag(values map[string]interface{}, newTag string) map[string]interface{} {
	updatedValues := make(map[string]interface{})

	for key, value := range values {
		switch v := value.(type) {
		case map[string]interface{}:
			if key == "image" {
				// Update the tag if it exists
				if _, exists := v["tag"]; exists {
					v["tag"] = newTag
				}
				updatedValues[key] = v
			} else {
				updatedValues[key] = updateImageTag(v, newTag)
			}
		case []interface{}:
			// Handle slice of maps for cases like containers in the spec
			var updatedSlice []interface{}
			for _, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					updatedSlice = append(updatedSlice, updateImageTag(itemMap, newTag))
				} else {
					updatedSlice = append(updatedSlice, item)
				}
			}
			updatedValues[key] = updatedSlice
		default:
			updatedValues[key] = value
		}
	}

	return updatedValues
}


// modifyIngressHost modifies the host values in Ingress resources
func modifyIngressHost(values map[string]interface{}, preview bool, prefix string) map[string]interface{} {
    modifiedValues := make(map[string]interface{})

    for key, value := range values {
        switch v := value.(type) {
        case map[string]interface{}:
            if key == "ingress" {
                ingressMap := v
                for ingressKey, ingressValue := range ingressMap {
                    switch iv := ingressValue.(type) {
                    case []interface{}:
                        if ingressKey == "hosts" {
                            for _, hostItem := range iv {
                                hostMap, ok := hostItem.(map[string]interface{})
                                if ok {
                                    if host, exists := hostMap["host"]; exists {
                                        hostStr, ok := host.(string)
                                        if ok && preview {
                                            hostStr = fmt.Sprintf("%s-%s", prefix, hostStr)
                                        }
                                        hostMap["host"] = hostStr
                                    }
                                }
                            }
                            ingressMap[ingressKey] = iv
                        }
                    }
                }
                modifiedValues[key] = ingressMap
            } else {
                modifiedValues[key] = modifyIngressHost(v, preview, prefix)
            }
        default:
            modifiedValues[key] = value
        }
    }

    return modifiedValues
}

// formatValuesAsHelmString converts a map of values to a Helm-compatible YAML string
func formatValuesAsHelmString(logger *zap.SugaredLogger,values map[string]interface{}) string {
    // Convert the map to YAML
    yamlData, err := yaml.Marshal(values)
    if err != nil {
        logger.Fatalf("error: %v", err)
    }
    return string(yamlData)
}


func indent(text, prefix string) string {
	var indented strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if line != "" {
			indented.WriteString(prefix + line + "\n")
		}
	}
	return indented.String()
}

// replacePlaceholder uses Go templates to replace placeholders with corresponding values from the output map
func replacePlaceholder(value string, output map[string]string) (string, error) {
	tmpl, err := template.New("placeholder").Parse(value)
	if err != nil {
		return "", err
	}

	var result bytes.Buffer
	if err := tmpl.Execute(&result, output); err != nil {
		return "", err
	}

	return result.String(), nil
}

// Helper function to convert RawExtension map to interface map
func convertRawExtensionsToInterface(values map[string]runtime.RawExtension) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for key, value := range values {
		var decodedValue interface{}
		if err := json.Unmarshal(value.Raw, &decodedValue); err != nil {
			return nil, fmt.Errorf("error decoding value for key %s: %v", key, err)
		}
		result[key] = decodedValue
	}
	return result, nil
}

func CreateApplicationSet(
    logger *zap.SugaredLogger,
    clientset kubernetes.Interface,
    appSetClient applicationset.ApplicationSetServiceClient,
    appClient application.ApplicationServiceClient, 
    observed *v1alpha1.App,
    secretName, key, latestTag string,
) ([]appv1alpha1.ApplicationCondition, error) { 

    argocdNamespace := "argocd"
    secretTypeLabel := "alustan.io/secret-type"
    secretTypeValue := "cluster"
    environmentLabel := "environment"
    environmentValue := observed.Spec.Environment
    values := observed.Spec.Source.Values
    preview := observed.Spec.PreviewEnvironment.Enabled
    gitOwner := observed.Spec.PreviewEnvironment.GitOwner
    gitRepo := observed.Spec.PreviewEnvironment.GitRepo
    intervalSeconds := observed.Spec.PreviewEnvironment.IntervalSeconds
    name := observed.ObjectMeta.Name
    namespace := observed.ObjectMeta.Namespace
    repoURL := observed.Spec.Source.RepoURL
    path := observed.Spec.Source.Path
    releaseName := observed.Spec.Source.ReleaseName
    targetRevision := observed.Spec.Source.TargetRevision
    requeueAfterSeconds := 600
    if intervalSeconds > 0 {
        requeueAfterSeconds = intervalSeconds
    }

    logger.Infof("Creating ApplicationSet with name: %s in namespace: %s", name, namespace)

    // Convert RawExtension values to interface{}
    convertedValues, err := convertRawExtensionsToInterface(values)
    if err != nil {
        logger.Errorf("Failed to convert values: %v", err)
        return nil, fmt.Errorf("failed to convert values: %v", err)
    }

    var modifiedValues map[string]interface{}

    // Regular expression pattern to match Go template placeholders
    placeholderPattern := `\{\{\.[^}]+\}\}`

    // Check if values contain Go template placeholders
    if containsPlaceholders(convertedValues, placeholderPattern) {
        logger.Info("Values contain placeholders. Fetching annotations.")
        annotations, err := fetchSecretAnnotations(clientset, secretTypeLabel, secretTypeValue, environmentLabel, environmentValue)
        if err != nil {
            if err.Error() == fmt.Sprintf("no secret found with label %s=%s and %s=%s", secretTypeLabel, secretTypeValue, environmentLabel, environmentValue) {
                // Return an empty ApplicationSet and log the error
                logger.Warnf("No secret found with specified labels: %s", err.Error())
                return nil, nil
            }
            logger.Errorf("Failed to fetch secret annotations: %v", err)
            return nil, err
        }

        // Check if annotations are empty
        if len(annotations) == 0 {
            logger.Error("No annotations found and values contain placeholders")
            return nil, nil
        }

        // Replace placeholders with values from annotations
        modifiedValues, err = replaceWorkspaceValues(convertedValues, annotations)
        if err != nil {
            return nil, err
        }
    } else {
        logger.Info("No placeholders in values, continuing execution with default values")
        modifiedValues = convertedValues
    }

    modifiedValues = updateImageTag(modifiedValues, latestTag)

    // Modify Ingress hosts if preview is true
    if preview {
        logger.Info("Preview environment enabled. Modifying Ingress hosts.")
        modifiedValues = modifyIngressHost(modifiedValues, preview, "{{.branch}}-{{.number}}")
    }

    // Convert modifiedValues to Helm string format
    helmValues := formatValuesAsHelmString(logger, modifiedValues)

    // Check if the secret exists
    var secretExists bool
    _, err = clientset.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
    if err == nil {
        secretExists = true
    } else if errors.IsNotFound(err) {
        secretExists = false
    } else {
        logger.Errorf("Failed to check if secret exists: %v", err)
        return nil, err
    }

    var generators []appv1alpha1.ApplicationSetGenerator

    // Define generators based on the strategy
    if preview {
        logger.Info("Defining generators for preview environment.")
        requeueAfterSeconds := int64(requeueAfterSeconds)
        pullRequestGenerator := appv1alpha1.PullRequestGenerator{
            Github: &appv1alpha1.PullRequestGeneratorGithub{
                Owner:  gitOwner,
                Repo:   gitRepo,
                Labels: []string{"preview"},
            },
            RequeueAfterSeconds: &requeueAfterSeconds,
        }

        // Add TokenRef if the secret exists
        if secretExists {
            pullRequestGenerator.Github.TokenRef = &appv1alpha1.SecretRef{
                SecretName: secretName,
                Key:        key,
            }
        }

        generators = []appv1alpha1.ApplicationSetGenerator{
            {
                Matrix: &appv1alpha1.MatrixGenerator{
                    Generators: []appv1alpha1.ApplicationSetNestedGenerator{
                        {
                            PullRequest: &pullRequestGenerator,
                        },
                        {
                            Clusters: &appv1alpha1.ClusterGenerator{
                                Selector: metav1.LabelSelector{
                                    MatchLabels: map[string]string{
                                        "environment": environmentValue,
                                    },
                                },
                            },
                        },
                    },
                },
            },
        }
    } else {
        logger.Info("Defining generators for non-preview environment.")
        generators = []appv1alpha1.ApplicationSetGenerator{
            {
                Clusters: &appv1alpha1.ClusterGenerator{
                    Selector: metav1.LabelSelector{
                        MatchLabels: map[string]string{
                            "environment": environmentValue,
                        },
                    },
                },
            },
        }
    }

    // Define the template metadata and destination based on the preview flag
    templateMeta := appv1alpha1.ApplicationSetTemplateMeta{
        Name: name,
        Labels: map[string]string{
            "workload": "true",
        },
    }
    templateDestination := appv1alpha1.ApplicationDestination{
        Name:      "{{.name}}",
        Namespace: namespace,
    }
    if preview {
        templateMeta.Name = fmt.Sprintf("%s-{{.branch}}-{{.number}}", name)
        templateDestination = appv1alpha1.ApplicationDestination{
            Server:    "https://kubernetes.default.svc",
            Namespace: "preview-{{.branch}}-{{.number}}",
        }
    }

    appSet := &appv1alpha1.ApplicationSet{
        TypeMeta: metav1.TypeMeta{
            APIVersion: "argoproj.io/v1alpha1",
            Kind:       "ApplicationSet",
        },
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: argocdNamespace,
        },
        Spec: appv1alpha1.ApplicationSetSpec{
            SyncPolicy: &appv1alpha1.ApplicationSetSyncPolicy{
                PreserveResourcesOnDeletion: false,
            },
            GoTemplate:        true,
            GoTemplateOptions: []string{"missingkey=error"},
            Generators:        generators,
            Template: appv1alpha1.ApplicationSetTemplate{
                ApplicationSetTemplateMeta: templateMeta,
                Spec: appv1alpha1.ApplicationSpec{
                    Project:    "default",
                    Destination: templateDestination,
                    SyncPolicy: &appv1alpha1.SyncPolicy{
                        Automated: &appv1alpha1.SyncPolicyAutomated{},
                        SyncOptions: []string{
                            "CreateNamespace=true",
                        },
                    },
                    Source: &appv1alpha1.ApplicationSource{
                        RepoURL:        repoURL,
                        Path:           path,
                        TargetRevision: targetRevision,
                        Helm: &appv1alpha1.ApplicationSourceHelm{
                            ReleaseName: releaseName,
                            Values:      helmValues,
                        },
                    },
                },
            },
        },
    }

    logger.Info("Creating ApplicationSet in ArgoCD.")

  
    err = retry.OnError(retry.DefaultRetry, errors.IsInternalError, func() error {
        _, err = appSetClient.Create(context.Background(), &applicationset.ApplicationSetCreateRequest{
            Applicationset: appSet,
        })
        return err
    })

    if err != nil {
        logger.Errorf("Failed to create ApplicationSet: %v", err)
        return nil, err
    }

    logger.Infof("Successfully applied ApplicationSet '%s' using ArgoCD", appSet.Name)

    // Wait for a short period to allow the ApplicationSet to be processed
    time.Sleep(15 * time.Second)

    var app *appv1alpha1.Application

    // Retrieve the status of the created application
    appNamespace := "argocd"
	app, err = appClient.Get(context.Background(), &application.ApplicationQuery{
		Name:        &name,
		AppNamespace: &appNamespace,
	})
	if err != nil {
		logger.Errorf("Failed to get application: %v", err)
		return nil, err
	}

	logger.Infof("Successfully retrieved application for '%s'", name)

	// Extract all ApplicationCondition instances
	conditions := app.Status.Conditions

	return conditions, nil
}




func DeleteApplicationSet(logger *zap.SugaredLogger, clientset kubernetes.Interface, dynamicClient dynamic.Interface, appSetClient applicationset.ApplicationSetServiceClient, observed *v1alpha1.App) (v1alpha1.AppStatus, error) {

	appSetName := observed.ObjectMeta.Name

	logger.Info("Attempting to delete ApplicationSet")

	// Check for dependent services
	dependentServices, err := checkDependentServices(dynamicClient, observed)
	if err != nil {
		return v1alpha1.AppStatus{
			State:   "Failed",
			Message: fmt.Sprintf("Error checking dependent services: %v", err),
		}, err
	}
	if len(dependentServices) > 0 {
		return v1alpha1.AppStatus{
			State:   "Blocked",
			Message: "Service has dependent services, cannot delete",
		}, nil
	}

	// Retry mechanism to delete ApplicationSet
	err = retry.OnError(retry.DefaultRetry, errors.IsInternalError, func() error {
		
     _, err = appSetClient.Delete(context.Background(), &applicationset.ApplicationSetDeleteRequest{
			Name: appSetName,  
		})
		return err
	})
	if err != nil {
		return v1alpha1.AppStatus{
			State:   "Failed",
			Message: fmt.Sprintf("Error deleting ApplicationSet: %v", err),
		}, err
	}

	logger.Infof("Successfully deleted ApplicationSet '%s' using ArgoCD", appSetName)

	// If successful, remove finalizer
	err = kubernetespkg.RemoveFinalizer(logger, dynamicClient, observed.ObjectMeta.Name, observed.ObjectMeta.Namespace)
	if err != nil {
		logger.Errorf("Failed to remove finalizer for %s/%s: %v", observed.ObjectMeta.Namespace, observed.ObjectMeta.Name, err)
		return v1alpha1.AppStatus{
			State:   "Error",
			Message: fmt.Sprintf("Failed to remove finalizer: %v", err),
		}, err
	}

	return v1alpha1.AppStatus{
		State:   "Completed",
		Message: "Successfully deleted ApplicationSet",
	}, nil
}




// checkDependentServices checks if there are other services depending on the given service.
func checkDependentServices(dynamicClient dynamic.Interface, observed *v1alpha1.App) ([]string, error) {
    var dependentServices []string
    apps, err := dynamicClient.Resource(schema.GroupVersionResource{
        Group:    "alustan.io",
        Version:  "v1alpha1",
        Resource: "apps",
    }).Namespace(observed.Namespace).List(context.TODO(), metav1.ListOptions{})
    if err != nil {
        return nil, err
    }

    for _, svc := range apps.Items {
        appSpec, ok := svc.Object["spec"].(map[string]interface{})
        if !ok {
            continue
        }
        dependencies, exists := appSpec["dependencies"].(map[string]interface{})
        if exists {
            for depType, depList := range dependencies {
                if depType == "service" {
                    for _, depName := range depList.([]interface{}) {
                        if depName == observed.ObjectMeta.Name {
                            dependentServices = append(dependentServices, svc.GetName())
                        }
                    }
                }
            }
        }
    }

    return dependentServices, nil
}


func CheckApplicationHealthAndSyncStatus(logger *zap.SugaredLogger, appClient application.ApplicationServiceClient, appName string) (bool, error) {
    // Retrieve the Application
    appNamespace := "argocd"
    app, err := appClient.Get(context.Background(), &application.ApplicationQuery{
        Name:      &appName, 
        AppNamespace: &appNamespace,   
		
    })
    if err != nil {
        logger.Error(err.Error())
        return false, err
    }

    // Check if the Application status indicates it is healthy and synced
    isHealthy := app.Status.Health.Status == "Healthy"
    isSynced := app.Status.Sync.Status == "Synced"

 

    return isHealthy && isSynced, nil
}



func ExtractDependencies(observed *v1alpha1.App) []string {
    if observed.Spec.Dependencies.Service == nil {
        return nil
    }

    var dependencies []string
    for _, dep := range observed.Spec.Dependencies.Service {
        if name, exists := dep["name"]; exists {
            dependencies = append(dependencies, name)
        }
    }

    return dependencies
}




func WaitForAllDependenciesHealthAndSyncStatus(
    logger *zap.SugaredLogger, 
    appClient application.ApplicationServiceClient,
    dependencies []string, 
    
    retryInterval, timeout time.Duration,
) error {
    ticker := time.NewTicker(retryInterval)
    defer ticker.Stop() // Ensure the ticker is stopped when the function exits

    timeoutChan := time.After(timeout) // Channel that triggers after the timeout

    // Map to track the health and sync status of dependencies
    dependencyStatus := make(map[string]bool)
    for _, dep := range dependencies {
        dependencyStatus[dep] = false // Initialize all dependencies as not healthy/synced
    }

    for {
        select {
        case <-ticker.C: // Execute every retryInterval
            allHealthyAndSynced := true
            for dep := range dependencyStatus {
                healthyAndSynced, err := CheckApplicationHealthAndSyncStatus(logger, appClient, dep)
                if err != nil {
                    return err // Return if an error occurs
                }
                if healthyAndSynced {
                    dependencyStatus[dep] = true // Mark dependency as healthy and synced
                } else {
                    allHealthyAndSynced = false // Mark that not all dependencies are healthy and synced
                    logger.Infof("Waiting for Application %s to become healthy and synced...", dep)
                }
            }
            if allHealthyAndSynced {
                return nil // All dependencies are healthy and synced
            }
        case <-timeoutChan: // Execute if the timeout elapses
            return fmt.Errorf("timed out waiting for dependencies to become healthy and synced")
        }
    }
}

// convertToRawExtensionMap converts a map[string]interface{} to map[string]runtime.RawExtension
func convertToRawExtensionMap(values map[string]interface{}) (map[string]runtime.RawExtension, error) {
    result := make(map[string]runtime.RawExtension)
    for key, value := range values {
        raw, err := json.Marshal(value)
        if err != nil {
            return nil, err
        }
        result[key] = runtime.RawExtension{Raw: raw}
    }
    return result, nil
}

// containsPlaceholders checks for Go template-style placeholders in the format {{.PLACEHOLDER}}
func containsPlaceholders(values interface{}, placeholderPattern string) bool {
	placeholderRegex := regexp.MustCompile(placeholderPattern)

	switch v := values.(type) {
	case map[string]interface{}:
		for _, val := range v {
			if containsPlaceholders(val, placeholderPattern) {
				return true
			}
		}
	case []interface{}:
		for _, val := range v {
			if containsPlaceholders(val, placeholderPattern) {
				return true
			}
		}
	case string:
		if placeholderRegex.MatchString(v) {
			return true
		}
	}
	return false
}
