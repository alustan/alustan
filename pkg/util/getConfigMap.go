package util

import (
	"context"
	
	"fmt"

	"k8s.io/client-go/kubernetes"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetConfigMapContent retrieves the content of the ConfigMap based on its name and key.
func GetConfigMapContent(logger *zap.SugaredLogger,clientset  kubernetes.Interface, namespace, name, key string) (string, error) {
	// Retrieve the ConfigMap
	configMap, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		logger.Infof("Failed to get ConfigMap '%s': %v", name, err)
		return "", err
	}

	// Extract the data from the ConfigMap using the specified key
	data, exists := configMap.Data[key]
	if !exists {
		errMsg := logErrorAndReturnConfigMap(logger,"Key '%s' not found in ConfigMap '%s'", key, name)
		return "", errMsg
	}

	return data, nil
}

// logErrorAndReturnConfigMap logs the error and returns it
func logErrorAndReturnConfigMap(logger *zap.SugaredLogger,format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	logger.Info(err)
	return err
}
