package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	
   

	appv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/repocreds"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
	"golang.org/x/crypto/ssh"
)

// Global lock to prevent concurrent installations
var installLock sync.Mutex

func main() {
	// Setup logging
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sugar := logger.Sugar()


    version := os.Getenv("ARGOCD_VERSION")
	if version == "" {
		sugar.Fatal("ARGOCD_VERSION environment variable is not set")
	}

	// Create Kubernetes clients using in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		sugar.Fatalf("Failed to build Kubernetes config: %v", err)
	}

	config.QPS = 100.0
	config.Burst = 200

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		sugar.Fatalf("Failed to create Kubernetes clientset: %v", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		sugar.Fatalf("Failed to create dynamic client: %v", err)
	}

	// Call InstallArgoCD function
	err = InstallArgoCD(sugar, clientset, dynClient, version)
	if err != nil {
		sugar.Fatalf("Failed to install ArgoCD: %v", err)
	}

	// Retrieve ArgoCD admin password
	adminPassword, err := getArgoCDAdminPassword(clientset)
	if err != nil {
		sugar.Fatalf("Failed to get ArgoCD admin password: %v", err)
	}

	// Obtain ArgoCD token using admin username and password
	token, err := getArgoCDToken("admin", adminPassword)
	if err != nil {
		sugar.Fatalf("Failed to get ArgoCD token: %v", err)
	}

	// Set up ArgoCD client with the token
	argoClient, err := argocdclient.NewClient(&argocdclient.ClientOptions{
		ServerAddr:  "argo-cd-argocd-server.argocd.svc.cluster.local",
	    AuthToken:   token,
	    PlainText:   true,
	})
	if err != nil {
		sugar.Fatalf("Failed to create ArgoCD client: %v", err)
	}

	// Instantiate NewRepoCredsClient and store SSH key from environment variable
	err = storeSSHKeyFromEnv(argoClient, sugar)
	if err != nil {
		sugar.Fatalf("Failed to store SSH key: %v", err)
	}

	sugar.Info("ArgoCD installation and setup completed successfully")
}

func InstallArgoCD(logger *zap.SugaredLogger, clientset kubernetes.Interface, dynClient dynamic.Interface, version string) error {
	// Get ArgoCD configuration from environment variable
	argocdConfig := os.Getenv("ARGOCD_CONFIG")

	// Check if ArgoCD is already installed and ready
	installed, ready, err := isArgoCDInstalledAndReady(logger, clientset)
	if err != nil {
		return fmt.Errorf("failed to check if ArgoCD is installed and ready: %w", err)
	}

	if installed && ready {
		logger.Info("ArgoCD is already installed and ready")
		return nil
	}

	// Lock to prevent concurrent installations
	installLock.Lock()
	defer installLock.Unlock()

	// Check again if ArgoCD is still not ready after acquiring lock
	installed, ready, err = isArgoCDInstalledAndReady(logger, clientset)
	if err != nil {
		return fmt.Errorf("failed to check if ArgoCD is installed and ready after acquiring lock: %w", err)
	}

	if installed && ready {
		logger.Info("ArgoCD is already installed and ready after acquiring lock")
		return nil
	}

	// Install ArgoCD using Helm
	err = installArgoCDWithHelm(logger, clientset, argocdConfig, version)
	if err != nil {
		return fmt.Errorf("failed to install ArgoCD with Helm: %w", err)
	}

	logger.Info("ArgoCD installed successfully")
	return nil
}

func isArgoCDInstalledAndReady(logger *zap.SugaredLogger, clientset kubernetes.Interface) (bool, bool, error) {
	_, err := clientset.CoreV1().Namespaces().Get(context.TODO(), "argocd", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, false, nil
		}
		return false, false, err
	}

	// Check for the presence and readiness of ArgoCD components
	deployments := []string{
		"argo-cd-argocd-applicationset-controller",
		"argo-cd-argocd-notifications-controller",
		"argo-cd-argocd-server",
		"argo-cd-argocd-repo-server",
		"argo-cd-argocd-redis",
		"argo-cd-argocd-dex-server",
	}

	for _, deployment := range deployments {
		deploy, err := clientset.AppsV1().Deployments("argocd").Get(context.TODO(), deployment, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				logger.Info("ArgoCD Components not found. Installing...")
				return false, false, nil
			}
			return false, false, err
		}

		// Check if the number of ready replicas matches the desired replicas
		if deploy.Status.ReadyReplicas != *deploy.Spec.Replicas {
			return true, false, nil // Components are installed but not ready
		}
	}

	return true, true, nil // All components are installed and ready
}

