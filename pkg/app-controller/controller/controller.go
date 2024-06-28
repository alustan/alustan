package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/alustan/pkg/app-controller/imagetag"
	"github.com/alustan/pkg/app-controller/kubernetes"
	"github.com/alustan/pkg/app-controller/pluginregistry"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	dynclient "k8s.io/client-go/dynamic"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/blang/semver/v4"
)

const (
	maxRetries = 5
)

type Controller struct {
	clientset    *k8sclient.Clientset
	dynClient    dynclient.Interface
	syncInterval time.Duration
	lastSyncTime  time.Time
	cache        map[string]string // Cache to store CRD states
    cacheMutex   sync.Mutex        // Mutex to protect cache access
}

// ApplicationSpec defines the desired state of Application
type ApplicationSpec struct {
	Provider          string            `json:"provider"` 
	Cluster           string            `json:"cluster"`
	Environment       string            `json:"environment"`
	Port              int               `json:"port"`
	Host              string            `json:"host,omitempty"`
	Strategy          string            `json:"strategy,omitempty"`
	Git               Git               `json:"git,omitempty"`
	ContainerRegistry ContainerRegistry `json:"containerRegistry,omitempty"`
	Config            map[string]string `json:"config,omitempty"`
}

// GitRepo defines the repository information
type Git struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Branch string `json:"branch,omitempty"`
}

// ContainerRegistry defines the container registry information
type ContainerRegistry struct {
	Provider        string `json:"provider"`
	ImageName       string `json:"imageName"`
	SemanticVersion string `json:"semanticVersion"`
}

// ApplicationStatus defines the observed state of Application
type ApplicationStatus struct {
	Message      string `json:"message,omitempty"`
	HealthStatus string `json:"healthStatus,omitempty"`
}

type ParentResource struct {
	ApiVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Metadata   metav1.ObjectMeta `json:"metadata"`
	Spec       ApplicationSpec   `json:"spec"`
}

type ChildResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              interface{}       `json:"spec,omitempty"`
	Status            ApplicationStatus `json:"status,omitempty"`
}

type SyncRequest struct {
	Parent     ParentResource `json:"parent"`
	Children   []ChildResource  `json:"children"`
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

func checkHealthStatus(children []ChildResource) ApplicationStatus {
	for _, child := range children {
		if child.Kind == "ApplicationSet" {
			status := child.Status.HealthStatus
			message := child.Status.Message
			if status == "Synced" {
				return ApplicationStatus{Message: message, HealthStatus: "Healthy"}
			} else {
				return ApplicationStatus{Message: message, HealthStatus: "Unhealthy"}
			}
		}
	}
	return ApplicationStatus{Message: "", HealthStatus: "Unknown"}
}

func (c *Controller) handleSyncRequest(observed SyncRequest) map[string]interface{} {
	name := observed.Parent.Metadata.Name
	cluster := observed.Parent.Spec.Cluster
	namespace := observed.Parent.Spec.Environment
	image := observed.Parent.Spec.ContainerRegistry.ImageName
	semanticVersion := observed.Parent.Spec.ContainerRegistry.SemanticVersion
	port := observed.Parent.Spec.Port
	host := observed.Parent.Spec.Host
	strategy := observed.Parent.Spec.Strategy
	gitOwner := observed.Parent.Spec.Git.Owner
	gitRepo := observed.Parent.Spec.Git.Repo
	provider := observed.Parent.Spec.ContainerRegistry.Provider
	config := observed.Parent.Spec.Config

	var configData string
	for key, value := range config {
		configData += fmt.Sprintf("  %s: %s\n", key, value)
	}

	var desiredResources v1alpha1.ApplicationSet

	// Initial status update: processing started
	initialStatus := map[string]interface{}{
		"state":   "Progressing",
		"message": "Starting processing",
	}
	c.updateStatus(observed, initialStatus)

	// Create registry client based on provider
	var registryClient imagetag.RegistryClientInterface
	var err error
	switch provider {

	case "ghcr":
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			status := c.errorResponse("creating GHCR client", fmt.Errorf("GITHUB_TOKEN environment variable is required for GHCR"))
			c.updateStatus(observed, status)
			return status
		}
		registryClient = imagetag.NewGHCRClient(token)
	case "docker":
		dockerToken := os.Getenv("DOCKER_TOKEN")
		if dockerToken == "" {
			status := c.errorResponse("creating docker client", fmt.Errorf("DOCKER_TOKEN environment variable is required for Docker"))
			c.updateStatus(observed, status)
			return status
		}
		registryClient = imagetag.NewDockerHubClient(dockerToken)
	default:
		status := c.errorResponse("creating registry client", fmt.Errorf("unknown container registry provider: %s", provider))
		c.updateStatus(observed, status)
		return status
	}

	// Fetch the latest image tag based on semantic versioning
	tags, err := registryClient.GetTags(image)
	if (err != nil) {
		status := c.errorResponse("fetching image tags", err)
		c.updateStatus(observed, status)
		return status
	}

	latestTag, err := c.getLatestTag(tags, semanticVersion)
	if (err != nil) {
		status := c.errorResponse("determining latest image tag", err)
		c.updateStatus(observed, status)
		return status
	}

	taggedImageName := fmt.Sprintf("%s:%s", image, latestTag)

	switch strategy {
	case "default", "canary", "preview":
		desiredResources, err = c.setupApplicationSet(provider, strategy, name, cluster, namespace, taggedImageName, strconv.Itoa(port), host, gitOwner, gitRepo, configData)
		if (err != nil) {
			status := c.errorResponse("setting up applicationset", err)
			c.updateStatus(observed, status)
			return status
		}
	default:
		return map[string]interface{}{
			"error": fmt.Sprintf("Invalid strategy: %s", strategy),
		}
	}

	status := checkHealthStatus(observed.Children)

	c.updateStatus(observed, map[string]interface{}{
		"message":      status.Message,
		"healthStatus": status.HealthStatus,
	})

	return map[string]interface{}{
		"status":   status,
		"children": desiredResources,
	}
}


