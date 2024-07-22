package installargocd

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func InstallArgoCD(logger *zap.SugaredLogger, clientset kubernetes.Interface, dynClient dynamic.Interface, version string) error {
	// Check if ArgoCD is already installed
	installed, err := isArgoCDInstalled(clientset)
	if err != nil {
		return fmt.Errorf("failed to check if ArgoCD is installed: %w", err)
	}

	if installed {
		logger.Info("ArgoCD is already installed")
		return nil
	}

	// Get ArgoCD configuration from environment variable
	argocdConfig := os.Getenv("ARGOCD_CONFIG")

	// Install ArgoCD using Helm
	err = installArgoCDWithHelm(logger, argocdConfig, version)
	if err != nil {
		return fmt.Errorf("failed to install ArgoCD with Helm: %w", err)
	}

	logger.Info("ArgoCD installed successfully")
	return nil
}

func isArgoCDInstalled(clientset kubernetes.Interface) (bool, error) {
	_, err := clientset.CoreV1().Namespaces().Get(context.TODO(), "argocd", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	// Check for the presence of ArgoCD components
	deployments := []string{"argocd-server", "argocd-repo-server", "argocd-application-controller"}
	for _, deployment := range deployments {
		_, err = clientset.AppsV1().Deployments("argocd").Get(context.TODO(), deployment, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
	}

	return true, nil
}

func installArgoCDWithHelm(logger *zap.SugaredLogger, argocdConfig, version string) error {
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

		// Merge default values with provided values
		vals = deepMerge(defaultVals, providedVals)
	} else {
		// Use default values if no configuration is provided
		vals = defaultVals
	}

	// Check if the release already exists
	histClient := action.NewHistory(actionConfig)
	histClient.Max = 1
	_, err = histClient.Run("argo-cd")
	if err == nil {
		// If the release exists, perform an upgrade with retry mechanism
		return wait.ExponentialBackoff(retryBackoff(), func() (bool, error) {
			return false, upgradeArgoCD(actionConfig, chart, vals, logger)
		})
	}

	// If the release does not exist, perform a new installation with retry mechanism
	return wait.ExponentialBackoff(retryBackoff(), func() (bool, error) {
		return false, installArgoCD(actionConfig, chart, vals, logger)
	})
}

func upgradeArgoCD(actionConfig *action.Configuration, chart *chart.Chart, vals map[string]interface{}, logger *zap.SugaredLogger) error {
	upgrade := action.NewUpgrade(actionConfig)
	upgrade.Namespace = "argocd"
	upgrade.Wait = true
	upgrade.Timeout = 20 * time.Minute // Set timeout to 20 minutes

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

	_, err := install.Run(chart, vals)
	if err != nil {
		return fmt.Errorf("failed to install ArgoCD with Helm: %w", err)
	}
	return nil
}

func deepMerge(dst, src map[string]interface{}) map[string]interface{} {
	for key, srcValue := range src {
		if srcMap, ok := srcValue.(map[string]interface{}); ok {
			if dstValue, found := dst[key]; found {
				if dstMap, ok := dstValue.(map[string]interface{}); ok {
					dst[key] = deepMerge(dstMap, srcMap)
					continue
				}
			}
		}
		dst[key] = srcValue
	}
	return dst
}

func retryBackoff() wait.Backoff {
	return wait.Backoff{
		Duration: 5 * time.Second, // Initial delay
		Factor:   2,               // Exponential factor
		Steps:    3,               // Number of retry attempts
	}
}
