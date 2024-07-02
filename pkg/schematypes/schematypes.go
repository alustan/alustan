package schematypes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TerraformConfigSpec struct {
	Variables         map[string]string `json:"variables"`
	Scripts           Scripts           `json:"scripts"`
	PostDeploy        PostDeploy        `json:"postDeploy"`
	ContainerRegistry ContainerRegistry `json:"containerRegistry"`
}

type Scripts struct {
	Deploy  string `json:"deploy"`
	Destroy string `json:"destroy"`
}

type PostDeploy struct {
	Script string            `yaml:"script"`
	Args   map[string]string `yaml:"args"`
}

type ContainerRegistry struct {
	Provider        string `json:"provider"`
	ImageName       string `json:"imageName"`
	SemanticVersion string `json:"semanticVersion"`
}

type ParentResource struct {
	ApiVersion string              `json:"apiVersion"`
	Kind       string              `json:"kind"`
	Metadata   metav1.ObjectMeta   `json:"metadata"`
	Spec       TerraformConfigSpec `json:"spec"`
}

type SyncRequest struct {
	Parent     ParentResource `json:"parent"`
	Finalizing bool           `json:"finalizing"`
}