package util

import (
	"context"
	"log"
	"fmt"

	"k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetConfigMapContent retrieves the content of the ConfigMap based on its name and key.
func GetConfigMapContent(clientset  kubernetes.Interface, namespace, name, key string) (string, error) {
	// Retrieve the ConfigMap
	configMap, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		log.Printf("Failed to get ConfigMap '%s': %v", name, err)
		return "", err
	}

	// Extract the data from the ConfigMap using the specified key
	data, exists := configMap.Data[key]
	if !exists {
		errMsg := logErrorAndReturn("Key '%s' not found in ConfigMap '%s'", key, name)
		return "", errMsg
	}

	return data, nil
}

// logErrorAndReturnConfigMap logs the error and returns it
func logErrorAndReturnConfigMap(format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	log.Println(err)
	return err
}
