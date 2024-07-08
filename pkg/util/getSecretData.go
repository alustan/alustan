package util

import (
	"context"
	
	"fmt"

	"k8s.io/client-go/kubernetes"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetDataFromSecret retrieves the SSH key from a Kubernetes Secret
func GetDataFromSecret(logger *zap.SugaredLogger,clientset  kubernetes.Interface, namespace, secretName, keyName string) (string, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		logger.Infof("Failed to get secret '%s': %v", secretName, err)
		return "", err
	}

	sshKey, ok := secret.Data[keyName]
	if !ok {
		errMsg := logErrorAndReturn(logger,"Key '%s' not found in secret '%s'", keyName, secretName)
		return "", errMsg
	}

	return string(sshKey), nil
}

// logErrorAndReturn logs the error and returns it
func logErrorAndReturn(logger *zap.SugaredLogger,format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	logger.Info(err)
	return err
}
