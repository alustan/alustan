package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubernetespkg "github.com/alustan/pkg/kubernetes"
	"github.com/alustan/pkg/schematypes"
	"github.com/alustan/pkg/terraform"
	"github.com/alustan/pkg/registry"
	"github.com/alustan/pkg/util"
	
)

type Controller struct {
	clientset    kubernetes.Interface
	dynClient    dynamic.Interface
	syncInterval time.Duration
	lastSyncTime time.Time
	Cache        map[string]string // Cache to store CRD states
	cacheMutex   sync.Mutex        // Mutex to protect cache access
	workqueue    workqueue.RateLimitingInterface
	observedMap  map[string]schematypes.SyncRequest // Map to store observed SyncRequests
	mapMutex     sync.Mutex                         // Mutex to protect map access
}

type SyncRequestWrapper struct {
	SyncRequest schematypes.SyncRequest
}

func (w *SyncRequestWrapper) GetObjectMeta() metav1.Object {
	return &w.SyncRequest.Parent.Metadata
}

func NewController(clientset kubernetes.Interface, dynClient dynamic.Interface, syncInterval time.Duration) *Controller {
	return &Controller{
		clientset:    clientset,
		dynClient:    dynClient,
		syncInterval: syncInterval,
		lastSyncTime: time.Now().Add(-syncInterval), // Initialize to allow immediate first run
		Cache:        make(map[string]string),       // Initialize cache
		workqueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "terraforms"),
		observedMap:  make(map[string]schematypes.SyncRequest), // Initialize observed map
	}
}

func NewInClusterController(syncInterval time.Duration) *Controller {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error creating in-cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating Kubernetes clientset: %v", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating dynamic Kubernetes client: %v", err)
	}

	return NewController(clientset, dynClient, syncInterval)
}

func (c *Controller) ServeHTTP(r *gin.Context) {
	var observed schematypes.SyncRequest
	err := json.NewDecoder(r.Request.Body).Decode(&observed)
	if err != nil {
		response := gin.H{"body": err.Error()}
		r.Writer.Header().Set("Content-Type", "application/json")
		r.JSON(http.StatusBadRequest, response)
		return
	}
	defer func() {
		if err := r.Request.Body.Close(); err != nil {
			log.Printf("Error closing request body: %v", err)
		}
	}()

	key := fmt.Sprintf("%s/%s", observed.Parent.Metadata.Namespace, observed.Parent.Metadata.Name)

	// Create a channel to receive the final status
	statusChan := make(chan schematypes.ParentResourceStatus)

	// Store the observed SyncRequest in the map
	c.mapMutex.Lock()
	c.observedMap[key] = observed
	c.mapMutex.Unlock()

	// Check if the CRD has changed before processing
	if !c.IsCRDChanged(observed) {
		finalStatus := schematypes.ParentResourceStatus{
			State:   "Unchanged",
			Message: "No changes detected in the CRD",
		}
		r.Writer.Header().Set("Content-Type", "application/json")
		r.JSON(http.StatusOK, gin.H{"body": finalStatus})
		return
	}
	// Enqueue SyncRequest for processing and pass the status channel
	go func() {
		status, err := c.handleSyncRequest(observed)
		if err != nil {
			log.Printf("Error handling sync request: %v", err)
			statusChan <- status
			return
		}
		statusChan <- status
	}()

	// Wait for the final status
	finalStatus := <-statusChan

	// Check for error in the status and send an appropriate HTTP response
	if finalStatus.State == "Error" {
		r.Writer.Header().Set("Content-Type", "application/json")
		r.JSON(http.StatusBadRequest, gin.H{"body": finalStatus})
		return
	}
    
	// Update the cache as the CRD has changed
	c.UpdateCache(observed)
	
	// Return the response in the expected format
	response := gin.H{"body": finalStatus}
	r.Writer.Header().Set("Content-Type", "application/json")
	r.JSON(http.StatusOK, response)
}

func (c *Controller) handleSyncRequest(observed schematypes.SyncRequest) (schematypes.ParentResourceStatus, error) {
	envVars := util.ExtractEnvVars(observed.Parent.Spec.Variables)
	secretName := fmt.Sprintf("%s-container-secret", observed.Parent.Metadata.Name)
	log.Printf("Observed Parent Spec: %+v", observed.Parent.Spec)

	commonStatus := schematypes.ParentResourceStatus{
		State:   "Progressing",
		Message: "Starting processing",
	}

    // Handle script content
	scriptContent, scriptContentStatus := terraform.GetScriptContent(observed)
	commonStatus = mergeStatuses(commonStatus, scriptContentStatus)
	if scriptContentStatus.State == "Error" {
		return commonStatus, fmt.Errorf("error getting script content")
	}

	// Handle tagged image name
	taggedImageName, taggedImageStatus := registry.GetTaggedImageName(observed, scriptContent, c.clientset)
	commonStatus = mergeStatuses(commonStatus, taggedImageStatus)
	if taggedImageStatus.State == "Error" {
	  return commonStatus, fmt.Errorf("error getting tagged image name")
	}

	log.Printf("taggedImageName: %v", taggedImageName)

	// Handle ExecuteTerraform
	execTerraformStatus := terraform.ExecuteTerraform(c.clientset, observed, scriptContent, taggedImageName, secretName, envVars)
	commonStatus = mergeStatuses(commonStatus, execTerraformStatus)
    
	if execTerraformStatus.State == "Error" {
		return commonStatus, fmt.Errorf("error executing terraform")
	  }
	

	return commonStatus, nil
}

