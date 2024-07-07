package kubernetes

import (
    "context"
    "log"
    "time"

    "github.com/alustan/api/v1alpha1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/client-go/dynamic"
    
    "k8s.io/client-go/util/retry"
    "github.com/mitchellh/mapstructure"
)

// UpdateStatus updates the status of a Terraform custom resource.
func UpdateStatus(dynamicClient dynamic.Interface, namespace, name string, status v1alpha1.ParentResourceStatus) error {
    resource := schema.GroupVersionResource{
        Group:    "alustan.io",
        Version:  "v1alpha1",
        Resource: "terraforms",
    }

    retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
        // Get the existing resource
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()

        tf, err := dynamicClient.Resource(resource).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
        if err != nil {
            log.Printf("Failed to get resource %s in namespace %s: %v", name, namespace, err)
            return err
        }

        // Convert status struct to map
        statusMap, err := toMap(status)
        if err != nil {
            log.Printf("Failed to convert status to map: %v", err)
            return err
        }

        // Ensure the status field exists
        if tf.Object["status"] == nil {
            tf.Object["status"] = make(map[string]interface{})
        }

        // Initialize missing status fields
        statusFields := []string{"Credentials", "Finalized", "IngressURLs", "Message", "Output", "PostDeployOutput", "State"}
        for _, field := range statusFields {
            if _, exists := tf.Object["status"].(map[string]interface{})[field]; !exists {
                tf.Object["status"].(map[string]interface{})[field] = nil
            }
        }

        // Update the status fields
        for k, v := range statusMap {
            tf.Object["status"].(map[string]interface{})[k] = v
        }

        // Print the status object before updating
        log.Printf("Updating Terraform status: %+v\n", tf.Object["status"])

        // Update the resource with the new status
        _, err = dynamicClient.Resource(resource).Namespace(namespace).UpdateStatus(ctx, tf, metav1.UpdateOptions{})
        if err != nil {
            log.Printf("Failed to update Terraform status: %v", err)
            return err
        }

        log.Println("Successfully updated status for resource", name)
        return nil
    })

    if retryErr != nil {
        log.Printf("Failed to update status for resource %s in namespace %s after retrying: %v", name, namespace, retryErr)
        return retryErr
    }

    return nil
}

// toMap converts the status struct to a map.
func toMap(status v1alpha1.ParentResourceStatus) (map[string]interface{}, error) {
    var statusMap map[string]interface{}
    err := mapstructure.Decode(status, &statusMap)
    if err != nil {
        return nil, err
    }
    return statusMap, nil
}
