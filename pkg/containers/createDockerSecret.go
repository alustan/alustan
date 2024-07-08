package containers

import (
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    
     "strings"

     "go.uber.org/zap"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// DockerConfig represents the structure of the Docker config JSON.
type DockerConfig struct {
    Auths map[string]struct {
        Auth string `json:"auth"`
    } `json:"auths"`
}

// CreateDockerConfigSecret creates a Kubernetes Secret of type kubernetes.io/dockerconfigjson
// dockerConfigJSON should be base64-encoded JSON string.
// It returns the decoded username and password used in the Docker config JSON.
func CreateDockerConfigSecret(logger *zap.SugaredLogger,clientset kubernetes.Interface, secretName, namespace, encodedDockerConfigJSON string) (string, string, error) {
    // Decode the base64 string to verify it's correct
    decodedData, err := base64.StdEncoding.DecodeString(encodedDockerConfigJSON)
    if err != nil {
        return "", "", fmt.Errorf("invalid base64 encoded docker config JSON: %v", err)
    }

    // Parse the decoded JSON to extract the auth field
    var dockerConfig DockerConfig
    if err := json.Unmarshal(decodedData, &dockerConfig); err != nil {
        return "", "", fmt.Errorf("failed to parse docker config JSON: %v", err)
    }

    // Extract and decode the auth value (assuming a single auth entry)
    var username, password string
    for registry, authEntry := range dockerConfig.Auths {
        decodedAuth, err := base64.StdEncoding.DecodeString(authEntry.Auth)
        if err != nil {
            return "", "", fmt.Errorf("failed to decode auth for registry %s: %v", registry, err)
        }
        
        credentials := string(decodedAuth)
        parts := strings.SplitN(credentials, ":", 2)
        if len(parts) != 2 {
            return "", "", fmt.Errorf("invalid auth format for registry %s", registry)
        }
        username = parts[0]
        password = parts[1]
        break // Assume there's only one entry in the auths map
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
            logger.Infof("Secret %s already exists, checking if it needs to be updated", secretName)

            // Fetch the existing secret
            existingSecret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
            if err != nil {
                return "", "", fmt.Errorf("failed to get existing secret: %v", err)
            }

            // Check if the existing secret's data is different from the new data
            if existingSecret.Data[".dockerconfigjson"] == nil || !equal(existingSecret.Data[".dockerconfigjson"], decodedData) {
                logger.Infof("Secret %s needs to be updated", secretName)
                existingSecret.Data[".dockerconfigjson"] = decodedData

                // Update the existing secret
                _, err = clientset.CoreV1().Secrets(namespace).Update(context.TODO(), existingSecret, metav1.UpdateOptions{})
                if err != nil {
                    return "", "", fmt.Errorf("failed to update existing secret: %v", err)
                }
            } else {
                logger.Infof("Secret %s is already up-to-date", secretName)
            }
        } else {
            return "", "", fmt.Errorf("failed to create secret: %v", err)
        }
    }

    return username, password, nil
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
