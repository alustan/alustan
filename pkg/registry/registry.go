package registry

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/alustan/pkg/imagetag"
	"github.com/alustan/pkg/util"
	containers "github.com/alustan/pkg/containers"
	"github.com/alustan/pkg/schematypes"
)

func GetTaggedImageName(
	observed schematypes.SyncRequest,
	scriptContent string,
	clientset kubernetes.Interface,
) (string, schematypes.ParentResourceStatus) {
	var status schematypes.ParentResourceStatus

	if observed.Finalizing {
		taggedImageName, err := getTaggedImageNameFromConfigMap(clientset, observed.Parent.Metadata.Namespace, observed.Parent.Metadata.Name)
		if err != nil {
			status = util.ErrorResponse("retrieving tagged image name", err)
			return "", status
		}
		return taggedImageName, status
	}

	taggedImageName, status := handleContainerRegistry(observed, scriptContent, clientset)
	return taggedImageName, status
}

func handleContainerRegistry(
	observed schematypes.SyncRequest,
	scriptContent string,
	clientset kubernetes.Interface,
) (string, schematypes.ParentResourceStatus) {
	var status schematypes.ParentResourceStatus

	encodedDockerConfigJSON := os.Getenv("CONTAINER_REGISTRY_SECRET")
	if encodedDockerConfigJSON == "" {
		log.Println("Environment variable CONTAINER_REGISTRY_SECRET is not set")
		status = util.ErrorResponse("creating Docker config secret", fmt.Errorf("CONTAINER_REGISTRY_SECRET is not set"))
		return "", status
	}

	secretName := fmt.Sprintf("%s-container-secret", observed.Parent.Metadata.Name)
	_, token, err := containers.CreateDockerConfigSecret(clientset, secretName, observed.Parent.Metadata.Namespace, encodedDockerConfigJSON)
	if err != nil {
		status = util.ErrorResponse("creating Docker config secret", err)
		return "", status
	}

	provider := observed.Parent.Spec.ContainerRegistry.Provider
	registryClient, err := getRegistryClient(provider, token)
	if err != nil {
		status = util.ErrorResponse("creating registry client", err)
		return "", status
	}

	image := observed.Parent.Spec.ContainerRegistry.ImageName
	tags, err := registryClient.GetTags(image)
	if err != nil {
		status = util.ErrorResponse("fetching image tags", err)
		return "", status
	}

	semanticVersion := observed.Parent.Spec.ContainerRegistry.SemanticVersion
	latestTag, err := getLatestTag(tags, semanticVersion)
	if err != nil {
		status = util.ErrorResponse("determining latest image tag", err)
		return "", status
	}

	taggedImageName := fmt.Sprintf("%s:%s", image, latestTag)
	err = updateTaggedImageConfigMap(clientset, observed.Parent.Metadata.Namespace, observed.Parent.Metadata.Name, taggedImageName)
	if err != nil {
		status = util.ErrorResponse("updating image tag in configmap", err)
		return "", status
	}

	return taggedImageName, status
}

func getRegistryClient(provider, token string) (imagetag.RegistryClientInterface, error) {
	switch provider {
	case "ghcr":
		return imagetag.NewGHCRClient(token), nil
	case "docker":
		return imagetag.NewDockerHubClient(token), nil
	default:
		return nil, fmt.Errorf("unknown container registry provider: %s", provider)
	}
}

func getLatestTag(tags []string, semanticVersion string) (string, error) {
	constraint, err := semver.NewConstraint(semanticVersion)
	if err != nil {
		return "", fmt.Errorf("error parsing semantic version constraint: %w", err)
	}

	var latestVersion *semver.Version
	for _, tag := range tags {
		version, err := semver.NewVersion(tag)
		if err != nil {
			continue // Skip tags that are not valid semantic versions
		}

		if constraint.Check(version) {
			if latestVersion == nil || version.GreaterThan(latestVersion) {
				latestVersion = version
			}
		}
	}

	if latestVersion == nil {
		return "", fmt.Errorf("no valid versions found for constraint %s", semanticVersion)
	}

	return latestVersion.String(), nil
}

// updateTaggedImageConfigMap updates or creates a ConfigMap with the tagged image name
func updateTaggedImageConfigMap(clientset kubernetes.Interface, namespace, name, taggedImageName string) error {
	configMapData := map[string]string{
		"lastTaggedImage": taggedImageName,
	}
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-tagged-image", name),
			Namespace: namespace,
		},
		Data: configMapData,
	}

	// Try to create the ConfigMap
	_, err := clientset.CoreV1().ConfigMaps(namespace).Create(context.Background(), configMap, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			// If the ConfigMap already exists, update it
			existingConfigMap, getErr := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMap.Name, metav1.GetOptions{})
			if getErr != nil {
				return fmt.Errorf("failed to get existing ConfigMap: %v", getErr)
			}
			existingConfigMap.Data = configMapData
			_, updateErr := clientset.CoreV1().ConfigMaps(namespace).Update(context.Background(), existingConfigMap, metav1.UpdateOptions{})
			if updateErr != nil {
				return fmt.Errorf("failed to update existing ConfigMap: %v", updateErr)
			}
		} else {
			return fmt.Errorf("failed to create ConfigMap: %v", err)
		}
	}
	return nil
}

func getTaggedImageNameFromConfigMap(clientset kubernetes.Interface, namespace, name string) (string, error) {
	configMapName := fmt.Sprintf("%s-tagged-image", name)
	configMap, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMapName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get ConfigMap: %v", err)
	}
	taggedImageName, ok := configMap.Data["lastTaggedImage"]
	if !ok {
		return "", fmt.Errorf("tagged image name not found in ConfigMap")
	}
	return taggedImageName, nil
}
