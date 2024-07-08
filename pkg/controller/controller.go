package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
    
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
    "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/alustan/pkg/registry"
	"github.com/alustan/api/v1alpha1"
	"github.com/alustan/pkg/terraform"
	"github.com/alustan/pkg/util"
    "github.com/alustan/pkg/listers"
    
)

type Controller struct {
	Clientset        kubernetes.Interface
	dynClient        dynamic.Interface
	syncInterval     time.Duration
	lastSyncTime     time.Time
	workqueue        workqueue.RateLimitingInterface
    terraformLister listers.TerraformLister
	informerFactory  informers.SharedInformerFactory // Shared informer factory for Terraform resources
    informer         cache.SharedIndexInformer       // Informer for Terraform resources
}

func NewController(clientset kubernetes.Interface, dynClient dynamic.Interface, syncInterval time.Duration) *Controller {
    // Register the custom resource types with the global scheme
    utilruntime.Must(v1alpha1.AddToScheme(scheme.Scheme))

    ctrl := &Controller{
        Clientset:       clientset,
        dynClient:       dynClient,
        syncInterval:    syncInterval,
        lastSyncTime:    time.Now().Add(-syncInterval), // Initialize to allow immediate first run
        workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "terraforms"),
        informerFactory: informers.NewSharedInformerFactory(clientset, syncInterval),
    }

    // Initialize informer
	ctrl.initInformer()

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

func (c *Controller) initInformer() {
    // Define the GroupVersionResource for the custom resource
    gvr := schema.GroupVersionResource{
        Group:    "alustan.io",
        Version:  "v1alpha1",
        Resource: "terraforms",
    }

    // Get the informer and error returned by ForResource
    informer, err := c.informerFactory.ForResource(gvr)
    if err != nil {
        utilruntime.HandleError(fmt.Errorf("error creating informer for %s: %v", gvr.Resource, err))
        return
    }
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
	// Start the informer
	go c.informer.Run(stopCh)
}


func (c *Controller) handleAddTerraform(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	c.enqueue(key)
}

func (c *Controller) handleUpdateTerraform(old, new interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(new)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", new, err))
		return
	}
	c.enqueue(key)
}

func (c *Controller) handleDeleteTerraform(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	c.enqueue(key)
}

func (c *Controller) enqueue(key string) {
	c.workqueue.AddRateLimited(key)
}

func (c *Controller) RunLeader(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	log.Println("Starting Terraform controller")

	// Setup informers and listers
	c.setupInformer(stopCh)

	// Wait for the informer's cache to sync
	informerSynced := c.informer.HasSynced
	if !cache.WaitForCacheSync(stopCh, informerSynced) {
		utilruntime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

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
		log.Fatalf("Failed to create resource lock: %v", err)
	}

	leaderelection.RunOrDie(context.TODO(), leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				log.Println("Became leader, starting reconciliation loop")
				// Start processing items
				go c.runWorker()
			},
			OnStoppedLeading: func() {
				log.Println("Lost leadership, stopping reconciliation loop")
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
            utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %T", obj))
            return nil
        }

        namespace, name, err := cache.SplitMetaNamespaceKey(key)
        if err != nil {
            c.workqueue.Forget(obj)
            utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
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
            return fmt.Errorf("error fetching resource %s: %v", key, err)
        }

        // Retrieve generation information from status
        generation := terraformObj.GetGeneration()
        observedGeneration := terraformObj.Status.ObservedGeneration

        // Convert generation to int if necessary
        gen := int(generation)

        if gen > observedGeneration {
            // Perform synchronization and update observed generation
            finalStatus, err := c.handleSyncRequest(terraformObj)
            if err != nil {
                finalStatus.State = "Error"
                finalStatus.Message = err.Error()
                c.workqueue.AddRateLimited(key)
                return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
            }

            finalStatus.ObservedGeneration = gen  
            updateErr := c.updateStatus(terraformObj, finalStatus)
            if updateErr != nil {
                log.Printf("Failed to update status for %s: %v", key, updateErr)
                c.workqueue.AddRateLimited(key)
                return updateErr
            }
        }

        c.workqueue.Forget(obj)
        return nil
    }(obj)

    if err != nil {
        utilruntime.HandleError(err)
    }

    return true
}




func (c *Controller) handleSyncRequest(observed *v1alpha1.Terraform) (v1alpha1.ParentResourceStatus, error) {
    envVars := util.ExtractEnvVars(observed.Spec.Variables)
    secretName := fmt.Sprintf("%s-container-secret", observed.ObjectMeta.Name)
    log.Printf("Observed Parent Spec: %+v", observed.Spec)

    commonStatus := v1alpha1.ParentResourceStatus{
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
    scriptContent, scriptContentStatus := terraform.GetScriptContent(observed, finalizing)
    commonStatus = mergeStatuses(commonStatus, scriptContentStatus)
    if scriptContentStatus.State == "Error" {
        return commonStatus, fmt.Errorf("error getting script content")
    }

    // Handle tagged image name
    taggedImageName, taggedImageStatus := registry.GetTaggedImageName(observed, scriptContent, c.Clientset, finalizing)
    commonStatus = mergeStatuses(commonStatus, taggedImageStatus)
    if taggedImageStatus.State == "Error" {
        return commonStatus, fmt.Errorf("error getting tagged image name")
    }

    log.Printf("taggedImageName: %v", taggedImageName)

    // Handle ExecuteTerraform
    execTerraformStatus := terraform.ExecuteTerraform(c.Clientset, observed, scriptContent, taggedImageName, secretName, envVars, finalizing)
    commonStatus = mergeStatuses(commonStatus, execTerraformStatus)

    if execTerraformStatus.State == "Error" {
        return commonStatus, fmt.Errorf("error executing terraform")
    }

    return commonStatus, nil
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

func (c *Controller) updateStatus(parent *v1alpha1.Terraform, finalStatus v1alpha1.ParentResourceStatus) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		namespace := parent.Namespace
		name := parent.Name

		terraformObj, err := c.dynClient.Resource(schema.GroupVersionResource{
			Group:    "alustan.io",
			Version:  "v1alpha1",
			Resource: "terraforms",
		}).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		unstructuredContent := terraformObj.UnstructuredContent()
		statusContent, err := json.Marshal(finalStatus)
		if err != nil {
			return err
		}

		statusMap := make(map[string]interface{})
		if err := json.Unmarshal(statusContent, &statusMap); err != nil {
			return err
		}

		unstructured.SetNestedMap(unstructuredContent, statusMap, "status")

		_, updateErr := c.dynClient.Resource(schema.GroupVersionResource{
			Group:    "alustan.io",
			Version:  "v1alpha1",
			Resource: "terraforms",
		}).Namespace(namespace).UpdateStatus(context.TODO(), terraformObj, metav1.UpdateOptions{})
		return updateErr
	})
}

