package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/alustan/pkg/terraform-controller/container"
	"github.com/alustan/pkg/terraform-controller/kubernetes"
	"github.com/alustan/pkg/util"
	containers "github.com/alustan/pkg/containers"
	"github.com/alustan/pkg/terraform-controller/pluginregistry"
    
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	dynclient "k8s.io/client-go/dynamic"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	maxRetries = 5
)

type Controller struct {
	clientset   *k8sclient.Clientset
	dynClient   dynclient.Interface
	syncInterval time.Duration
	lastSyncTime  time.Time
	cache        map[string]string // Cache to store CRD states
    cacheMutex   sync.Mutex        // Mutex to protect cache access
}

type TerraformConfigSpec struct {
	Provider           string            `json:"provider"`
	Variables          map[string]string `json:"variables"`
	Scripts            Scripts           `json:"scripts"`
	GitRepo            GitRepo           `json:"gitRepo"`
	ContainerRegistry  ContainerRegistry `json:"containerRegistry"`
}

type Scripts struct {
	Deploy  string `json:"deploy"`
	Destroy string `json:"destroy"`
}

type GitRepo struct {
	URL    string `json:"url"`
	Branch string `json:"branch"`
}

type ContainerRegistry struct {
	ImageName string `json:"imageName"`
}

type ParentResource struct {
	ApiVersion string              `json:"apiVersion"`
	Kind       string              `json:"kind"`
	Metadata   metav1.ObjectMeta   `json:"metadata"`
	Spec       TerraformConfigSpec `json:"spec"`
}

type SyncRequest struct {
	Parent     ParentResource `json:"parent"`
	Finalizing bool           `json:"finalizing"`
}

func NewController(clientset *k8sclient.Clientset, dynClient dynclient.Interface, syncInterval time.Duration) *Controller {
	return &Controller{
		clientset:    clientset,
		dynClient:    dynClient,
		syncInterval: syncInterval,
		lastSyncTime: time.Now().Add(-syncInterval), // Initialize to allow immediate first run
		cache:        make(map[string]string),       // Initialize cache
	}
}

func NewInClusterController(syncInterval time.Duration) *Controller {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error creating in-cluster config: %v", err)
	}

	clientset, err := k8sclient.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating Kubernetes clientset: %v", err)
	}

	dynClient, err := dynclient.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating dynamic Kubernetes client: %v", err)
	}

	return NewController(clientset, dynClient, syncInterval)
}

func (c *Controller) ServeHTTP(r *gin.Context) {
	var observed SyncRequest
	err := json.NewDecoder(r.Request.Body).Decode(&observed)
	if err != nil {
		r.String(http.StatusBadRequest, err.Error())
		return
	}
	defer func() {
		if err := r.Request.Body.Close(); err != nil {
			log.Printf("Error closing request body: %v", err)
		}
	}()

	 // Trigger immediate sync on new CRD or CRD update
	 if c.isCRDChanged(observed) {
        response := c.handleSyncRequest(observed)
        c.updateCache(observed)
        r.Writer.Header().Set("Content-Type", "application/json")
        r.JSON(http.StatusOK, gin.H{"body": response})
    } else {
       r.String(http.StatusOK, "No changes detected, no action taken")
    }

}

func (c *Controller) isCRDChanged(observed SyncRequest) bool {
    c.cacheMutex.Lock()
    defer c.cacheMutex.Unlock()

    cachedState, exists := c.cache[observed.Parent.Metadata.Name]
    if !exists {
        return true // New CRD
    }

    currentState, err := json.Marshal(observed.Parent.Spec)
    if err != nil {
        log.Printf("Error marshalling CRD spec: %v", err)
        return false
    }

    return cachedState != string(currentState)
}

func (c *Controller) updateCache(observed SyncRequest) {
    c.cacheMutex.Lock()
    defer c.cacheMutex.Unlock()

    currentState, err := json.Marshal(observed.Parent.Spec)
    if err != nil {
        log.Printf("Error marshalling CRD spec: %v", err)
        return
    }

    c.cache[observed.Parent.Metadata.Name] = string(currentState)
}

