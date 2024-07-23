package controller

import (
	"context"
	"sync"
	"fmt"
	"strings"
	"time"
	"os"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"  
	

	"k8s.io/client-go/util/workqueue"
	"k8s.io/client-go/dynamic/dynamicinformer"
	apiclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	appv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	

	"github.com/alustan/alustan/pkg/application/registry"
	"github.com/alustan/alustan/api/service/v1alpha1"
	"github.com/alustan/alustan/pkg/application/service"
	"github.com/alustan/alustan/pkg/util"
	"github.com/alustan/alustan/pkg/application/listers"
	Kubernetespkg "github.com/alustan/alustan/pkg/application/kubernetes"
	"github.com/alustan/alustan/pkg/installargocd"
)

type Controller struct {
	Clientset        kubernetes.Interface
	dynClient        dynamic.Interface
	syncInterval     time.Duration
	lastSyncTime     time.Time
	workqueue        workqueue.RateLimitingInterface
	serviceLister    listers.ServiceLister
	informerFactory  dynamicinformer.DynamicSharedInformerFactory // Shared informer factory for Service resources
	informer         cache.SharedIndexInformer                    // Informer for Service resources
	logger           *zap.SugaredLogger
	mu               sync.Mutex
	numWorkers       int
	maxWorkers       int
	workerStopCh  chan struct{}
    managerStopCh chan struct{}
	argoClient   apiclient.Client
	
}

// Ensure the setup function is called only once
var setupOnce sync.Once

func setupArgoCD(sugar *zap.SugaredLogger, clientset kubernetes.Interface, dynClient dynamic.Interface) {
	setupOnce.Do(func() {
		argoerr := installargocd.InstallArgoCD(sugar, clientset, dynClient, "6.6.0")
		if argoerr != nil {
			sugar.Fatal(argoerr.Error())
		}
	})
}


// NewController initializes a new controller
func NewController(clientset kubernetes.Interface, dynClient dynamic.Interface, syncInterval time.Duration) *Controller {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sugar := logger.Sugar()

	setupArgoCD(sugar, clientset, dynClient)

	argoURL := "http://argocd-server.argocd.svc.cluster.local"

	// Initialize ArgoCD client
	argoClientOpts := apiclient.ClientOptions{
		ServerAddr: argoURL,
		PlainText:  true,
	}
	argoClient, err := apiclient.NewClient(&argoClientOpts)
	if err != nil {
		sugar.Fatal(err.Error())
	}

	ctrl := &Controller{
		Clientset:       clientset,
		dynClient:       dynClient,
		syncInterval:    syncInterval,
		lastSyncTime:    time.Now().Add(-syncInterval), // Initialize to allow immediate first run
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "services"),
		informerFactory: dynamicinformer.NewDynamicSharedInformerFactory(dynClient, syncInterval),
		logger:          sugar,
		numWorkers:      0,
		maxWorkers:      5,
		workerStopCh:    make(chan struct{}),
		managerStopCh:   make(chan struct{}),
		argoClient:      argoClient,
	}

	// Initialize informer
	ctrl.initInformer()

	return ctrl
}


func NewInClusterController(syncInterval time.Duration) *Controller {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sugar := logger.Sugar()

	config, err := rest.InClusterConfig()
	if err != nil {
		sugar.Fatalf("Error creating in-cluster config: %v", err)
	}

	config.QPS = 100.0
    config.Burst = 200

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		sugar.Fatalf("Error creating Kubernetes clientset: %v", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		sugar.Fatalf("Error creating dynamic Kubernetes client: %v", err)
	}

	return NewController(clientset, dynClient, syncInterval)
}

func (c *Controller) initInformer() {
	// Define the GroupVersionResource for the custom resource
	gvr := schema.GroupVersionResource{
		Group:    "alustan.io",
		Version:  "v1alpha1",
		Resource: "services",
	}

	// Get the informer and error returned by ForResource
	informer := c.informerFactory.ForResource(gvr)
	c.informer = informer.Informer()

	// Set the lister for the custom resource
	c.serviceLister = listers.NewServiceLister(c.informer.GetIndexer())

	// Add event handlers to the informer
	c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handleAddService,
		UpdateFunc: c.handleUpdateService,
		DeleteFunc: c.handleDeleteService,
	})
}


func (c *Controller) setupInformer(stopCh <-chan struct{}) {
	if c.informer == nil {
		c.logger.Fatal("informer is nil, ensure initInformer is called before setupInformer")
	}

	// Start the informer
	go c.informer.Run(stopCh)

	// Wait for the informer's cache to sync
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		c.logger.Error("timed out waiting for caches to sync")
		return
	}
}

