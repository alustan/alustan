package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    

)

// +groupName=alustan.io

// TerraformSpec defines the desired state of Terraform
type TerraformSpec struct {
    Variables         map[string]string `json:"variables"`
    Scripts           Scripts           `json:"scripts"`
    PostDeploy        PostDeploy        `json:"postDeploy"`
    ContainerRegistry ContainerRegistry `json:"containerRegistry"`
}

// Scripts defines the deployment and destruction scripts
type Scripts struct {
    Deploy  string `json:"deploy"`
    Destroy string `json:"destroy"`
}

// PostDeploy defines the post-deployment actions
type PostDeploy struct {
    Script string            `json:"script"`
    Args   map[string]string `json:"args"`
}

// ContainerRegistry defines the container registry settings
type ContainerRegistry struct {
    Provider        string `json:"provider"`
    ImageName       string `json:"imageName"`
    SemanticVersion string `json:"semanticVersion"`
}

// TerraformStatus defines the observed state of Terraform
type TerraformStatus struct {
	State            string                           `json:"state"`
	Message          string                           `json:"message"`
	PostDeployOutput map[string]runtime.RawExtension  `json:"postDeployOutput,omitempty"`
	ObservedGeneration int                         `json:"observedGeneration,omitempty"`
}


// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=terraforms,scope=Namespaced

// Terraform is the Schema for the terraforms API
type Terraform struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   TerraformSpec  `json:"spec,omitempty"`
    Status TerraformStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TerraformList contains a list of Terraform
type TerraformList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []Terraform `json:"items"`
}









