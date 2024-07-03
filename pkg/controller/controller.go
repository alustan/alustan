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

	"github.com/alustan/pkg/schematypes"
	"github.com/alustan/pkg/terraform"
	"github.com/alustan/pkg/registry"
	"github.com/alustan/pkg/util"
	kubernetespkg "github.com/alustan/pkg/kubernetes"
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
		r.String(http.StatusBadRequest, err.Error())
		return
	}
	defer func() {
		if err := r.Request.Body.Close(); err != nil {
			log.Printf("Error closing request body: %v", err)
		}
	}()

	key := fmt.Sprintf("%s/%s", observed.Parent.Metadata.Namespace, observed.Parent.Metadata.Name)

	// Store the observed SyncRequest in the map
	c.mapMutex.Lock()
	c.observedMap[key] = observed
	c.mapMutex.Unlock()

	// Enqueue the request for processing
	c.enqueue(key)
	r.String(http.StatusOK, "Request enqueued for processing")
}

func (c *Controller) handleSyncRequest(observed schematypes.SyncRequest) map[string]interface{} {
	envVars := util.ExtractEnvVars(observed.Parent.Spec.Variables)
	secretName := fmt.Sprintf("%s-container-secret", observed.Parent.Metadata.Name)
	log.Printf("Observed Parent Spec: %+v", observed.Parent.Spec)

	initialStatus := map[string]interface{}{
		"state":   "Progressing",
		"message": "Starting processing",
	}
	err := c.updateStatus(observed, initialStatus)
	if err != nil {
		log.Printf("Error updating initial status: %v", err)
		return initialStatus
	}

	scriptContent, status := terraform.GetScriptContent(observed, c.updateStatus)
	if status != nil {
		return status
	}

	taggedImageName, status := registry.GetTaggedImageName(observed, scriptContent, c.clientset, c.updateStatus)
	if status != nil {
		return status
	}

	return terraform.ExecuteTerraform(c.clientset, observed, scriptContent, taggedImageName, secretName, envVars, c.updateStatus)
}

func (c *Controller) updateStatus(observed schematypes.SyncRequest, status map[string]interface{}) error {
	err := kubernetespkg.UpdateStatus(c.dynClient, observed.Parent.Metadata.Namespace, observed.Parent.Metadata.Name, status)
	if err != nil {
		log.Printf("Error updating status for %s: %v", observed.Parent.Metadata.Name, err)
	}
	return err
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

func (c *Controller) enqueue(obj interface{}) {
	var key string
	var err error

	switch o := obj.(type) {
	case schematypes.SyncRequest:
		wrapped := schematypes.SyncRequestWrapper{o}
		key, err = cache.MetaNamespaceKeyFunc(&wrapped)
	default:
		key, err = cache.MetaNamespaceKeyFunc(obj)
	}

	if err != nil {
		log.Printf("Error creating key for object: %v", err)
		return
	}
	c.workqueue.Add(key)
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

	// Here, instead of fetching the resource, we assume it was provided directly
	var observed schematypes.SyncRequest
	observed = c.getObservedFromKey(key.(string))
	if !c.isObservedSyncRequestEmpty(observed) {
		log.Printf("Error fetching resource for key %s", key)
		c.workqueue.AddRateLimited(key)
		return true
	}

	if c.IsCRDChanged(observed) {
		response := c.handleSyncRequest(observed)
		c.UpdateCache(observed)
		log.Printf("Processed resource: %+v", response)
	}

	c.workqueue.Forget(key)
	return true
}

func (c *Controller) isObservedSyncRequestEmpty(observed schematypes.SyncRequest) bool {
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
		c.enqueue(item)
	}
}
