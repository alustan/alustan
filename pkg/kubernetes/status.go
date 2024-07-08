


package kubernetes

import (
	"context"
	"log"
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"github.com/alustan/api/v1alpha1"
	"k8s.io/client-go/util/retry"
)

// UpdateStatus updates the status subresource of a Custom Resource
func UpdateStatus(dynClient dynamic.Interface, namespace, name string, status v1alpha1.TerraformStatus) error {
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

	// Check if the status subresource is defined
	if _, found := unstructuredResource.Object["status"]; !found {
		log.Printf("Status subresource not found for resource %s in namespace %s", name, namespace)
		return errors.New("status subresource not defined")
	}

	// Update the status
	unstructuredResource.Object["status"] = status

	// Update the resource with the new status
	updatedResource, err := dynClient.Resource(resource).Namespace(namespace).UpdateStatus(context.Background(), unstructuredResource, metav1.UpdateOptions{})
	if err != nil {
		log.Printf("Failed to update status for resource %s in namespace %s: %v", name, namespace, err)
		return err
	}

	log.Printf("Successfully updated status for resource %s in namespace %s", updatedResource.GetName(), namespace)
	return nil
})

if retryErr != nil {
	log.Printf("Failed to update status for resource %s in namespace %s after retrying: %v", name, namespace, retryErr)
	return retryErr
}

return nil
}


