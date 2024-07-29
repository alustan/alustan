package controller

import (
	"context"
	"sync"
	"fmt"
	"strings"
	"time"
	"os"
	"bytes"
    "crypto/tls"
    "encoding/json"
	"encoding/base64"
    "net/http"
	"io"

	
	
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	
	"k8s.io/client-go/util/workqueue"
	"k8s.io/client-go/dynamic/dynamicinformer"
	apiclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	appv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	
	applicationsetpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/applicationset"
	clusterpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/cluster"

	"github.com/alustan/alustan/pkg/application/registry"
	"github.com/alustan/alustan/api/app/v1alpha1"
	"github.com/alustan/alustan/pkg/application/service"
	"github.com/alustan/alustan/pkg/util"
	"github.com/alustan/alustan/pkg/application/listers"
	Kubernetespkg "github.com/alustan/alustan/pkg/application/kubernetes"
	"github.com/alustan/alustan/pkg/installargocd"
	
)

var (
    token     string
    tokenLock sync.Mutex
)


type Controller struct {
	Clientset        kubernetes.Interface
	dynClient        dynamic.Interface
	syncInterval     time.Duration
	lastSyncTime     time.Time
	workqueue        workqueue.RateLimitingInterface
	appLister    listers.AppLister
	informerFactory  dynamicinformer.DynamicSharedInformerFactory // Shared informer factory for App resources
	informer         cache.SharedIndexInformer                    // Informer for App resources
	logger           *zap.SugaredLogger
	mu               sync.Mutex
	numWorkers       int
	maxWorkers       int
	workerStopCh  chan struct{}
    managerStopCh chan struct{}
	argoClient   apiclient.Client
	appSetClient   applicationsetpkg.ApplicationSetServiceClient
	clusterClient  clusterpkg.ClusterServiceClient
	
}


// NewController initializes a new controller
func NewController(clientset kubernetes.Interface, dynClient dynamic.Interface, syncInterval time.Duration, logger *zap.SugaredLogger) *Controller {
	argoerr := installargocd.InstallArgoCD(logger, clientset, dynClient, "6.6.0")
	if argoerr != nil {
		logger.Fatal(argoerr.Error())
	}

    ctrl := &Controller{
		Clientset:       clientset,
		dynClient:       dynClient,
		syncInterval:    syncInterval,
		lastSyncTime:    time.Now().Add(-syncInterval), // Initialize to allow immediate first run
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "apps"),
		informerFactory: dynamicinformer.NewDynamicSharedInformerFactory(dynClient, syncInterval),
		logger:          logger,
		numWorkers:      0,
		maxWorkers:      5,
		workerStopCh:    make(chan struct{}),
		managerStopCh:   make(chan struct{}),
		
	}

	// Initialize informer
	ctrl.initInformer()

	return ctrl
}


func NewInClusterController(syncInterval time.Duration, logger *zap.SugaredLogger) *Controller {
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatalf("Error creating in-cluster config: %v", err)
	}

	config.QPS = 100.0
	config.Burst = 200

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatalf("Error creating Kubernetes clientset: %v", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		logger.Fatalf("Error creating dynamic Kubernetes client: %v", err)
	}

	return NewController(clientset, dynClient, syncInterval, logger)
}

