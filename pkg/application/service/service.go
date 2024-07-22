package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"encoding/json"

	"k8s.io/client-go/dynamic"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"k8s.io/apimachinery/pkg/runtime"
	corev1 "k8s.io/api/core/v1"
	appv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
    applicationset "github.com/argoproj/argo-cd/v2/pkg/apiclient/applicationset"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	

	"github.com/alustan/alustan/pkg/application/errorstatus"
	kubernetespkg "github.com/alustan/alustan/pkg/application/kubernetes"
	"github.com/alustan/alustan/api/service/v1alpha1"
    "github.com/alustan/alustan/pkg/installargocd"
    
 
)


func RunService(
    logger *zap.SugaredLogger,
    clientset kubernetes.Interface,
    dynamicClient dynamic.Interface,
    argoClient apiclient.Client,
    observed *v1alpha1.Service,
    secretName, key string,
    finalizing bool,
) v1alpha1.ServiceStatus {

    var status v1alpha1.ServiceStatus

    if finalizing {
        logger.Info("Attempting to delete application")
        status, _ = DeleteApplicationSet(logger, clientset, dynamicClient, argoClient, observed)
        return status
    }

    status = v1alpha1.ServiceStatus{
        State:   "Progressing",
        Message: "Running Service",
    }

    err := installargocd.InstallArgoCD(logger, clientset, dynamicClient)
    if err != nil {
        return errorstatus.ErrorResponse(logger, "Failed to install ArgoCD", err)
    }

    // Extract dependencies
    dependencies := ExtractDependencies(observed)

    // Check if all dependent ApplicationSets are healthy before proceeding
    namespace := observed.Namespace
    retryInterval := 30 * time.Second
    timeout := 10 * time.Minute

    err = WaitForAllDependenciesHealth(logger, argoClient, dependencies, namespace, retryInterval, timeout)
    if err != nil {
        return errorstatus.ErrorResponse(logger, "Waiting for dependencies to become healthy", err)
    }

    // Proceed with creating the ApplicationSet
    appStatus, err := CreateApplicationSet(logger, clientset, argoClient, observed, secretName, key)
    if err != nil {
        return errorstatus.ErrorResponse(logger, "Running Service", err)
    }

    // Fetch and validate Ingress URLs
    ingressURLs, err := kubernetespkg.GetAllIngressURLs(clientset)
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

    // Preserve any existing status fields in the ServiceStatus struct
    finalStatus := v1alpha1.ServiceStatus{
        State:        "Completed",
        Message:      "Successfully applied",
        HealthStatus: *appStatus, // Dereference the pointer here
        PreviewURLs:  convertedIngressURLs,
    }

    return finalStatus
}




func fetchSecretAnnotations(clientset kubernetes.Interface,  secretTypeLabel, secretTypeValue, environmentLabel, environmentValue string) (map[string]string, error) {
	
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
		return nil, fmt.Errorf("no secret found with label %s=%s and %s=%s", secretTypeLabel, secretTypeValue, environmentLabel, environmentValue)
	}

	return matchedSecret.Annotations, nil
}

