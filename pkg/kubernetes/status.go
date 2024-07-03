package kubernetes

import (
	"context"
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
)

// UpdateStatus updates the status subresource of a Custom Resource
func UpdateStatus(dynClient dynamic.Interface, namespace, name string, status map[string]interface{}) error {
	resource := schema.GroupVersionResource{
		Group:    "alustan.io",
		Version:  "v1alpha1",
		Resource: "terraforms",
	}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Fetch the existing resource
		unstructuredResource, err := dynClient.Resource(resource).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			log.Printf("Failed to get resource %s in namespace %s: %v", name, namespace, err)
			return err
		}

		// Initialize the status field if not present
		if _, found := unstructuredResource.Object["status"]; !found {
			unstructuredResource.Object["status"] = map[string]interface{}{}
		}

		// Update the status
		unstructuredResource.Object["status"] = status

		// Update the resource with the new status
		_, updateErr := dynClient.Resource(resource).Namespace(namespace).UpdateStatus(context.Background(), unstructuredResource, metav1.UpdateOptions{})
		if updateErr != nil {
			log.Printf("Failed to update status for resource %s in namespace %s: %v", name, namespace, updateErr)
			return updateErr
		}

		log.Printf("Successfully updated status for resource %s in namespace %s", unstructuredResource.GetName(), namespace)
		return nil
	})

	if retryErr != nil {
		log.Printf("Failed to update status for resource %s in namespace %s after retrying: %v", name, namespace, retryErr)
		return retryErr
	}

	return nil
}
