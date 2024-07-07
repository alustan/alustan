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
	"github.com/alustan/api/v1alpha1"
	"github.com/alustan/pkg/terraform"
	"github.com/alustan/pkg/util"
)

type Controller struct {
	Clientset    kubernetes.Interface
	dynClient    dynamic.Interface
	syncInterval time.Duration
	lastSyncTime time.Time
	Cache        map[string]string // Cache to store CRD states
	cacheMutex   sync.Mutex        // Mutex to protect cache access
	workqueue    workqueue.RateLimitingInterface
	observedMap  map[string]v1alpha1.SyncRequest // Map to store observed SyncRequests
	mapMutex     sync.Mutex                         // Mutex to protect map access
	statusMap    map[string]chan v1alpha1.ParentResourceStatus // Map to store status channels
	statusMutex  sync.Mutex                                      // Mutex to protect status map access
	
}

type SyncRequestWrapper struct {
	SyncRequest v1alpha1.SyncRequest
}

func (w *SyncRequestWrapper) GetObjectMeta() metav1.Object {
	return &w.SyncRequest.Parent.ObjectMeta
}

func NewController(clientset kubernetes.Interface, dynClient dynamic.Interface, syncInterval time.Duration) *Controller {
    ctrl := &Controller{
        Clientset:    clientset,
        dynClient:    dynClient,
        syncInterval: syncInterval,
        lastSyncTime: time.Now().Add(-syncInterval), // Initialize to allow immediate first run
        Cache:        make(map[string]string),       // Initialize cache
        workqueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "terraforms"),
        observedMap:  make(map[string]v1alpha1.SyncRequest), // Initialize observed map
        statusMap:    make(map[string]chan v1alpha1.ParentResourceStatus), // Initialize status map
    }

    return ctrl
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
	var observed v1alpha1.SyncRequest
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

	// Log the incoming SyncRequest
	log.Printf("Received SyncRequest: %+v", observed)

	key := fmt.Sprintf("%s/%s", observed.Parent.ObjectMeta.Namespace, observed.Parent.ObjectMeta.Name)

	// Create a channel to receive the final status
	statusChan := make(chan v1alpha1.ParentResourceStatus)

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
		c.mapMutex.Lock()
		existingStatus := c.statusMap[key]
		c.mapMutex.Unlock()
		
		// Enqueue the request if finalizing
		if observed.Finalizing {
			c.enqueue(key)
		} else {
			// Return the current status if the CRD hasn't changed
			finalStatus := <-existingStatus
			desired := v1alpha1.SyncResponse{
				Status:    finalStatus,
				Finalized: finalStatus.Finalized,
			}

			// Log the SyncResponse
			log.Printf("Sending SyncResponse: %+v", desired)
			r.Writer.Header().Set("Content-Type", "application/json")
			r.JSON(http.StatusOK, gin.H{"body": desired})
			return
		}
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
		desired := v1alpha1.SyncResponse{
			Status:    finalStatus,
			Finalized: false,
		}
		// Log the SyncResponse
		log.Printf("Sending SyncResponse: %+v", desired)
		r.Writer.Header().Set("Content-Type", "application/json")
		r.JSON(http.StatusBadRequest, gin.H{"body": desired})
		return
	}

	// Update the cache as the CRD has changed
	c.UpdateCache(observed)

	// Extract the finalized field from the status
	finalized := finalStatus.Finalized

	// Construct the desired state
	desired := v1alpha1.SyncResponse{
		Status:    finalStatus,
		Finalized: finalized,
	}

	// Log the SyncResponse
	log.Printf("Sending SyncResponse: %+v", desired)

	// Return the response in the expected format
	response := gin.H{"body": desired}
	r.Writer.Header().Set("Content-Type", "application/json")
	r.JSON(http.StatusOK, response)
}