func (c *Controller) handleAddService(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.logger.Errorf("couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.enqueue(key)
}

func (c *Controller) handleUpdateService(old, new interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(new)
	if err != nil {
		c.logger.Errorf("couldn't get key for object %+v: %v", new, err)
		return
	}
	c.enqueue(key)
}

func (c *Controller) handleDeleteService(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		c.logger.Errorf("couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.enqueue(key)
}

func (c *Controller) enqueue(key string) {
	c.workqueue.AddRateLimited(key)
}



func (c *Controller) RunLeader(stopCh <-chan struct{}) {
	defer c.logger.Sync()

	c.logger.Info("Starting Service controller")

	// Setup informers and listers
	c.setupInformer(stopCh)

	// Leader election configuration
	id := util.GetUniqueID()
	rl, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		"alustan",
		"service-controller-lock",
		c.Clientset.CoreV1(),
		c.Clientset.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: id,
		},
		
	)
	if err != nil {
		c.logger.Fatalf("Failed to create resource lock: %v", err)
	}

	leaderelection.RunOrDie(context.TODO(), leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				c.logger.Info("Became leader, starting reconciliation loop")
				// Start processing items
				go c.manageWorkers()
			},
			OnStoppedLeading: func() {
				c.logger.Info("Lost leadership, stopping reconciliation loop")
				// Stop processing items
				close(c.workerStopCh)  // Stop all individual runWorker functions
				close(c.managerStopCh) // Stop the manageWorkers function
                c.workqueue.ShutDown()
			},
		},
		ReleaseOnCancel: true,
	})
}


func (c *Controller) manageWorkers() {
	for {
		select {
		case <-c.managerStopCh:
		  return // Stops the manageWorkers loop
        default:
			queueLength := c.workqueue.Len()

			c.mu.Lock()
			currentWorkers := c.numWorkers
			c.mu.Unlock()

			// Calculate the desired number of workers based on the queue length
			// spawn a new runworker routine for every queue length greater than 50 vice versa
			desiredWorkers := (queueLength / 50) + 1
			if desiredWorkers > c.maxWorkers {
				desiredWorkers = c.maxWorkers
			}

			// Ensure at least one worker is running
			if desiredWorkers < 1 {
				desiredWorkers = 1
			}

			// Increase workers if needed
			if currentWorkers < desiredWorkers {
				for i := currentWorkers; i < desiredWorkers; i++ {
					go c.runWorker()
					c.mu.Lock()
					c.numWorkers++
					c.mu.Unlock()
				}
			}

			// Decrease workers if needed, but ensure at least one worker is running
			if currentWorkers > desiredWorkers {
				for i := currentWorkers; i > desiredWorkers && i > 1; i-- {
					c.mu.Lock()
					c.numWorkers--
					c.mu.Unlock()
					c.workerStopCh <- struct{}{} // Signal a worker to stop

				}
			}

			time.Sleep(15 * time.Second)
		}
	}
}


func (c *Controller) runWorker() {
    for {
        select {
        case <-c.workerStopCh:
            return // Stops the individual worker
        default:
            if !c.processNextWorkItem() {
                return
            }
        }
    }
}



func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		key, ok := obj.(string)
		if !ok {
			c.workqueue.Forget(obj)
			c.logger.Errorf("expected string in workqueue but got %T", obj)
			return nil
		}

		namespace, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			c.workqueue.Forget(obj)
			c.logger.Errorf("invalid resource key: %s", key)
			return nil
		}

		// Get the actual resource using the lister
		serviceObject, err := c.serviceLister.Service(namespace).Get(name)
		if err != nil {
		    // Check if the error message contains "not found"
			if strings.Contains(err.Error(), "not found") {
				c.workqueue.Forget(obj)
				c.logger.Infof("resource %s/%s no longer exists", namespace, name)
				return nil
			}

			// For other errors, decide whether to requeue or not
			if errors.IsInternalError(err) || errors.IsServerTimeout(err) {
				// These are considered transient errors, requeue the item
				c.workqueue.AddRateLimited(key)
				c.logger.Errorf("transient error fetching resource. requeing!! %s: %v", key, err)
				return fmt.Errorf("transient error fetching resource. requeing!! %s: %v", key, err)
			} else {
				// Non-transient errors, do not requeue the item
				c.workqueue.Forget(obj)
				c.logger.Errorf("non-transient error fetching resource %s: %v", key, err)
				return fmt.Errorf("non-transient error fetching resource %s: %v", key, err)
			}
		}

		serviceObj := serviceObject.DeepCopyObject()

		// Convert to *v1alpha1.Service
		unstructuredObj, ok := serviceObj.(*unstructured.Unstructured)
		if !ok {
			c.workqueue.Forget(obj)
			c.logger.Errorf("expected *unstructured.Unstructured but got %T", serviceObj)
			return nil
		}
		service := &v1alpha1.Service{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, service)
		if err != nil {
			c.workqueue.Forget(obj)
			c.logger.Errorf("error converting unstructured object to *v1alpha1.Service: %v", err)
			return nil
		}

		// Retrieve generation information from status
		generation := service.GetGeneration()
		observedGeneration := service.Status.ObservedGeneration

		// Convert generation to int if necessary
		gen := int(generation)

		if gen > observedGeneration {
			// Perform synchronization and update observed generation
			finalStatus, err := c.handleSyncRequest(c.argoClient,service)
			if finalStatus.Message == "Destroy completed successfully" {
               return nil
			}
			if err != nil {
				finalStatus.State = "Error"
				finalStatus.Message = err.Error()
				c.workqueue.AddRateLimited(key)
				c.logger.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
				return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
			}

			finalStatus.ObservedGeneration = gen
			updateErr := c.updateStatus(service, finalStatus)
			if updateErr != nil {
				c.logger.Infof("Failed to update status for %s: %v", key, updateErr)
				c.workqueue.AddRateLimited(key)
				return updateErr
			}
		}

		c.workqueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		c.logger.Error(err)
		return true
	}

	return true
}