func installArgoCDWithHelm(logger *zap.SugaredLogger, clientset kubernetes.Interface, argocdConfig, version string) error {
	settings := cli.New()
	actionConfig := new(action.Configuration)

	err := actionConfig.Init(settings.RESTClientGetter(), "argocd", "", logger.Infof)
	if err != nil {
		return fmt.Errorf("failed to initialize Helm action configuration: %w", err)
	}

	// Add the repository
	repoEntry := repo.Entry{
		Name: "argo-cd",
		URL:  "https://argoproj.github.io/argo-helm",
	}
	chartRepo, err := repo.NewChartRepository(&repoEntry, getter.All(settings))
	if err != nil {
		return fmt.Errorf("failed to create chart repository: %w", err)
	}

	_, err = chartRepo.DownloadIndexFile()
	if err != nil {
		return fmt.Errorf("failed to download index file: %w", err)
	}

	repoFile := &repo.File{}
	if _, err := os.Stat(settings.RepositoryConfig); err == nil {
		repoFile, err = repo.LoadFile(settings.RepositoryConfig)
		if err != nil {
			return fmt.Errorf("failed to load repository config: %w", err)
		}
	}

	if !repoFile.Has(repoEntry.Name) {
		repoFile.Update(&repoEntry)
		if err := repoFile.WriteFile(settings.RepositoryConfig, 0644); err != nil {
			return fmt.Errorf("failed to write repository config: %w", err)
		}
	}

	chartName := "argo-cd/argo-cd"
	install := action.NewInstall(actionConfig)
	chartPath, err := install.LocateChart(chartName, settings)
	if err != nil {
		return fmt.Errorf("failed to locate chart: %w", err)
	}

	chart, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart: %w", err)
	}

	valOpts := &values.Options{}
	defaultVals, err := valOpts.MergeValues(getter.All(settings))
	if err != nil {
		return fmt.Errorf("failed to get default values: %w", err)
	}

	var vals map[string]interface{}
	if argocdConfig != "" {
		// Parse the provided YAML values
		providedVals := map[string]interface{}{}
		err = yaml.Unmarshal([]byte(argocdConfig), &providedVals)
		if err != nil {
			return fmt.Errorf("failed to parse ArgoCD configuration: %w", err)
		}

		
		// Merge provided values with default values
		vals = chartutil.CoalesceTables(providedVals, defaultVals)
	} else {
		vals = defaultVals
	}

	// Check if another operation is in progress
	statusClient := action.NewStatus(actionConfig)
	release, err := statusClient.Run("argo-cd")
	if err == nil && release.Info.Status.IsPending() {
		logger.Warn("Another operation is in progress for release argo-cd, skipping new operation.")
		return nil // Return without starting a new operation
	}

	// Perform install or upgrade with exponential backoff retry mechanism
	err = wait.ExponentialBackoff(RetryBackoff(), func() (bool, error) {
		histClient := action.NewHistory(actionConfig)
		histClient.Max = 1
		_, err := histClient.Run("argo-cd")
		if err == nil {
			// If the release exists, perform an upgrade
			err = upgradeArgoCD(actionConfig, chart, vals, logger)
		} else {
			// If the release does not exist, perform a new installation
			err = installArgoCD(actionConfig, chart, vals, logger)
		}

		if err != nil {
			return false, err
		}

		return true, nil // Stop retrying if successful
	})

	if err != nil {
		return fmt.Errorf("failed to install/upgrade ArgoCD with Helm: %w", err)
	}

	return nil
}