func (c *Controller) setupApplicationSet(providerType, strategy, name, cluster, namespace, image, port, subDomain, gitOwner, gitRepo, configData string) (v1alpha1.ApplicationSet, error) {
	provider, err := pluginregistry.SetupPlugin(providerType, strategy, name, cluster, namespace, image, port, subDomain, gitOwner, gitRepo, configData)
	if err != nil {
		return v1alpha1.ApplicationSet{}, fmt.Errorf("error getting plugin: %v", err)
	}
	return provider.CreateApplicationSet(), nil
}

func (c *Controller) updateStatus(observed SyncRequest, status map[string]interface{}) {
	err := kubernetes.UpdateStatus(c.dynClient, observed.Parent.Metadata.Namespace, observed.Parent.Metadata.Name, status)
	if err != nil {
		log.Printf("Error updating status for %s: %v", observed.Parent.Metadata.Name, err)
	}
}

func (c *Controller) getLatestTag(tags []string, semanticVersion string) (string, error) {
	var validTags []semver.Version
	for _, tag := range tags {
		v, err := semver.ParseTolerant(tag)
		if err == nil {
			validTags = append(validTags, v)
		}
	}

	if len(validTags) == 0 {
		return "", fmt.Errorf("no valid semantic version tags found")
	}

	constraint, err := semver.ParseRange(semanticVersion)
	if err != nil {
		return "", fmt.Errorf("error parsing semantic version constraint: %w", err)
	}

	filteredTags := []semver.Version{}
	for _, tag := range validTags {
		if constraint(tag) {
			filteredTags = append(filteredTags, tag)
		}
	}

	if len(filteredTags) == 0 {
		log.Println("No tags matching the semantic version constraint found")
		return "", nil
	}

	sort.Slice(filteredTags, func(i, j int) bool {
		return filteredTags[i].GT(filteredTags[j])
	})
	latestTag := filteredTags[0]

	return latestTag.String(), nil
}

func (c *Controller) errorResponse(action string, err error) map[string]interface{} {
	log.Printf("Error %s: %v", action, err)
	return map[string]interface{}{
		"healthStatus":  "error",
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
		Resource: "applications",
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