func (c *Controller) handleSyncRequest(observed SyncRequest) map[string]interface{} {
	envVars := c.extractEnvVars(observed.Parent.Spec.Variables)
	secretName := fmt.Sprintf("%s-container-secret", observed.Parent.Metadata.Name)
	log.Printf("Observed Parent Spec: %+v", observed.Parent.Spec)

	// Initial status update: processing started
	initialStatus := map[string]interface{}{
		"state":   "Progressing",
		"message": "Starting processing",
	}
	c.updateStatus(observed, initialStatus)

	// Determine the script content based on whether it's finalizing or not
	var scriptContent string
	if observed.Finalizing {
		scriptContent = observed.Parent.Spec.Scripts.Destroy
	} else {
		scriptContent = observed.Parent.Spec.Scripts.Deploy
	}

	if scriptContent == "" {
		status := c.errorResponse("executing script", fmt.Errorf("script is missing"))
		c.updateStatus(observed, status)
		return status
	}

	// Retrieve the tagged image name from ConfigMap if finalizing
	var taggedImageName string
	if observed.Finalizing {
		var err error
		taggedImageName, err = c.getTaggedImageNameFromConfigMap(observed.Parent.Metadata.Namespace, observed.Parent.Metadata.Name)
		if err != nil {
			status := c.errorResponse("retrieving tagged image name", err)
			c.updateStatus(observed, status)
			return status
		}
	} else {
		// Build and tag image if not finalizing
		repoDir := filepath.Join("/workspace", "tmp", observed.Parent.Metadata.Name)
		sshKey := os.Getenv("GIT_SSH_SECRET")

		dockerfileAdditions, providerExists, err := c.setupProvider(observed.Parent.Spec.Provider, observed.Parent.Metadata.Labels["workspace"], observed.Parent.Metadata.Labels["region"])
		if err != nil {
			status := c.errorResponse("setting up backend", err)
			c.updateStatus(observed, status)
			return status
		}

		configMapName, err := container.CreateDockerfileConfigMap(c.clientset, observed.Parent.Metadata.Name, observed.Parent.Metadata.Namespace, dockerfileAdditions, providerExists)
		if err != nil {
			status := c.errorResponse("creating Dockerfile ConfigMap", err)
			c.updateStatus(observed, status)
			return status
		}

		encodedDockerConfigJSON := os.Getenv("CONTAINER_REGISTRY_SECRET")
		if encodedDockerConfigJSON == "" {
			log.Println("Environment variable CONTAINER_REGISTRY_SECRET is not set")
			status := c.errorResponse("creating Docker config secret", fmt.Errorf("CONTAINER_REGISTRY_SECRET is not set"))
			c.updateStatus(observed, status)
			return status
		}
		
		err = containers.CreateDockerConfigSecret(c.clientset, secretName, observed.Parent.Metadata.Namespace, encodedDockerConfigJSON)
		if err != nil {
			status := c.errorResponse("creating Docker config secret", err)
			c.updateStatus(observed, status)
			return status
		}

		pvcName := fmt.Sprintf("pvc-%s", observed.Parent.Metadata.Name)
		err = containers.EnsurePVC(c.clientset, observed.Parent.Metadata.Namespace, pvcName)
		if err != nil {
			status := c.errorResponse("creating PVC", err)
			c.updateStatus(observed, status)
			return status
		}

		taggedImageName, _, err = c.buildAndTagImage(observed, configMapName, repoDir, sshKey, secretName, pvcName)
		if err != nil {
			status := c.errorResponse("creating build job", err)
			c.updateStatus(observed, status)
			return status
		}
	}

	if observed.Finalizing {
		c.updateStatus(observed, map[string]interface{}{
			"state":   "Progressing",
			"message": "Running Terraform Destroy",
		})

		status := c.runDestroy(observed, scriptContent, taggedImageName, secretName, envVars)
		c.updateStatus(observed, status)

		finalStatus := map[string]interface{}{
			"state":   "Completed",
			"message": "Destroy process completed successfully",
		}
		c.updateStatus(observed, finalStatus)
		
		finalStatus["finalized"] = true
		return finalStatus
	}

	c.updateStatus(observed, map[string]interface{}{
		"state":   "Progressing",
		"message": "Running Terraform Apply",
	})

	status := c.runApply(observed, scriptContent, taggedImageName, secretName, envVars)
	c.updateStatus(observed, status)

	if observed.Parent.Spec.Provider != "" {
		resources, err := c.executePlugin(observed.Parent.Spec.Provider, observed.Parent.Metadata.Labels["workspace"], observed.Parent.Metadata.Labels["region"])
		if err != nil {
			finalStatus := c.errorResponse("executing plugin", err)
			c.updateStatus(observed, finalStatus)
			return finalStatus
		}

		pluginStatus := map[string]interface{}{
			"state":         "Completed",
			"message":       "Processing completed successfully",
			"cloudResources": resources,
		}
		c.updateStatus(observed, pluginStatus)
		return pluginStatus
	}

	finalStatus := map[string]interface{}{
		"state":   "Completed",
		"message": "Processing completed successfully",
	}
	c.updateStatus(observed, finalStatus)
	return finalStatus
}