func (c *Controller) IsCRDChanged(observed schematypes.SyncRequest) bool {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	newHash := HashSpec(observed.Parent.Spec)
	oldHash, exists := c.Cache[observed.Parent.Metadata.Name]

	return !exists || newHash != oldHash
}

func (c *Controller) UpdateCache(observed schematypes.SyncRequest) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	newHash := HashSpec(observed.Parent.Spec)
	c.Cache[observed.Parent.Metadata.Name] = newHash
}

func HashSpec(spec schematypes.TerraformConfigSpec) string {
	hash := sha256.New()
	data, err := json.Marshal(spec)
	if err != nil {
		log.Printf("Error hashing spec: %v", err)
		return ""
	}
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

func mergeStatuses(baseStatus, newStatus schematypes.ParentResourceStatus) schematypes.ParentResourceStatus {
	if newStatus.State != "" {
		baseStatus.State = newStatus.State
	}
	if newStatus.Message != "" {
		baseStatus.Message = newStatus.Message
	}
	if newStatus.Output != nil {
		if baseStatus.Output == nil {
			baseStatus.Output = make(map[string]interface{})
		}
		for k, v := range newStatus.Output {
			baseStatus.Output[k] = v
		}
	}
	if newStatus.PostDeployOutput != nil {
		if baseStatus.PostDeployOutput == nil {
			baseStatus.PostDeployOutput = make(map[string]interface{})
		}
		for k, v := range newStatus.PostDeployOutput {
			baseStatus.PostDeployOutput[k] = v
		}
	}
	if newStatus.IngressURLs != nil {
		if baseStatus.IngressURLs == nil {
			baseStatus.IngressURLs = make(map[string]interface{})
		}
		for k, v := range newStatus.IngressURLs {
			baseStatus.IngressURLs[k] = v
		}
	}
	if newStatus.Credentials != nil {
		if baseStatus.Credentials == nil {
			baseStatus.Credentials = make(map[string]interface{})
		}
		for k, v := range newStatus.Credentials {
			baseStatus.Credentials[k] = v
		}
	}
	if newStatus.Finalized {
		baseStatus.Finalized = newStatus.Finalized
	}
	return baseStatus
}


func (c *Controller) enqueue(obj interface{}) {
	var key string
	var err error

	switch o := obj.(type) {
	case schematypes.SyncRequest:
		wrapped := SyncRequestWrapper{o}
		key, err = cache.MetaNamespaceKeyFunc(&wrapped)
	case *SyncRequestWrapper:
		key, err = cache.MetaNamespaceKeyFunc(o)
	case string:
		key = o
	default:
		log.Printf("Unsupported object type passed to enqueue: %T", obj)
		return
	}

	if err != nil {
		log.Printf("Error creating key for object: %v", err)
		return
	}
	c.workqueue.Add(key)
}


func (c *Controller) updateStatus(observed schematypes.SyncRequest, status schematypes.ParentResourceStatus) error {
	err := kubernetespkg.UpdateStatus(c.dynClient, observed.Parent.Metadata.Namespace, observed.Parent.Metadata.Name, status)
	if err != nil {
		log.Printf("Error updating status for %s: %v", observed.Parent.Metadata.Name, err)
	}
	return err
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

func (c *Controller) processNextItem() bool {
	key, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}
	defer c.workqueue.Done(key)

	observed := c.getObservedFromKey(key.(string))
	if isNilSyncRequest(observed) {
		log.Printf("SyncRequest not found for key: %s", key)
		c.workqueue.Forget(key)
		return true
	}

	log.Printf("Processing item with key: %s", key)

	status, err := c.handleSyncRequest(observed)
	if err != nil {
		log.Printf("Error handling sync request: %v", err)
		// Update status to reflect the error
		updateErr := c.updateStatus(observed, status)
		if updateErr != nil {
			log.Printf("Error updating status for %s: %v", observed.Parent.Metadata.Name, updateErr)
		}
		// Re-enqueue the item for further processing
		c.workqueue.AddRateLimited(key)
		return true
	}

	// Remove the item from the workqueue
	c.workqueue.Forget(key)

	// Update the status in Kubernetes
	updateErr := c.updateStatus(observed, status)
	if updateErr != nil {
		log.Printf("Error updating status for %s: %v", observed.Parent.Metadata.Name, updateErr)
		// Re-enqueue the item if status update fails
		c.workqueue.AddRateLimited(key)
		return true
	}

	// Update the cache if the CRD has changed
	if c.IsCRDChanged(observed) {
		c.UpdateCache(observed)
	}

	log.Printf("Successfully processed item with key: %s", key)
	return true
}


func isNilSyncRequest(observed schematypes.SyncRequest) bool {
	return observed.Parent.Metadata.Name == "" && observed.Parent.Metadata.Namespace == ""
}

func (c *Controller) getObservedFromKey(key string) schematypes.SyncRequest {
	c.mapMutex.Lock()
	defer c.mapMutex.Unlock()

	observed, exists := c.observedMap[key]
	if !exists {
		log.Printf("SyncRequest not found for key: %s", key)
		return schematypes.SyncRequest{}
	}
	return observed
}

func (c *Controller) Reconcile(stopCh <-chan struct{}) {
	ticker := time.NewTicker(c.syncInterval)
	go c.runWorker()
	for {
		select {
		case <-ticker.C:
			c.reconcileLoop()
			c.lastSyncTime = time.Now()
		case <-stopCh:
			ticker.Stop()
			c.workqueue.ShutDown()
			return
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

	for _, item := range resourceList.Items {
		c.enqueue(item.GetName())
	}
}
