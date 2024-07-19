package kubernetes

import (
    "context"
   
    "fmt"

    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    "k8s.io/client-go/dynamic"
    "k8s.io/client-go/util/retry"
    "k8s.io/apimachinery/pkg/api/errors"
    "go.uber.org/zap"

    "github.com/alustan/alustan/api/service/v1alpha1"
)

func UpdateStatus(logger *zap.SugaredLogger, dynamicClient dynamic.Interface, name, namespace string, status v1alpha1.ServiceStatus) error {
    gvr := schema.GroupVersionResource{
        Group:    "alustan.io",
        Version:  "v1alpha1",
        Resource: "services",
    }

    retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
        // Get the existing resource
        unstructuredService, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
        if err != nil {
            if errors.IsNotFound(err) {
                logger.Infof("Resource %s in namespace %s does not exist, assuming it has been deleted", name, namespace)
                return nil
            }
            return err
        }

        // Convert unstructured data to Service
        service := &v1alpha1.Service{}
        err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredService.Object, service)
        if err != nil {
            return fmt.Errorf("failed to convert unstructured data to Service: %v", err)
        }

        // Update the status
        service.Status = status

        // Convert back to unstructured data
        updatedUnstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(service)
        if err != nil {
            return fmt.Errorf("failed to convert Service to unstructured data: %v", err)
        }
        updatedUnstructured := &unstructured.Unstructured{Object: updatedUnstructuredMap}

        // Update the status of the resource
        _, err = dynamicClient.Resource(gvr).Namespace(namespace).UpdateStatus(context.Background(), updatedUnstructured, metav1.UpdateOptions{})
        if err != nil {
            return err
        }

        return nil
    })

    if retryErr != nil {
        logger.Errorf("Failed to update status for resource %s in namespace %s after retrying: %v", name, namespace, retryErr)
        return retryErr
    }

    // Log the updated status or perform additional actions
    logger.Infof("Updated status for %s in namespace %s", name, namespace)
    return nil
}