func (c *Controller) initInformer() {
	// Define the GroupVersionResource for the custom resource
	gvr := schema.GroupVersionResource{
		Group:    "alustan.io",
		Version:  "v1alpha1",
		Resource: "apps",
	}

	// Get the informer and error returned by ForResource
	informer := c.informerFactory.ForResource(gvr)
	c.informer = informer.Informer()

	// Set the lister for the custom resource
	c.appLister = listers.NewAppLister(c.informer.GetIndexer())

	// Add event handlers to the informer
	c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handleAddApp,
		UpdateFunc: c.handleUpdateApp,
		DeleteFunc: c.handleDeleteApp,
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

func (c *Controller) handleAddApp(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.logger.Errorf("couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.enqueue(key)
}

func (c *Controller) handleUpdateApp(old, new interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(new)
	if err != nil {
		c.logger.Errorf("couldn't get key for object %+v: %v", new, err)
		return
	}
	c.enqueue(key)
}

func (c *Controller) handleDeleteApp(obj interface{}) {
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

	c.logger.Info("Starting App controller")

	// Setup informers and listers
	c.setupInformer(stopCh)

	// Leader election configuration
	id := util.GetUniqueID()
	rl, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		"alustan",
		"app-controller-lock",
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
		LeaseDuration: 30 * time.Second,
        RenewDeadline: 20 * time.Second,
        RetryPeriod:   5 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				c.logger.Info("got leadership")

				

			 // Authenticate and create ArgoCD client
			 password, err := getAdminPassword(c.Clientset)
			 if err != nil {
				 c.logger.Fatalf("Failed to get admin password: %v", err)
			 }

			 authToken, err := GetAuthToken(c, password)
			 if err != nil {
				 c.logger.Fatalf("Failed to get auth token: %v", err)
			 }

			 err = refreshClients(c, authToken)
			 if err != nil {
				 c.logger.Fatalf("Failed to refresh clients: %v", err)
			 }
		
			 c.logger.Infof("Successfully created ArgoCD client: %v\n", c.argoClient)
             c.logger.Infof("Successfully created ApplicationSet client: %v\n", c.appSetClient)
			 c.logger.Infof("Successfully created Cluster client: %v\n", c.clusterClient)

				// Start processing items
				go c.manageWorkers()
			},
			OnStoppedLeading: func() {
				c.logger.Infof("Pod %s lost leadership", id)
				// Stop processing items
				close(c.workerStopCh)  // Stop all individual runWorker functions
				close(c.managerStopCh) // Stop the manageWorkers function
				c.workqueue.ShutDown()
			},
			OnNewLeader: func(identity string) {
				if identity == id {
					c.logger.Infof("Pod %s is still the leader", id)
				} else {
					c.logger.Infof("New leader elected: %s", identity)
				}
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
		appObject, err := c.appLister.App(namespace).Get(name)
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

		appObj := appObject.DeepCopyObject()

		// Convert to *v1alpha1.App
		unstructuredObj, ok := appObj.(*unstructured.Unstructured)
		if !ok {
			c.workqueue.Forget(obj)
			c.logger.Errorf("expected *unstructured.Unstructured but got %T", appObj)
			return nil
		}
		app := &v1alpha1.App{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, app)
		if err != nil {
			c.workqueue.Forget(obj)
			c.logger.Errorf("error converting unstructured object to *v1alpha1.App: %v", err)
			return nil
		}

		// Retrieve generation information from status
		generation := app.GetGeneration()
		observedGeneration := app.Status.ObservedGeneration

		// Convert generation to int if necessary
		gen := int(generation)

		if gen > observedGeneration {
			// Perform synchronization and update observed generation
			finalStatus, err := c.handleSyncRequest(c.appSetClient,app)
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
			updateErr := c.updateStatus(app, finalStatus)
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

func (c *Controller) handleSyncRequest(appSetClient applicationsetpkg.ApplicationSetServiceClient, observed *v1alpha1.App) (v1alpha1.AppStatus, error) {
    secretName := fmt.Sprintf("%s-container-secret", observed.ObjectMeta.Name)
    key := "pat"
    gitHubPATBase64 := os.Getenv("GITHUB_TOKEN")

  

    commonStatus := v1alpha1.AppStatus{
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

    taggedImageName, registryStatus := registry.HandleContainerRegistry(c.logger, c.Clientset, observed)
    commonStatus = mergeStatuses(commonStatus, registryStatus)
    if registryStatus.State == "Error" {
        return commonStatus, fmt.Errorf("error getting tagged image name")
    }

    c.logger.Infof("taggedImageName: %v", taggedImageName)

    err = Kubernetespkg.CreateOrUpdateSecretWithGitHubPAT(c.logger, c.Clientset, observed.ObjectMeta.Namespace, secretName, key, gitHubPATBase64)
    if err != nil {
        return commonStatus, fmt.Errorf("Failed to create/update secret: %v", err)
    }

   // Handle RunService and process its status and error
    runServiceStatus, runServiceErr := service.RunService(c.logger, c.Clientset, c.clusterClient, c.dynClient, appSetClient, observed, secretName, key, finalizing)
	
	commonStatus = mergeStatuses(commonStatus, runServiceStatus)
	
	if runServiceErr != nil {
        return commonStatus, fmt.Errorf("error running service: %v", runServiceErr)
    }

   return commonStatus, nil
}


// Define the helper function to check if HealthStatus is empty
func isEmptyApplicationSetStatus(status appv1alpha1.ApplicationSetStatus) bool {
    return len(status.Conditions) == 0
}
 
func mergeStatuses(baseStatus, newStatus v1alpha1.AppStatus) v1alpha1.AppStatus {
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


func (c *Controller) updateStatus(observed *v1alpha1.App, status v1alpha1.AppStatus) error {
	err := Kubernetespkg.UpdateStatus(c.logger, c.dynClient, observed.ObjectMeta.Name, observed.ObjectMeta.Namespace, status)
	
	if err != nil {
		c.logger.Errorf("Failed to update status for %s/%s: %v", observed.ObjectMeta.Namespace, observed.ObjectMeta.Name, err)
		return err
	}
	return nil

}

// Retrieve the base64-encoded admin password from the Kubernetes secret and decode it

func getAdminPassword(clientset kubernetes.Interface) (string, error) {
    namespace := "argocd"
    secretName := "argocd-initial-admin-secret"

    secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
    if err != nil {
        return "", fmt.Errorf("failed to get secret: %v", err)
    }

    passwordBytes, exists := secret.Data["password"]
    if !exists {
        return "", fmt.Errorf("password not found in secret")
    }

    rawBase64Password := string(passwordBytes)
    fmt.Printf("Raw base64 password: %s", rawBase64Password)

    // Trim spaces and check for extraneous characters
    trimmedPassword := strings.TrimSpace(rawBase64Password)
    fmt.Printf("Trimmed base64 password: %s", trimmedPassword)

    decodedPassword, err := base64.StdEncoding.DecodeString(trimmedPassword)
    if err != nil {
        return "", fmt.Errorf("failed to decode password: %v", err)
    }

    return string(decodedPassword), nil
}


// GenerateAuthToken fetches an authentication token from Argo CD
func GenerateAuthToken(password string) (string, error) {
	// Define the Argo CD login URL and request payload
	argoURL := "https://argo-cd-argocd-server.argocd.svc.cluster.local/api/v1/session"
	payload := map[string]string{
		"username": "admin",
		"password": password,
	}
	payloadBytes, _ := json.Marshal(payload)

	// Create a custom Transport that skips TLS verification
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	// Create a custom HTTP client with the custom Transport
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	// Perform the HTTP POST request to obtain the token
	req, err := http.NewRequest("POST", argoURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read response body for more detailed error message
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("authentication failed: %v, response: %s", resp.Status, string(bodyBytes))
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	token, ok := response["token"].(string)
	if !ok {
		return "", fmt.Errorf("token not found in response")
	}

	return token, nil
}



// GetAuthToken retrieves the current token, generating a new one if necessary
func GetAuthToken(c *Controller, password string) (string, error) {
    tokenLock.Lock()
    defer tokenLock.Unlock()

    if token == "" {
        var err error
        token, err = GenerateAuthToken(password)
        if err != nil {
            return "", fmt.Errorf("failed to generate auth token: %v", err)
        }
        go scheduleTokenRefresh(c, password)
    }

    return token, nil
}


// scheduleTokenRefresh sets up a routine to refresh the token before it expires and update clients
func scheduleTokenRefresh(c *Controller, password string) {
    for {
        // Schedule the token refresh before the actual expiry time
        time.Sleep(23 * time.Hour) // Assuming a 24-hour token expiry, adjust as necessary
        tokenLock.Lock()

        newToken, err := GenerateAuthToken(password)
        if err != nil {
            fmt.Printf("failed to refresh token: %v\n", err)
            tokenLock.Unlock()
            continue
        }

        token = newToken
        err = refreshClients(c, newToken) // Pass controller reference
        if err != nil {
            fmt.Printf("failed to refresh clients: %v\n", err)
        }

        tokenLock.Unlock()
    }
}


// CreateArgoCDClient creates and returns an Argo CD client
func CreateArgoCDClient(authToken string) (apiclient.Client, error) {
    // Use the correct port for HTTPS
    argoURL := "argo-cd-argocd-server.argocd.svc.cluster.local:443"

    // Create Argo CD client options with the token
    argoClientOpts := apiclient.ClientOptions{
        ServerAddr: argoURL,
        AuthToken:  authToken,
    }

    // Create the Argo CD client
    argoClient, err := apiclient.NewClient(&argoClientOpts)
    if err != nil {
        return nil, fmt.Errorf("failed to create Argo CD client: %v", err)
    }

    return argoClient, nil
}

// refreshClients updates the Argo CD clients with the new token and updates the Controller struct
func refreshClients(c *Controller, newToken string) error {
    // Update the Argo CD client with the new token
    newArgoClient, err := CreateArgoCDClient(newToken)
    if err != nil {
        return fmt.Errorf("failed to create Argo CD client: %v", err)
    }

    // Update the clients stored in the controller struct
    c.argoClient = newArgoClient

    conn, newClusterClient, err := newArgoClient.NewClusterClient()
    if err != nil {
        return fmt.Errorf("failed to create ArgoCD cluster client: %v", err)
    }
    defer conn.Close()

    closer, newAppSetClient, err := newArgoClient.NewApplicationSetClient()
    if err != nil {
        return fmt.Errorf("failed to create ApplicationSet client: %v", err)
    }
    defer closer.Close()

    // Update the clients in the Controller struct
    c.appSetClient = newAppSetClient
    c.clusterClient = newClusterClient

    return nil
}
