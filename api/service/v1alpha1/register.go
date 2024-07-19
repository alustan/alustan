package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/runtime/schema"
    
)

const GroupName = "alustan.io"
const GroupVersion = "v1alpha1"

var SchemaGroupVersion = schema.GroupVersion{Group:GroupName, Version: GroupVersion}

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme = SchemeBuilder.AddToScheme
)


// AddToScheme adds the custom resource types to the scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemaGroupVersion,
	 &Service{},
	 &ServiceList{},
	)

   metav1.AddToGroupVersion(scheme,SchemaGroupVersion)
    return nil
}
