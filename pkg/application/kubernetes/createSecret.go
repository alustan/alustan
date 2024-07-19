package kubernetes

import (
    "context"
    "encoding/base64"
    "fmt"
    
    "go.uber.org/zap"
	
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    
)


func CreateOrUpdateSecretWithGitHubPAT(logger *zap.SugaredLogger, clientset kubernetes.Interface, namespace, secretName, key, gitHubPATBase64 string) error {
	// Decode the base64-encoded GitHub PAT to ensure it's valid base64 data
	_, err := base64.StdEncoding.DecodeString(gitHubPATBase64)
	if err != nil {
		logger.Errorf("Failed to decode base64 GitHub PAT: %v", err)
		return fmt.Errorf("failed to decode base64 GitHub PAT: %v", err)
	}

	// Define the secret object
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			key: []byte(gitHubPATBase64),
		},
		Type: corev1.SecretTypeOpaque,
	}

	// Check if the secret already exists
	existingSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err == nil {
		// If the secret exists, check if the content is the same
		existingPAT, exists := existingSecret.Data[key]
		if exists && string(existingPAT) == gitHubPATBase64 {
			logger.Infof("Secret %s already exists with the same content, no update needed", secretName)
			return nil
		}

		// If the content is different, update the secret
		logger.Infof("Updating secret %s with new content", secretName)
		_, err = clientset.CoreV1().Secrets(namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
		if err != nil {
			logger.Errorf("Failed to update secret %s: %v", secretName, err)
			return fmt.Errorf("failed to update secret: %v", err)
		}
	} else {
		// If the secret does not exist, create it
		logger.Infof("Creating secret %s", secretName)
		_, err = clientset.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil {
			logger.Errorf("Failed to create secret %s: %v", secretName, err)
			return fmt.Errorf("failed to create secret: %v", err)
		}
	}

	logger.Infof("Secret %s created/updated successfully", secretName)
	return nil
}