func upgradeArgoCD(actionConfig *action.Configuration, chart *chart.Chart, vals map[string]interface{}, logger *zap.SugaredLogger) error {
	upgrade := action.NewUpgrade(actionConfig)
	upgrade.Namespace = "argocd"
	upgrade.Wait = true
	upgrade.Timeout = 20 * time.Minute // Set timeout to 20 minutes
	upgrade.Atomic = true // Enable atomic option

	_, err := upgrade.Run("argo-cd", chart, vals)
	if err != nil {
		return fmt.Errorf("failed to upgrade ArgoCD with Helm: %w", err)
	}
	return nil
}

func installArgoCD(actionConfig *action.Configuration, chart *chart.Chart, vals map[string]interface{}, logger *zap.SugaredLogger) error {
	install := action.NewInstall(actionConfig)
	install.ReleaseName = "argo-cd"
	install.Namespace = "argocd"
	install.CreateNamespace = true
	install.Wait = true
	install.Timeout = 20 * time.Minute // Set timeout to 20 minutes
	install.Atomic = true // Enable atomic option

	_, err := install.Run(chart, vals)
	if err != nil {
		return fmt.Errorf("failed to install ArgoCD with Helm: %w", err)
	}
	return nil
}

func RetryBackoff() wait.Backoff {
	return wait.Backoff{
		Duration: 5 * time.Second, // Initial delay
		Factor:   2,               // Exponential factor
		Steps:    5,               // Number of retry attempts
	}
}

func getArgoCDAdminPassword(clientset kubernetes.Interface) (string, error) {
	namespace := "argocd"
    secretName := "argocd-initial-admin-secret"

    // Retrieve the secret from Kubernetes
    secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
    if err != nil {
        return "", fmt.Errorf("failed to get secret: %v", err)
    }

    // Ensure the password field exists
    passwordBytes, exists := secret.Data["password"]
    if !exists {
        return "", fmt.Errorf("password not found in secret")
    }

    // Print the raw password for debugging
    password := string(passwordBytes)
   


    return password, nil
}

func getArgoCDToken(username, password string) (string, error) {
	argoURL := "http://argo-cd-argocd-server.argocd.svc.cluster.local/api/v1/session"
    payload := map[string]string{
        "username": username,
        "password": password,
    }
    payloadBytes, _ := json.Marshal(payload)


    client := &http.Client{
        Timeout: 30 * time.Second,
    }

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

func storeSSHKeyFromEnv(argoClient argocdclient.Client, logger *zap.SugaredLogger) error {
    // Retrieve the secret and org URL from environment variables
    secretValue := os.Getenv("GIT_SSH_SECRET")
    orgURL := os.Getenv("GIT_ORG_URL")

    // Check if SSH secret and org URL are provided
    if secretValue == "" {
        return nil // If no SSH secret is provided, nothing to do
    }

    if orgURL == "" {
        return fmt.Errorf("organization URL (GIT_ORG_URL) must be provided when SSH secret is set")
    }

    // Trim any leading and trailing spaces from the SSH secret
    trimmedSecret := strings.TrimSpace(secretValue)

    // Validate the SSH private key
    if err := isValidSSHPrivateKey(trimmedSecret); err != nil {
        return fmt.Errorf("invalid SSH private key: %v", err)
    }

    repoCloser, repoCredsClient, err := argoClient.NewRepoCredsClient()
    if err != nil {
        return fmt.Errorf("failed to create repo creds client: %v", err)
    }
    defer repoCloser.Close()

    // Create the request to store the SSH repo secret
    credsRequest := &repocreds.RepoCredsCreateRequest{
        Upsert: true,
        Creds: &appv1alpha1.RepoCreds{
            Type:          "git",
            URL:           orgURL,
            SSHPrivateKey: trimmedSecret,
        },
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Store the SSH repo secret using the repo creds client
    _, err = repoCredsClient.CreateRepositoryCredentials(ctx, credsRequest)
    if err != nil {
        return fmt.Errorf("failed to store SSH repo secret: %v", err)
    }

    logger.Info("SSH repo secret stored successfully")
    return nil
}

// Validate SSH private key using golang/crypto/ssh
func isValidSSHPrivateKey(key string) error {
   
    _, err := ssh.ParsePrivateKey([]byte(key))
    if err != nil {
        return fmt.Errorf("failed to parse private key: %v", err)
    }
    return nil
}