package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    appv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
    

)

// +groupName=alustan.io

// ServiceSpec defines the desired state of Service
type ServiceSpec struct {
    Workspace          string             `json:"workspace"`
    PreviewEnvironment   PreviewEnvironment    `json:"previewEnvironment"` 
    Source           SourceSpec         `json:"source"`
    ContainerRegistry ContainerRegistry `json:"containerRegistry"`
    Dependencies     Dependencies       `json:"dependencies"`
}

type PreviewEnvironment struct {
	Enabled  bool   `json:"enabled"`
	GitOwner string `json:"gitOwner"`
	GitRepo  string `json:"gitRepo"`
}

// SourceSpec defines the source repository and deployment values
type SourceSpec struct {
    RepoURL        string                 `json:"repoURL"`
    Path           string                 `json:"path"`
    ReleaseName    string                 `json:"releaseName"`
    TargetRevision string                 `json:"targetRevision"`
    Values         map[string]runtime.RawExtension `json:"values,omitempty"`
}

// ContainerRegistry defines the container registry information
type ContainerRegistry struct {
    Provider        string `json:"provider"`
    ImageName       string `json:"imageName"`
    SemanticVersion string `json:"semanticVersion"`
}

// Dependencies defines the service dependencies
type Dependencies struct {
    Service []map[string]string `json:"service"`
}


// ServiceStatus defines the observed state of Service
type ServiceStatus struct {
    State    string    `json:"state"`
	Message   string    `json:"message,omitempty"`
    HealthStatus   appv1alpha1.ApplicationSetStatus     `json:"healthStatus,omitempty"`
    PreviewURLs    map[string]runtime.RawExtension     `json:"previewURLs,omitempty"`
	ObservedGeneration int                         `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=services,scope=Namespaced

// Service is the Schema for the services API
type Service struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   ServiceSpec   `json:"spec,omitempty"`
    Status ServiceStatus `json:"status,omitempty"`
}


// +kubebuilder:object:root=true

// ServiceList contains a list of Service
type ServiceList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []Service `json:"items"`
}