func replaceWorkspaceValues(values map[string]interface{}, output map[string]string, preview bool, prefix string) (string, string) {
	var builder strings.Builder
	var clusterValue string

	for key, value := range values {
		switch v := value.(type) {
		case string:
			replacedValue := replacePlaceholder(v, output)
			fmt.Fprintf(&builder, "%s: %s\n", key, replacedValue)
			if key == "cluster" {
				clusterValue = replacedValue
			}
		case map[string]interface{}:
			if key == "ingress" {
				ingressMap := v
				for ingressKey, ingressValue := range ingressMap {
					switch iv := ingressValue.(type) {
					case []interface{}:
						if ingressKey == "hosts" {
							var updatedHosts []interface{}
							for _, host := range iv {
								hostStr, ok := host.(string)
								if ok && preview {
									hostStr = fmt.Sprintf("%s-%s", prefix, hostStr)
								}
								updatedHosts = append(updatedHosts, hostStr)
							}
							ingressMap[ingressKey] = updatedHosts
						} else if ingressKey == "tls" {
							for _, tlsItem := range iv {
								tlsMap, ok := tlsItem.(map[string]interface{})
								if ok {
									if tlsHosts, exists := tlsMap["hosts"]; exists {
										var updatedTlsHosts []interface{}
										for _, tlsHost := range tlsHosts.([]interface{}) {
											tlsHostStr, ok := tlsHost.(string)
											if ok && preview {
												tlsHostStr = fmt.Sprintf("%s-%s", prefix, tlsHostStr)
											}
											updatedTlsHosts = append(updatedTlsHosts, tlsHostStr)
										}
										tlsMap["hosts"] = updatedTlsHosts
									}
								}
							}
						}
					}
				}
				v = ingressMap
			}

			nestedOutput, nestedClusterValue := replaceWorkspaceValues(v, output, preview, prefix)
			fmt.Fprintf(&builder, "%s:\n%s\n", key, indent(nestedOutput, "  "))
			if nestedClusterValue != "" {
				clusterValue = nestedClusterValue
			}
		default:
			fmt.Fprintf(&builder, "%s: %v\n", key, value)
		}
	}
	return builder.String(), clusterValue
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

func replacePlaceholder(value string, output map[string]string) string {
	for key, val := range output {
		placeholder := fmt.Sprintf("${workspace.%s}", key)
		value = strings.ReplaceAll(value, placeholder, val)
	}
	return value
}

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
    argoClient apiclient.Client,
    observed *v1alpha1.Service,
    secretName, key string,
) (*appv1alpha1.ApplicationSetStatus, error) {

    argocdNamespace := "argocd"
    secretTypeLabel := "alustan.io/secret-type"
    secretTypeValue := "cluster"
    environmentLabel := "environment"
    environmentValue := observed.Spec.Workspace
    values := observed.Spec.Source.Values
    preview := observed.Spec.PreviewEnvironment.Enabled
    gitOwner := observed.Spec.PreviewEnvironment.GitOwner
    gitRepo := observed.Spec.PreviewEnvironment.GitRepo
    name := observed.ObjectMeta.Name
    namespace := observed.ObjectMeta.Namespace
    repoURL := observed.Spec.Source.RepoURL
    path := observed.Spec.Source.Path
    releaseName := observed.Spec.Source.ReleaseName
    targetRevision := observed.Spec.Source.TargetRevision

    
    annotations, err := fetchSecretAnnotations(clientset, secretTypeLabel, secretTypeValue, environmentLabel, environmentValue)
    if err != nil {
        if err.Error() == fmt.Sprintf("no secret found with label %s=%s and %s=%s", secretTypeLabel, secretTypeValue, environmentLabel, environmentValue) {
            // Return an empty ApplicationSet and log the error
            logger.Warnf("No secret found with specified labels: %s", err.Error())
            return nil, nil
        }
        logger.Error(err.Error())
        return nil, err
    }

    // Convert RawExtension values to interface{}
	convertedValues, err := convertRawExtensionsToInterface(values)
	if err != nil {
		return nil, fmt.Errorf("failed to convert values: %v", err)
	}

	modifiedValues, cluster := replaceWorkspaceValues(convertedValues, annotations, preview, "preview-{{.branch}}-{{.number}}")

    
   var generators []appv1alpha1.ApplicationSetGenerator

    // Define generators based on the strategy
    if preview {
        requeueAfterSeconds := int64(600)
        generators = []appv1alpha1.ApplicationSetGenerator{
            {
                Matrix: &appv1alpha1.MatrixGenerator{
                    Generators: []appv1alpha1.ApplicationSetNestedGenerator{
                        {
                            PullRequest: &appv1alpha1.PullRequestGenerator{
                                Github: &appv1alpha1.PullRequestGeneratorGithub{
                                    Owner:  gitOwner,
                                    Repo:   gitRepo,
                                    Labels: []string{"preview"},
                                    TokenRef: &appv1alpha1.SecretRef{
                                        SecretName: secretName,
                                        Key:        key,
                                    },
                                },
                                RequeueAfterSeconds: &requeueAfterSeconds,
                            },
                        },
                        {
                            Clusters: &appv1alpha1.ClusterGenerator{
                                Selector: metav1.LabelSelector{
                                    MatchLabels: map[string]string{
                                        "environment": cluster,
                                    },
                                },
                            },
                        },
                    },
                },
            },
        }
    } else {
        generators = []appv1alpha1.ApplicationSetGenerator{
            {
                Clusters: &appv1alpha1.ClusterGenerator{
                    Selector: metav1.LabelSelector{
                        MatchLabels: map[string]string{
                            "environment": cluster,
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
                    Project: "default",
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
                            Values:      modifiedValues,
                        },
                    },
                },
            },
        },
    }

  
// Create or update ApplicationSet in ArgoCD
_, applicationSetClient, err := argoClient.NewApplicationSetClient()
if err != nil {
    return nil, err
}

// Create the ApplicationSet
_, err = applicationSetClient.Create(context.TODO(), &applicationset.ApplicationSetCreateRequest{
    Applicationset: &appv1alpha1.ApplicationSet{
        ObjectMeta: metav1.ObjectMeta{
            Name:      appSet.Name,
            Namespace: appSet.Namespace,
        },
        Spec: appSet.Spec,
    },
})
if err != nil {
    return nil, err
}


fmt.Printf("Successfully applied ApplicationSet '%s' using ArgoCD\n", appSet.Name)

return &appSet.Status, nil
}

