package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "encoding/json"
)

// +groupName=alustan.io

// TerraformConfigSpec defines the desired state of Terraform
type TerraformConfigSpec struct {
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

// ParentResourceStatus defines the observed state of Terraform
type ParentResourceStatus struct {
    State            string          `json:"state"`
    Message          string          `json:"message"`
    Output           json.RawMessage `json:"output,omitempty"`           
    PostDeployOutput json.RawMessage `json:"postDeployOutput,omitempty"` 
    IngressURLs      json.RawMessage `json:"ingressURLs,omitempty"`      
    Credentials      json.RawMessage `json:"credentials,omitempty"`      
    Finalized        bool            `json:"finalized"`
    ObservedGeneration int          `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=terraforms,scope=Namespaced

// Terraform is the Schema for the terraforms API
type Terraform struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   TerraformConfigSpec  `json:"spec,omitempty"`
    Status ParentResourceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TerraformList contains a list of Terraform
type TerraformList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []Terraform `json:"items"`
}

// SyncRequest represents the sync request from the controller
type SyncRequest struct {
    Parent     Terraform `json:"parent"`
    Finalizing bool      `json:"finalizing"`
}

// SyncResponse represents the sync response from the controller
type SyncResponse struct {
    Status    ParentResourceStatus `json:"status,omitempty"`
    Finalized bool                 `json:"finalized"`
}


