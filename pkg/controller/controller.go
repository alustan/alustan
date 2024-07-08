package controller

import (
	"context"
	
	"fmt"
	
	"time"

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

	"github.com/alustan/pkg/registry"
	"github.com/alustan/api/v1alpha1"
	"github.com/alustan/pkg/terraform"
	"github.com/alustan/pkg/util"
	"github.com/alustan/pkg/listers"
	Kubernetespkg "github.com/alustan/pkg/kubernetes"
)

type Controller struct {
	Clientset        kubernetes.Interface
	dynClient        dynamic.Interface
	syncInterval     time.Duration
	lastSyncTime     time.Time
	workqueue        workqueue.RateLimitingInterface
	terraformLister  listers.TerraformLister
	informerFactory  dynamicinformer.DynamicSharedInformerFactory // Shared informer factory for Terraform resources
	informer         cache.SharedIndexInformer                    // Informer for Terraform resources
	logger           *zap.SugaredLogger
}


func NewController(clientset kubernetes.Interface, dynClient dynamic.Interface, syncInterval time.Duration) *Controller {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sugar := logger.Sugar()
	
	ctrl := &Controller{
		Clientset:       clientset,
		dynClient:       dynClient,
		syncInterval:    syncInterval,
		lastSyncTime:    time.Now().Add(-syncInterval), // Initialize to allow immediate first run
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "terraforms"),
		informerFactory: dynamicinformer.NewDynamicSharedInformerFactory(dynClient, syncInterval),
		logger:          sugar,
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
		Resource: "terraforms",
	}

	// Get the informer and error returned by ForResource
	informer := c.informerFactory.ForResource(gvr)
	c.informer = informer.Informer()

	// Set the lister for the custom resource
	c.terraformLister = listers.NewTerraformLister(c.informer.GetIndexer())

	// Add event handlers to the informer
	c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handleAddTerraform,
		UpdateFunc: c.handleUpdateTerraform,
		DeleteFunc: c.handleDeleteTerraform,
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

