package schematypes

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

type SyncRequestWrapper struct {
	SyncRequest
}

func (s *SyncRequestWrapper) GetNamespace() string {
	return s.Parent.Metadata.Namespace
}

func (s *SyncRequestWrapper) GetName() string {
	return s.Parent.Metadata.Name
}

func (s *SyncRequestWrapper) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }
func (s *SyncRequestWrapper) SetGroupVersionKind(gvk schema.GroupVersionKind) {}
func (s *SyncRequestWrapper) GroupVersionKind() schema.GroupVersionKind { return schema.GroupVersionKind{} }
func (s *SyncRequestWrapper) SetNamespace(namespace string) {}
func (s *SyncRequestWrapper) SetName(name string) {}
func (s *SyncRequestWrapper) GetUID() types.UID { return "" }
func (s *SyncRequestWrapper) SetUID(uid types.UID) {}
func (s *SyncRequestWrapper) GetGeneration() int64 { return 0 }
func (s *SyncRequestWrapper) SetGeneration(generation int64) {}
func (s *SyncRequestWrapper) GetSelfLink() string { return "" }
func (s *SyncRequestWrapper) SetSelfLink(selfLink string) {}
func (s *SyncRequestWrapper) GetCreationTimestamp() metav1.Time { return metav1.Time{} }
func (s *SyncRequestWrapper) SetCreationTimestamp(timestamp metav1.Time) {}
func (s *SyncRequestWrapper) GetDeletionTimestamp() *metav1.Time { return nil }
func (s *SyncRequestWrapper) SetDeletionTimestamp(timestamp *metav1.Time) {}
func (s *SyncRequestWrapper) GetDeletionGracePeriodSeconds() *int64 { return nil }
func (s *SyncRequestWrapper) SetDeletionGracePeriodSeconds(*int64) {}
func (s *SyncRequestWrapper) GetLabels() map[string]string { return nil }
func (s *SyncRequestWrapper) SetLabels(labels map[string]string) {}
func (s *SyncRequestWrapper) GetAnnotations() map[string]string { return nil }
func (s *SyncRequestWrapper) SetAnnotations(annotations map[string]string) {}
func (s *SyncRequestWrapper) GetOwnerReferences() []metav1.OwnerReference { return nil }
func (s *SyncRequestWrapper) SetOwnerReferences([]metav1.OwnerReference) {}
func (s *SyncRequestWrapper) GetFinalizers() []string { return nil }
func (s *SyncRequestWrapper) SetFinalizers(finalizers []string) {}
func (s *SyncRequestWrapper) GetClusterName() string { return "" }
func (s *SyncRequestWrapper) SetClusterName(clusterName string) {}
func (s *SyncRequestWrapper) GetManagedFields() []metav1.ManagedFieldsEntry { return nil }
func (s *SyncRequestWrapper) SetManagedFields(managedFields []metav1.ManagedFieldsEntry) {}
