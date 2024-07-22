package installargocd


import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/apimachinery/pkg/api/errors"
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

	err := actionConfig.Init(settings.RESTClientGetter(), "argocd", os.Getenv("HELM_DRIVER"), logger.Infof)
	if err != nil {
		return fmt.Errorf("failed to initialize Helm action configuration: %w", err)
	}

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

	// Add the repository
	repoFile := settings.RepositoryConfig
	repoFile.Lock()
	defer repoFile.Unlock()
	if repoFile.Has(repoEntry.Name) == false {
		if err := repoFile.Add(&repoEntry, settings.RepositoryCache); err != nil {
			return fmt.Errorf("failed to add repository: %w", err)
		}
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

	install.ReleaseName = "argo-cd"
	install.Namespace = "argocd"
	install.CreateNamespace = true
	install.Wait = true

	valOpts := &values.Options{}
	defaultVals, err := valOpts.MergeValues(getter.All(settings))
	if err != nil {
		return fmt.Errorf("failed to get default values: %w", err)
	}

	var vals map[string]interface{}
	if argocdConfig != "" {
		// Parse the provided values
		providedVals := map[string]interface{}{}
		err = yaml.Unmarshal([]byte(argocdConfig), &providedVals)
		if err != nil {
			return fmt.Errorf("failed to parse ArgoCD configuration: %w", err)
		}

		// Merge default values with provided values
		mergedVals := deepMerge(defaultVals, providedVals)
		vals = mergedVals
	} else {
		vals = defaultVals
	}

	_, err = install.Run(chart, vals)
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
