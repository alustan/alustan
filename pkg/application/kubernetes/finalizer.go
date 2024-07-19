package kubernetes

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
	"go.uber.org/zap"

	"github.com/alustan/alustan/pkg/util"
)

func AddFinalizer(logger *zap.SugaredLogger, dynamicClient dynamic.Interface, name, namespace string) error {
	gvr := schema.GroupVersionResource{
		Group:    "alustan.io",
		Version:  "v1alpha1",
		Resource: "services",
	}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get the existing resource
		unstructuredObj, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get Service resource: %v", err)
		}

		// Convert unstructured object to map[string]interface{}
		objMap := unstructuredObj.Object

		// Check if finalizer is already present
		finalizerName := "service.finalizer.alustan.io"
		finalizers, _, _ := unstructured.NestedStringSlice(objMap, "metadata", "finalizers")
		if util.ContainsString(finalizers, finalizerName) {
			logger.Infof("Finalizer %s already exists for Service %s in namespace %s", finalizerName, name, namespace)
			return nil
		}

		// Add finalizer
		finalizers = append(finalizers, finalizerName)
		err = unstructured.SetNestedStringSlice(objMap, finalizers, "metadata", "finalizers")
		if err != nil {
			return fmt.Errorf("failed to set finalizers: %v", err)
		}

		// Update the resource
		unstructuredObj.Object = objMap
		_, err = dynamicClient.Resource(gvr).Namespace(namespace).Update(context.TODO(), unstructuredObj, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update Service resource: %v", err)
		}

		logger.Infof("Added finalizer %s for Service %s in namespace %s", finalizerName, name, namespace)
		return nil
	})

	if retryErr != nil {
		logger.Errorf("Failed to add finalizer for Service %s in namespace %s: %v", name, namespace, retryErr)
		return retryErr
	}

	return nil
}


func RemoveFinalizer(logger *zap.SugaredLogger, dynamicClient dynamic.Interface, name, namespace string) error {
	gvr := schema.GroupVersionResource{
		Group:    "alustan.io",
		Version:  "v1alpha1",
		Resource: "services",
	}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get the existing resource
		unstructuredObj, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get Service resource: %v", err)
		}

		// Convert unstructured object to map[string]interface{}
		objMap := unstructuredObj.Object

		// Check if finalizer is present
		finalizerName := "service.finalizer.alustan.io"
		finalizers, _, _ := unstructured.NestedStringSlice(objMap, "metadata", "finalizers")
		if !util.ContainsString(finalizers, finalizerName) {
			logger.Infof("Finalizer %s not found for Service %s in namespace %s", finalizerName, name, namespace)
			return nil
		}

		// Remove finalizer
		finalizers = util.RemoveString(finalizers, finalizerName)
		err = unstructured.SetNestedStringSlice(objMap, finalizers, "metadata", "finalizers")
		if err != nil {
			return fmt.Errorf("failed to set finalizers: %v", err)
		}

		// Update the resource
		unstructuredObj.Object = objMap
		_, err = dynamicClient.Resource(gvr).Namespace(namespace).Update(context.TODO(), unstructuredObj, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update Service resource: %v", err)
		}

		logger.Infof("Removed finalizer %s for Service %s in namespace %s", finalizerName, name, namespace)
		return nil
	})

	if retryErr != nil {
		logger.Errorf("Failed to remove finalizer for Service %s in namespace %s: %v", name, namespace, retryErr)
		return retryErr
	}

	return nil
}
