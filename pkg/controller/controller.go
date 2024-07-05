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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	kubernetespkg "github.com/alustan/pkg/kubernetes"
	"github.com/alustan/pkg/registry"
	"github.com/alustan/pkg/schematypes"
	"github.com/alustan/pkg/terraform"
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
	statusMap    map[string]chan schematypes.ParentResourceStatus // Map to store status channels
	statusMutex  sync.Mutex                                      // Mutex to protect status map access
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
		statusMap:    make(map[string]chan schematypes.ParentResourceStatus), // Initialize status map
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

	// Store the status channel in the map
	c.statusMutex.Lock()
	c.statusMap[key] = statusChan
	c.statusMutex.Unlock()

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

	// Enqueue SyncRequest for processing
	c.enqueue(key)

	// Wait for the final status
	finalStatus := <-statusChan

	// Clean up the status channel map
	c.statusMutex.Lock()
	delete(c.statusMap, key)
	c.statusMutex.Unlock()

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
	case string:
		key = o
	}

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object: %v, %w", obj, err))
		return
	}

	c.workqueue.AddRateLimited(key)
}


func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %T", obj))
			return nil
		}

		if err := c.syncHandler(key); err != nil {
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}

		c.workqueue.Forget(obj)
		log.Printf("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) updateStatus(observed schematypes.SyncRequest, status schematypes.ParentResourceStatus) error {
	err := kubernetespkg.UpdateStatus(c.dynClient, observed.Parent.Metadata.Namespace, observed.Parent.Metadata.Name, status)
	if err != nil {
		log.Printf("Error updating status for %s: %v", observed.Parent.Metadata.Name, err)
	}
	return err
}


func (c *Controller) syncHandler(key string) error {
	// Retrieve the observed SyncRequest from the map
	c.mapMutex.Lock()
	observed, exists := c.observedMap[key]
	c.mapMutex.Unlock()

	if !exists {
		return fmt.Errorf("no observed SyncRequest found for key %s", key)
	}

	// Process the SyncRequest
	finalStatus, err := c.handleSyncRequest(observed)
	if err != nil {
		finalStatus.State = "Error"
		finalStatus.Message = err.Error()
	}

	updateErr := c.updateStatus(observed, finalStatus)
		if updateErr != nil {
			log.Printf("Error updating status for %s: %v", observed.Parent.Metadata.Name, updateErr)
		}

	// Send the final status through the status channel
	c.statusMutex.Lock()
	statusChan, exists := c.statusMap[key]
	c.statusMutex.Unlock()

	if exists {
		statusChan <- finalStatus
	}

	return nil
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
