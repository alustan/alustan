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
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/client-go/dynamic"
    "k8s.io/client-go/informers"
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
    informer     cache.SharedIndexInformer
    hasSyncedFunc cache.InformerSynced
    workqueue    workqueue.RateLimitingInterface
}

func NewController(clientset kubernetes.Interface, dynClient dynamic.Interface, syncInterval time.Duration, stopCh <-chan struct{}) *Controller {
    // Define the GVR for the CRD
    gvr := schema.GroupVersionResource{
        Group:    "alustan.io",
        Version:  "v1alpha1",
        Resource: "terraforms",
    }

    // Create the informer for the CRD
    factory := informers.NewSharedInformerFactoryWithOptions(clientset, syncInterval, informers.WithTweakListOptions(func(options *metav1.ListOptions) {}))
    genericInformer, err := factory.ForResource(gvr)
    if err != nil {
        log.Fatalf("Error creating informer for resource: %v", err)
    }

    informer := genericInformer.Informer()

    controller := &Controller{
        clientset:    clientset,
        dynClient:    dynClient,
        syncInterval: syncInterval,
        lastSyncTime: time.Now().Add(-syncInterval), // Initialize to allow immediate first run
        Cache:        make(map[string]string),       // Initialize cache
        informer:     informer,
        hasSyncedFunc: informer.HasSynced,
        workqueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "terraforms"),
    }

    // Setup event handlers
    informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc: controller.enqueue,
        UpdateFunc: func(old, new interface{}) {
            controller.enqueue(new)
        },
        DeleteFunc: controller.enqueue,
    })

    // Wait for cache to sync
    go factory.Start(stopCh)
    if ok := cache.WaitForCacheSync(stopCh, controller.hasSyncedFunc); !ok {
        log.Fatalf("Failed to wait for caches to sync")
    }

    return controller
}

func NewInClusterController(syncInterval time.Duration, stopCh <-chan struct{}) *Controller {
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

    return NewController(clientset, dynClient, syncInterval, stopCh)
}

func (c *Controller) HasSynced() bool {
    return c.hasSyncedFunc()
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

    // Trigger immediate sync on new CRD or CRD update
    if c.IsCRDChanged(observed) {
        response := c.handleSyncRequest(observed)
        c.UpdateCache(observed)
        r.Writer.Header().Set("Content-Type", "application/json")
        r.JSON(http.StatusOK, gin.H{"body": response})
    } else {
        r.String(http.StatusOK, "No changes detected, no action taken")
    }
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

func (c *Controller) Reconcile(stopCh <-chan struct{}) {
    ticker := time.NewTicker(c.syncInterval)
    for {
        select {
        case <-ticker.C:
            if c.HasSynced() {
                c.reconcileLoop()
                c.lastSyncTime = time.Now()
            }
        case <-stopCh:
            ticker.Stop()
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

	var wg sync.WaitGroup
	for _, item := range resourceList.Items {
		wg.Add(1)
		go func(item unstructured.Unstructured) {
			defer wg.Done()
			var observed schematypes.SyncRequest
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

func (c *Controller) enqueue(obj interface{}) {
    key, err := cache.MetaNamespaceKeyFunc(obj)
    if err != nil {
        log.Printf("Error creating key for object: %v", err)
        return
    }
    c.workqueue.Add(key)
}
