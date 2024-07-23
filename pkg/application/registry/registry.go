package registry

import (

	"fmt"
	
	"os"

	"github.com/Masterminds/semver/v3"
	
	"k8s.io/client-go/kubernetes"
	
	"go.uber.org/zap"

	"github.com/alustan/alustan/pkg/imagetag"
	
	containers "github.com/alustan/alustan/pkg/containers"
	"github.com/alustan/alustan/api/app/v1alpha1"
	"github.com/alustan/alustan/pkg/application/errorstatus"
	
)



func HandleContainerRegistry(
	logger *zap.SugaredLogger,
	clientset kubernetes.Interface,
	observed *v1alpha1.App,
	
) (string, v1alpha1.AppStatus) {
	var status v1alpha1.AppStatus

	encodedDockerConfigJSON := os.Getenv("CONTAINER_REGISTRY_SECRET")
	if encodedDockerConfigJSON == "" {
		logger.Info("Environment variable CONTAINER_REGISTRY_SECRET is not set")
		status = errorstatus.ErrorResponse(logger,"creating Docker config secret", fmt.Errorf("CONTAINER_REGISTRY_SECRET is not set"))
		return "", status
	}

	secretName := fmt.Sprintf("%s-container-secret", observed.ObjectMeta.Name)
	_, token, err := containers.CreateDockerConfigSecret(logger,clientset, secretName, observed.ObjectMeta.Namespace, encodedDockerConfigJSON)
	if err != nil {
		status = errorstatus.ErrorResponse(logger,"creating Docker config secret", err)
		return "", status
	}

	provider := observed.Spec.ContainerRegistry.Provider
	registryClient, err := getRegistryClient(provider, token)
	if err != nil {
		status = errorstatus.ErrorResponse(logger,"creating registry client", err)
		return "", status
	}

	image := observed.Spec.ContainerRegistry.ImageName
	tags, err := registryClient.GetTags(image)
	if err != nil {
		status = errorstatus.ErrorResponse(logger,"fetching image tags", err)
		return "", status
	}

	semanticVersion := observed.Spec.ContainerRegistry.SemanticVersion
	latestTag, err := getLatestTag(tags, semanticVersion)
	if err != nil {
		status = errorstatus.ErrorResponse(logger,"determining latest image tag", err)
		return "", status
	}

	taggedImageName := fmt.Sprintf("%s:%s", image, latestTag)

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


