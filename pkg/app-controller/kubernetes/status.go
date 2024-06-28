package kubernetes

import (
	"context"
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func UpdateStatus(dynClient dynamic.Interface, namespace, name string, status map[string]interface{}) error {
	resource := schema.GroupVersionResource{
		Group:    "alustan.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}

	// Fetch the existing resource
	unstructuredResource, err := dynClient.Resource(resource).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		log.Printf("Failed to get resource: %v", err)
		return err
	}

	// Update the status
	unstructuredResource.Object["status"] = status

	// Update the resource with the new status
	_, err = dynClient.Resource(resource).Namespace(namespace).UpdateStatus(context.Background(), unstructuredResource, metav1.UpdateOptions{})
	if err != nil {
		log.Printf("Failed to update status: %v", err)
		return err
	}

	return nil
}