func (c *Controller) handleSyncRequest(observed v1alpha1.SyncRequest) (v1alpha1.ParentResourceStatus, error) {
	envVars := util.ExtractEnvVars(observed.Parent.Spec.Variables)
	secretName := fmt.Sprintf("%s-container-secret", observed.Parent.ObjectMeta.Name)
	log.Printf("Observed Parent Spec: %+v", observed.Parent.Spec)

	commonStatus := v1alpha1.ParentResourceStatus{
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
	taggedImageName, taggedImageStatus := registry.GetTaggedImageName(observed, scriptContent, c.Clientset)
	commonStatus = mergeStatuses(commonStatus, taggedImageStatus)
	if taggedImageStatus.State == "Error" {
		return commonStatus, fmt.Errorf("error getting tagged image name")
	}

	log.Printf("taggedImageName: %v", taggedImageName)

	// Handle ExecuteTerraform
	execTerraformStatus := terraform.ExecuteTerraform(c.Clientset, observed, scriptContent, taggedImageName, secretName, envVars)
	commonStatus = mergeStatuses(commonStatus, execTerraformStatus)

	if execTerraformStatus.State == "Error" {
		return commonStatus, fmt.Errorf("error executing terraform")
	}

	return commonStatus, nil
}

func (c *Controller) IsCRDChanged(observed v1alpha1.SyncRequest) bool {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	newHash := HashSpec(observed.Parent.Spec)
	oldHash, exists := c.Cache[observed.Parent.ObjectMeta.Name]

	return !exists || newHash != oldHash
}

func (c *Controller) UpdateCache(observed v1alpha1.SyncRequest) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	newHash := HashSpec(observed.Parent.Spec)
	c.Cache[observed.Parent.ObjectMeta.Name] = newHash
}

func HashSpec(spec v1alpha1.TerraformConfigSpec) string {
	hash := sha256.New()
	data, err := json.Marshal(spec)
	if err != nil {
		log.Printf("Error hashing spec: %v", err)
		return ""
	}
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

func mergeStatuses(baseStatus, newStatus v1alpha1.ParentResourceStatus) v1alpha1.ParentResourceStatus {
    if newStatus.State != "" {
        baseStatus.State = newStatus.State
    }
    if newStatus.Message != "" {
        baseStatus.Message = newStatus.Message
    }
    if newStatus.Output != nil {
        if baseStatus.Output == nil {
            baseStatus.Output = make(json.RawMessage, 0) // Initialize as RawMessage
        }
        // Convert newStatus.Output to []byte and assign to baseStatus.Output
        data, err := json.Marshal(newStatus.Output)
        if err != nil {
            // Handle error if necessary
        }
        baseStatus.Output = json.RawMessage(data)
    }
    if newStatus.PostDeployOutput != nil {
        if baseStatus.PostDeployOutput == nil {
            baseStatus.PostDeployOutput = make(json.RawMessage, 0) // Initialize as RawMessage
        }
        // Convert newStatus.PostDeployOutput to []byte and assign to baseStatus.PostDeployOutput
        data, err := json.Marshal(newStatus.PostDeployOutput)
        if err != nil {
            // Handle error if necessary
        }
        baseStatus.PostDeployOutput = json.RawMessage(data)
    }
    if newStatus.IngressURLs != nil {
        if baseStatus.IngressURLs == nil {
            baseStatus.IngressURLs = make(json.RawMessage, 0) // Initialize as RawMessage
        }
        // Convert newStatus.IngressURLs to []byte and assign to baseStatus.IngressURLs
        data, err := json.Marshal(newStatus.IngressURLs)
        if err != nil {
            // Handle error if necessary
        }
        baseStatus.IngressURLs = json.RawMessage(data)
    }
    if newStatus.Credentials != nil {
        if baseStatus.Credentials == nil {
            baseStatus.Credentials = make(json.RawMessage, 0) // Initialize as RawMessage
        }
        // Convert newStatus.Credentials to []byte and assign to baseStatus.Credentials
        data, err := json.Marshal(newStatus.Credentials)
        if err != nil {
            // Handle error if necessary
        }
        baseStatus.Credentials = json.RawMessage(data)
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
	case v1alpha1.SyncRequest:
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

func (c *Controller) updateStatus(observed v1alpha1.SyncRequest, status v1alpha1.ParentResourceStatus) error {
	err := kubernetespkg.UpdateStatus(c.dynClient, observed.Parent.ObjectMeta.Namespace, observed.Parent.ObjectMeta.Name, status)
	if err != nil {
		log.Printf("Error updating status for %s: %v", observed.Parent.ObjectMeta.Name, err)
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

	// updateErr := c.updateStatus(observed, finalStatus)
	// 	if updateErr != nil {
	// 		log.Printf("Error updating status for %s: %v", observed.Parent.ObjectMeta.Name, updateErr)
	// 	}

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
