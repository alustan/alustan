package installargocd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	yamlmarshal "sigs.k8s.io/yaml"
)

func InstallArgoCD(logger *zap.SugaredLogger, clientset kubernetes.Interface, dynClient dynamic.Interface) error {
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

	// Apply the ArgoCD manifest with or without configurations
	if argocdConfig != "" {
		// Parse ArgoCD configuration
		parsedConfig := make(map[string]interface{})
		err = yaml.Unmarshal([]byte(argocdConfig), &parsedConfig)
		if err != nil {
			return fmt.Errorf("failed to parse ArgoCD configuration: %w", err)
		}

		err = applyManifestWithValues(dynClient, parsedConfig)
		if err != nil {
			return fmt.Errorf("failed to apply ArgoCD manifest with values: %w", err)
		}
	} else {
		err = applyManifestWithoutValues(dynClient)
		if err != nil {
			return fmt.Errorf("failed to apply ArgoCD manifest without values: %w", err)
		}
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

func applyManifestWithValues(dynClient dynamic.Interface, values map[string]interface{}) error {
	resp, err := fetchArgoCDManifest()
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	manifest, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Apply the parsed configurations to the manifest
	modifiedManifest, err := applyValuesToManifest(manifest, values)
	if err != nil {
		return err
	}

	return applyManifest(dynClient, modifiedManifest)
}

func applyManifestWithoutValues(dynClient dynamic.Interface) error {
	resp, err := fetchArgoCDManifest()
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	manifest, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return applyManifest(dynClient, manifest)
}

func fetchArgoCDManifest() (*http.Response, error) {
	// Ensure HTTP client has a timeout for safety
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	// Fetch the manifest using HTTP GET
	resp, err := client.Get("https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml")
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch ArgoCD manifest, status code: %d", resp.StatusCode)
	}

	return resp, nil
}

func applyValuesToManifest(manifest []byte, values map[string]interface{}) ([]byte, error) {
	var manifestMap map[string]interface{}
	err := yaml.Unmarshal(manifest, &manifestMap)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	mergedMap := deepMerge(manifestMap, values)

	modifiedManifest, err := yamlmarshal.Marshal(mergedMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified manifest: %w", err)
	}

	return modifiedManifest, nil
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

func applyManifest(dynClient dynamic.Interface, manifest []byte) error {
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifest), 4096)
	for {
		unstruct := &unstructured.Unstructured{}
		err := decoder.Decode(unstruct)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		gvr := schema.GroupVersionResource{
			Group:    unstruct.GetObjectKind().GroupVersionKind().Group,
			Version:  unstruct.GetObjectKind().GroupVersionKind().Version,
			Resource: strings.ToLower(unstruct.GetKind()) + "s",
		}

		_, err = dynClient.Resource(gvr).Namespace(unstruct.GetNamespace()).Create(context.TODO(), unstruct, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