func (c *Controller) getTaggedImageNameFromConfigMap(namespace, name string) (string, error) {
	configMapName := fmt.Sprintf("%s-tagged-image", name)
	configMap, err := c.clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMapName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get ConfigMap: %v", err)
	}
	taggedImageName, ok := configMap.Data["lastTaggedImage"]
	if !ok {
		return "", fmt.Errorf("tagged image name not found in ConfigMap")
	}
	return taggedImageName, nil
}
func (c *Controller) updateStatus(observed SyncRequest, status map[string]interface{}) {
	err := kubernetes.UpdateStatus(c.dynClient, observed.Parent.Metadata.Namespace, observed.Parent.Metadata.Name, status)
	if err != nil {
		log.Printf("Error updating status for %s: %v", observed.Parent.Metadata.Name, err)
	}
}

func (c *Controller) extractEnvVars(variables map[string]string) map[string]string {
	if variables == nil {
		return nil
	}
	return util.ExtractEnvVars(variables)
}

func (c *Controller) setupProvider(providerType, workspace, region string) (string, bool, error) {
	if providerType == "" {
		// No provider specified, return without error
		return "", false, nil
	}
	provider, err := pluginregistry.SetupPlugin(providerType, workspace, region)
	if err != nil {
		return "", false, err
	}
	return provider.GetDockerfileAdditions(), true, nil
}

func (c *Controller) executePlugin(providerType, workspace, region string) (map[string]interface{}, error) {
	provider, err := pluginregistry.SetupPlugin(providerType, workspace, region)
	if err != nil {
		return nil, fmt.Errorf("error getting plugin: %v", err)
	}
	return provider.Execute()
}

func (c *Controller) buildAndTagImage(observed SyncRequest, configMapName, repoDir, sshKey, secretName, pvcName string) (string, string, error) {
	imageName := observed.Parent.Spec.ContainerRegistry.ImageName

	taggedImageName, _, err := container.CreateBuildPod(c.clientset,
		observed.Parent.Metadata.Name,
		observed.Parent.Metadata.Namespace,
		configMapName,
		imageName,
		secretName,
		repoDir,
		observed.Parent.Spec.GitRepo.URL,
		observed.Parent.Spec.GitRepo.Branch,
		sshKey,
		pvcName)
	if err != nil {
		return "", "", err
	}

	// Update the ConfigMap with the tagged image name
	err = c.updateTaggedImageConfigMap(observed.Parent.Metadata.Namespace, observed.Parent.Metadata.Name, taggedImageName)
	if err != nil {
		return "", "", err
	}

	return taggedImageName, "", nil
}
func (c *Controller) updateTaggedImageConfigMap(namespace, name, taggedImageName string) error {
	configMapData := map[string]string{
		"lastTaggedImage": taggedImageName,
	}
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-tagged-image", name),
			Namespace: namespace,
		},
		Data: configMapData,
	}

	_, err := c.clientset.CoreV1().ConfigMaps(namespace).Create(context.Background(), configMap, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create ConfigMap: %v", err)
	}
	return nil
}