func DeleteApplicationSet(logger *zap.SugaredLogger, clientset kubernetes.Interface, dynamicClient dynamic.Interface, argoClient apiclient.Client, observed *v1alpha1.Service) (v1alpha1.ServiceStatus, error) {

	appSetName := observed.ObjectMeta.Name

	logger.Info("Attempting to delete ApplicationSet")

	// Check for dependent services
	dependentServices, err := checkDependentServices(dynamicClient, observed)
	if err != nil {
		return v1alpha1.ServiceStatus{
			State:   "Failed",
			Message: fmt.Sprintf("Error checking dependent services: %v", err),
		}, err
	}
	if len(dependentServices) > 0 {
		return v1alpha1.ServiceStatus{
			State:   "Blocked",
			Message: "Service has dependent services, cannot delete",
		}, nil
	}

	// Retry mechanism to delete ApplicationSet
	err = retry.OnError(retry.DefaultRetry, errors.IsInternalError, func() error {
		closer, applicationSetClient, err := argoClient.NewApplicationSetClient()
		if err != nil {
			return err
		}
		defer closer.Close()

		_, err = applicationSetClient.Delete(context.TODO(), &applicationset.ApplicationSetDeleteRequest{
			Name: appSetName,  
		})
		return err
	})
	if err != nil {
		return v1alpha1.ServiceStatus{
			State:   "Failed",
			Message: fmt.Sprintf("Error deleting ApplicationSet: %v", err),
		}, err
	}

	logger.Infof("Successfully deleted ApplicationSet '%s' using ArgoCD", appSetName)

	// If successful, remove finalizer
	err = kubernetespkg.RemoveFinalizer(logger, dynamicClient, observed.ObjectMeta.Name, observed.ObjectMeta.Namespace)
	if err != nil {
		logger.Errorf("Failed to remove finalizer for %s/%s: %v", observed.ObjectMeta.Namespace, observed.ObjectMeta.Name, err)
		return v1alpha1.ServiceStatus{
			State:   "Error",
			Message: fmt.Sprintf("Failed to remove finalizer: %v", err),
		}, err
	}

	return v1alpha1.ServiceStatus{
		State:   "Completed",
		Message: "Successfully deleted ApplicationSet",
	}, nil
}




// checkDependentServices checks if there are other services depending on the given service.
func checkDependentServices(dynamicClient dynamic.Interface, observed *v1alpha1.Service) ([]string, error) {
    var dependentServices []string
    services, err := dynamicClient.Resource(schema.GroupVersionResource{
        Group:    "alustan.io",
        Version:  "v1alpha1",
        Resource: "services",
    }).Namespace(observed.Namespace).List(context.TODO(), metav1.ListOptions{})
    if err != nil {
        return nil, err
    }

    for _, svc := range services.Items {
        serviceSpec, ok := svc.Object["spec"].(map[string]interface{})
        if !ok {
            continue
        }
        dependencies, exists := serviceSpec["dependencies"].(map[string]interface{})
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


func CheckApplicationSetHealth(logger *zap.SugaredLogger, argoClient apiclient.Client, appSetName, namespace string) (bool, error) {
	// Get the application set client and handle errors appropriately
	closer, appSetClient, err := argoClient.NewApplicationSetClient()
	if err != nil {
		logger.Error(err.Error())
		return false, err
	}
	// Ensure the closer is closed when the function exits
	defer closer.Close()

	// Retrieve the ApplicationSet
	appSet, err := appSetClient.Get(context.TODO(), &applicationset.ApplicationSetGetQuery{
		Name:      appSetName,
		AppsetNamespace: namespace,
	})
	if err != nil {
		logger.Error(err.Error())
		return false, err
	}

	// Check if the ApplicationSet status indicates it is healthy
	for _, condition := range appSet.Status.Conditions {
		if condition.Type == "Healthy" && condition.Status == "True" {
			return true, nil
		}
	}

	return false, nil
}




func ExtractDependencies(observed *v1alpha1.Service) []string {
    if observed.Spec.Dependencies.Service == nil {
        return nil
    }
    
    var dependencies []string
    for _, dep := range observed.Spec.Dependencies.Service {
        for _, serviceName := range dep {
            dependencies = append(dependencies, serviceName)
        }
    }
    
    return dependencies
}




func WaitForAllDependenciesHealth(
    logger *zap.SugaredLogger, 
    argoClient apiclient.Client, 
    dependencies []string, 
    namespace string, 
    retryInterval, timeout time.Duration,
) error {
    ticker := time.NewTicker(retryInterval)
    defer ticker.Stop() // Ensure the ticker is stopped when the function exits

    timeoutChan := time.After(timeout) // Channel that triggers after the timeout

    // Map to track the health status of dependencies
    dependencyStatus := make(map[string]bool)
    for _, dep := range dependencies {
        dependencyStatus[dep] = false // Initialize all dependencies as not healthy
    }

    for {
        select {
        case <-ticker.C: // Execute every retryInterval
            allHealthy := true
            for dep := range dependencyStatus {
                healthy, err := CheckApplicationSetHealth(logger, argoClient, dep, namespace)
                if err != nil {
                    return err // Return if an error occurs
                }
                if healthy {
                    dependencyStatus[dep] = true // Mark dependency as healthy
                } else {
                    allHealthy = false // Mark that not all dependencies are healthy
                    logger.Infof("Waiting for ApplicationSet %s to become healthy...", dep)
                }
            }
            if allHealthy {
                return nil // All dependencies are healthy
            }
        case <-timeoutChan: // Execute if the timeout elapses
            return fmt.Errorf("timed out waiting for dependencies to become healthy")
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
