package kubernetes

import (
	"context"
    "fmt"
	
	"k8s.io/client-go/kubernetes"
	"go.uber.org/zap"
	
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
)

func CreateOrUpdateArgoSecret(logger *zap.SugaredLogger, clientset kubernetes.Interface, secretName, environment string) error {
	namespace := "argocd"
	labelSelector := "argocd.argoproj.io/secret-type=cluster,!alustan.io/secret-type"

	secrets, err := clientset.CoreV1().Secrets(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list secrets: %w", err)
	}

	if len(secrets.Items) > 0 {
		logger.Info("Found existing secret with required labels, returning without creating a new secret.")
		return nil
	}

	_, err = clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err == nil {
		logger.Infof("Secret %s already exists, deleting it.\n", secretName)
		err = clientset.CoreV1().Secrets(namespace).Delete(context.TODO(), secretName, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete existing secret: %w", err)
		}
	} else {
		logger.Infof("Secret %s not found, creating a new one.\n", secretName)
	}

	secretData := map[string]string{
		"name":   secretName,
		"server": "https://kubernetes.default.svc",
		"config": `{
			"tlsClientConfig": {
				"insecure": false
			}
		}`,
	}

	secretObj := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"argocd.argoproj.io/secret-type": "cluster",
				"alustan.io/secret-type": "cluster",
				"environment":                    environment,
			},
		},
		StringData: secretData,
		Type:       corev1.SecretTypeOpaque,
	}

	_, err = clientset.CoreV1().Secrets(namespace).Create(context.TODO(), secretObj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}

	logger.Infof("Secret %s created successfully.\n", secretName)
	return nil
}