func (c *Controller) runDestroy(observed SyncRequest, scriptContent, taggedImageName, secretName string, envVars map[string]string) map[string]interface{} {
	// Call to run Terraform destroy
	var terraformErr error
	

	for i := 0; i < maxRetries; i++ {
		_, terraformErr = container.CreateRunPod(c.clientset, observed.Parent.Metadata.Name, observed.Parent.Metadata.Namespace, scriptContent, envVars, taggedImageName, secretName)
		
		if terraformErr == nil {
			break
		}
		log.Printf("Retrying Terraform command due to error: %v", terraformErr)
		time.Sleep(2 * time.Minute)
	}
	status := map[string]interface{}{
		"state":   "Success",
		"message": "Terraform destroyed successfully",
	}
	if terraformErr != nil {
		status["state"] = "Failed"
		status["message"] = terraformErr.Error()
		return status
	}

	return status
}


func (c *Controller) runApply(observed SyncRequest, scriptContent, taggedImageName, secretName string, envVars map[string]string) map[string]interface{} {
	var terraformErr error
	var podName string

	for i := 0; i < maxRetries; i++ {
		podName, terraformErr = container.CreateRunPod(c.clientset, observed.Parent.Metadata.Name, observed.Parent.Metadata.Namespace, scriptContent, envVars, taggedImageName, secretName)
		
		if terraformErr == nil {
			break
		}
		log.Printf("Retrying Terraform command due to error: %v", terraformErr)
		time.Sleep(2 * time.Minute)
	}

	status := map[string]interface{}{
		"state":   "Success",
		"message": "Terraform applied successfully",
	}
	if terraformErr != nil {
		status["state"] = "Failed"
		status["message"] = terraformErr.Error()
		return status
	}

	// Wait for the pod to complete and retrieve the logs
	output, err := container.WaitForPodCompletion(c.clientset, observed.Parent.Metadata.Namespace, podName)
	if err != nil {
		status["state"] = "Failed"
		status["message"] = fmt.Sprintf("Error retrieving Terraform output: %v", err)
		return status
	}

	status["output"] = output

	// Retrieve ingress URLs and include them in the status
	ingressURLs, err := kubernetes.GetAllIngressURLs(c.clientset)
	if err != nil {
		status["state"] = "Failed"
		status["message"] = fmt.Sprintf("Error retrieving Ingress URLs: %v", err)
		return status
	}
	status["ingressURLs"] = ingressURLs

	// Retrieve credentials and include them in the status
	credentials, err := kubernetes.FetchCredentials(c.clientset)
	if err != nil {
		status["state"] = "Failed"
		status["message"] = fmt.Sprintf("Error retrieving credentials: %v", err)
		return status
	}
	status["credentials"] = credentials

	return status
}


func (c *Controller) errorResponse(action string, err error) map[string]interface{} {
	log.Printf("Error %s: %v", action, err)
	return map[string]interface{}{
		"state":   "error",
		"message": fmt.Sprintf("Error %s: %v", action, err),
	}
}

func (c *Controller) Reconcile() {
    ticker := time.NewTicker(c.syncInterval)
    for {
        select {
        case <-ticker.C:
            c.reconcileLoop()
            c.lastSyncTime = time.Now()
        }
    }
}


func (c *Controller) reconcileLoop() {
	log.Println("Starting reconciliation loop")
	resourceList, err := c.dynClient.Resource(schema.GroupVersionResource{
		Group:    "alustan.io",
		Version:  "v1alpha1",
		Resource: "terraforms",
	}).Namespace("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Error fetching Terraform resources: %v", err)
		return
	}

	log.Printf("Fetched %d Terraform resources", len(resourceList.Items))

	var wg sync.WaitGroup
	for _, item := range resourceList.Items {
		wg.Add(1)
		go func(item unstructured.Unstructured) {
			defer wg.Done()
			var observed SyncRequest
			raw, err := item.MarshalJSON()
			if err != nil {
				log.Printf("Error marshalling item: %v", err)
				return
			}
			err = json.Unmarshal(raw, &observed)
			if err != nil {
				log.Printf("Error unmarshalling item: %v", err)
				return
			}

			log.Printf("Handling resource: %s", observed.Parent.Metadata.Name)
			c.handleSyncRequest(observed)
		}(item)
	}
	wg.Wait()
}