func (c *Controller) handleSyncRequest(argoClient apiclient.Client,observed *v1alpha1.Service) (v1alpha1.ServiceStatus, error) {
     
	 secretName := fmt.Sprintf("%s-container-secret", observed.ObjectMeta.Name)
	 key := "pat"
	 gitHubPATBase64 := os.Getenv("GITHUB_TOKEN")

	 c.logger.Infof("Observed Parent Spec: %+v", observed.Spec)
	 
 
	 commonStatus := v1alpha1.ServiceStatus{
		 State:   "Progressing",
		 Message: "Starting processing",
	 }
	 // Add finalizer if not already present
	 err := Kubernetespkg.AddFinalizer(c.logger, c.dynClient, observed.ObjectMeta.Name, observed.ObjectMeta.Namespace)
	 if err != nil {
		 c.logger.Errorf("Failed to add finalizer for %s/%s: %v", observed.ObjectMeta.Namespace, observed.ObjectMeta.Name, err)
		 commonStatus.State = "Error"
		 commonStatus.Message = fmt.Sprintf("Failed to add finalizer: %v", err)
		 return commonStatus, err
	 }
	  finalizing := false
	 // Check if the resource is being deleted
	 if observed.ObjectMeta.DeletionTimestamp != nil {
		 
		 finalizing = true
	 }

	 taggedImageName,registryStatus :=  registry.HandleContainerRegistry(c.logger,c.Clientset,observed)
	  commonStatus = mergeStatuses(commonStatus, registryStatus)
	  if registryStatus.State == "Error" {
        return commonStatus, fmt.Errorf("error getting tagged image name")
    }

     c.logger.Infof("taggedImageName: %v", taggedImageName)

	 err = Kubernetespkg.CreateOrUpdateSecretWithGitHubPAT(c.logger , c.Clientset, observed.ObjectMeta.Namespace, secretName, key, gitHubPATBase64 )
     
	 if err != nil {
		return commonStatus, fmt.Errorf("Failed to create/update secret: %v", err)
	}
	 // Handle RunService
	 runServiceStatus := service.RunService(c.logger,c.Clientset, c.dynClient, argoClient, observed, secretName, key, finalizing)
	 commonStatus = mergeStatuses(commonStatus, runServiceStatus)
 
	 if runServiceStatus.State == "Error" {
		 return commonStatus, fmt.Errorf("error running service")
	 }
 
	 return commonStatus, nil
 }

// Define the helper function to check if HealthStatus is empty
func isEmptyApplicationSetStatus(status appv1alpha1.ApplicationSetStatus) bool {
    return len(status.Conditions) == 0
}
 
func mergeStatuses(baseStatus, newStatus v1alpha1.ServiceStatus) v1alpha1.ServiceStatus {
    if newStatus.State != "" {
        baseStatus.State = newStatus.State
    }
    if newStatus.Message != "" {
        baseStatus.Message = newStatus.Message
    }
    
    if !isEmptyApplicationSetStatus(newStatus.HealthStatus) {
        baseStatus.HealthStatus = newStatus.HealthStatus
    }

    if newStatus.PreviewURLs != nil {
        baseStatus.PreviewURLs = newStatus.PreviewURLs
    }
   
    return baseStatus
}


func (c *Controller) updateStatus(observed *v1alpha1.Service, status v1alpha1.ServiceStatus) error {
	err := Kubernetespkg.UpdateStatus(c.logger, c.dynClient, observed.ObjectMeta.Name, observed.ObjectMeta.Namespace, status)
	
	if err != nil {
		c.logger.Errorf("Failed to update status for %s/%s: %v", observed.ObjectMeta.Namespace, observed.ObjectMeta.Name, err)
		return err
	}
	return nil

}