func (c *Controller) handleAddTerraform(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.logger.Errorf("couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.enqueue(key)
}

func (c *Controller) handleUpdateTerraform(old, new interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(new)
	if err != nil {
		c.logger.Errorf("couldn't get key for object %+v: %v", new, err)
		return
	}
	c.enqueue(key)
}

func (c *Controller) handleDeleteTerraform(obj interface{}) {
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

	c.logger.Info("Starting Terraform controller")

	// Setup informers and listers
	c.setupInformer(stopCh)

	// Leader election configuration
	id := util.GetUniqueID()
	rl, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		"alustan",
		"terraform-controller-lock",
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
				go c.runWorker()
			},
			OnStoppedLeading: func() {
				c.logger.Info("Lost leadership, stopping reconciliation loop")
				// Stop processing items
				c.workqueue.ShutDown()
			},
		},
		ReleaseOnCancel: true,
	})
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
		terraformObj, err := c.terraformLister.Terraform(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				c.workqueue.Forget(obj)
				return nil
			}

			c.workqueue.AddRateLimited(key)
			c.logger.Errorf("error fetching resource %s: %v", key, err)
			return fmt.Errorf("error fetching resource %s: %v", key, err)
		}

		// Convert to *v1alpha1.Terraform
		unstructuredObj, ok := terraformObj.(*unstructured.Unstructured)
		if !ok {
			c.workqueue.Forget(obj)
			c.logger.Errorf("expected *unstructured.Unstructured but got %T", terraformObj)
			return nil
		}
		terraform := &v1alpha1.Terraform{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, terraform)
		if err != nil {
			c.workqueue.Forget(obj)
			c.logger.Errorf("error converting unstructured object to *v1alpha1.Terraform: %v", err)
			return nil
		}

		// Retrieve generation information from status
		generation := terraform.GetGeneration()
		observedGeneration := terraform.Status.ObservedGeneration

		// Convert generation to int if necessary
		gen := int(generation)

		if gen > observedGeneration {
			// Perform synchronization and update observed generation
			finalStatus, err := c.handleSyncRequest(terraform)
			if err != nil {
				finalStatus.State = "Error"
				finalStatus.Message = err.Error()
				c.workqueue.AddRateLimited(key)
				c.logger.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
				return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
			}

			finalStatus.ObservedGeneration = gen
			updateErr := c.updateStatus(terraform, finalStatus)
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

func (c *Controller) handleSyncRequest(observed *v1alpha1.Terraform) (v1alpha1.TerraformStatus, error) {
    envVars := util.ExtractEnvVars(observed.Spec.Variables)
    secretName := fmt.Sprintf("%s-container-secret", observed.ObjectMeta.Name)
    c.logger.Infof("Observed Parent Spec: %+v", observed.Spec)

    commonStatus := v1alpha1.TerraformStatus{
        State:   "Progressing",
        Message: "Starting processing",
    }

    finalizing := false
    // Check if the resource is being deleted
    if observed.ObjectMeta.DeletionTimestamp != nil {
        finalizing = true

        // Add finalizer if not already present
        finalizerName := "terraform.finalizer.alustan.io"
        if !util.ContainsString(observed.ObjectMeta.Finalizers, finalizerName) {
            observed.ObjectMeta.Finalizers = append(observed.ObjectMeta.Finalizers, finalizerName)
            _, err := c.Clientset.CoreV1().RESTClient().
                Put().
                Namespace(observed.Namespace).
                Resource("terraforms").
                Name(observed.Name).
                Body(observed).
                Do(context.TODO()).
                Get()
            if err != nil {
                return commonStatus, fmt.Errorf("error adding finalizer: %v", err)
            }
        }
    }

    // Handle script content
    scriptContent, scriptContentStatus := terraform.GetScriptContent(c.logger,observed, finalizing)
    commonStatus = mergeStatuses(commonStatus, scriptContentStatus)
    if scriptContentStatus.State == "Error" {
        return commonStatus, fmt.Errorf("error getting script content")
    }

    // Handle tagged image name
    taggedImageName, taggedImageStatus := registry.GetTaggedImageName(c.logger,observed, scriptContent, c.Clientset, finalizing)
    commonStatus = mergeStatuses(commonStatus, taggedImageStatus)
    if taggedImageStatus.State == "Error" {
        return commonStatus, fmt.Errorf("error getting tagged image name")
    }

    c.logger.Infof("taggedImageName: %v", taggedImageName)

    // Handle ExecuteTerraform
    execTerraformStatus := terraform.ExecuteTerraform(c.logger,c.Clientset, observed, scriptContent, taggedImageName, secretName, envVars, finalizing)
    commonStatus = mergeStatuses(commonStatus, execTerraformStatus)

    if execTerraformStatus.State == "Error" {
        return commonStatus, fmt.Errorf("error executing terraform")
    }

    return commonStatus, nil
}

func mergeStatuses(baseStatus, newStatus v1alpha1.TerraformStatus) v1alpha1.TerraformStatus {
    if newStatus.State != "" {
        baseStatus.State = newStatus.State
    }
    if newStatus.Message != "" {
        baseStatus.Message = newStatus.Message
    }
    if newStatus.Output != nil {
        baseStatus.Output = newStatus.Output
    }
    if newStatus.PostDeployOutput != nil {
        baseStatus.PostDeployOutput = newStatus.PostDeployOutput
    }
    if newStatus.IngressURLs != nil {
        baseStatus.IngressURLs = newStatus.IngressURLs
    }
    if newStatus.Credentials != nil {
        baseStatus.Credentials = newStatus.Credentials
    }
   
    return baseStatus
}


func (c *Controller) updateStatus(observed *v1alpha1.Terraform, status v1alpha1.TerraformStatus) error {
	err := Kubernetespkg.UpdateStatus(c.logger, c.dynClient, observed.ObjectMeta.Namespace, observed.ObjectMeta.Name, status)
	if err != nil {
		c.logger.Errorf("Failed to update status for %s/%s: %v", observed.ObjectMeta.Namespace, observed.ObjectMeta.Name, err)
		return err
	}
	return nil
}





