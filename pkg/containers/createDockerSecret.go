package containers

import (
    "context"
    "encoding/base64"
    "fmt"
    "log"

    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// CreateDockerConfigSecret creates a Kubernetes Secret of type kubernetes.io/dockerconfigjson
// dockerConfigJSON should be base64-encoded JSON string.
func CreateDockerConfigSecret(clientset *kubernetes.Clientset, secretName, namespace, encodedDockerConfigJSON string) error {
    // Decode the base64 string to verify it's correct
    decodedData, err := base64.StdEncoding.DecodeString(encodedDockerConfigJSON)
    if err != nil {
        return fmt.Errorf("invalid base64 encoded docker config JSON: %v", err)
    }

    // Define the secret
    secret := &corev1.Secret{
        ObjectMeta: metav1.ObjectMeta{
            Name:      secretName,
            Namespace: namespace,
        },
        Data: map[string][]byte{
            ".dockerconfigjson": decodedData,
        },
        Type: corev1.SecretTypeDockerConfigJson,
    }

    // Attempt to create the secret
    _, err = clientset.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
    if err != nil {
        // If the secret already exists, update it if necessary
        if apierrors.IsAlreadyExists(err) {
            log.Printf("Secret %s already exists, checking if it needs to be updated", secretName)
            
            // Fetch the existing secret
            existingSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
            if err != nil {
                return fmt.Errorf("failed to get existing secret: %v", err)
            }

            // Check if the existing secret's data is different from the new data
            if existingSecret.Data[".dockerconfigjson"] == nil || !equal(existingSecret.Data[".dockerconfigjson"], decodedData) {
                log.Printf("Secret %s needs to be updated", secretName)
                existingSecret.Data[".dockerconfigjson"] = decodedData

                // Update the existing secret
                _, err = clientset.CoreV1().Secrets(namespace).Update(context.TODO(), existingSecret, metav1.UpdateOptions{})
                if err != nil {
                    return fmt.Errorf("failed to update existing secret: %v", err)
                }
            } else {
                log.Printf("Secret %s is already up-to-date", secretName)
            }
        } else {
            return fmt.Errorf("failed to create secret: %v", err)
        }
    }

    return nil
}

// equal checks if two byte slices are equal
func equal(a, b []byte) bool {
    if len(a) != len(b) {
        return false
    }
    for i := range a {
        if a[i] != b[i] {
            return false
        }
    }
    return true
}

