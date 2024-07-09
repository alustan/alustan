package kubernetes

import (
	"context"
	"encoding/base64"
	"fmt"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Credentials struct holds ArgoCD and Grafana credentials.
type Credentials struct {
	ArgoCDUsername  string `json:"argocdUsername"`
	ArgoCDPassword  string `json:"argocdPassword"`
	GrafanaUsername string `json:"grafanaUsername"`
	GrafanaPassword string `json:"grafanaPassword"`
}

// FetchCredentials retrieves ArgoCD and Grafana credentials from the Kubernetes cluster.
func FetchCredentials(clientset kubernetes.Interface) (map[string]interface{}, error) {
	credentials := Credentials{
		ArgoCDUsername:  "admin",
		GrafanaUsername: "admin",
	}

	// Fetch ArgoCD password
	argoSecret, err := clientset.CoreV1().Secrets("argocd").Get(context.Background(), "argocd-secret", metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get ArgoCD secret: %v", err)
		}
	} else {
		argoCDPasswordEncoded := argoSecret.Data["admin.password"]
		argoCDPassword, err := base64.StdEncoding.DecodeString(string(argoCDPasswordEncoded))
		if err != nil {
			return nil, fmt.Errorf("failed to decode ArgoCD password: %v", err)
		}
		credentials.ArgoCDPassword = string(argoCDPassword)
	}

	// Fetch Grafana password
	grafanaSecret, err := clientset.CoreV1().Secrets("monitoring").Get(context.Background(), "grafana", metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get Grafana secret: %v", err)
		}
	} else {
		grafanaPasswordEncoded := grafanaSecret.Data["admin-password"]
		grafanaPassword, err := base64.StdEncoding.DecodeString(string(grafanaPasswordEncoded))
		if err != nil {
			return nil, fmt.Errorf("failed to decode Grafana password: %v", err)
		}
		credentials.GrafanaPassword = string(grafanaPassword)
	}

	// Return the credentials as a map with object structure for usernames and passwords
	credentialsMap := map[string]interface{}{
		"argocdUsername": map[string]interface{}{
			"value": credentials.ArgoCDUsername,
		},
		"argocdPassword": map[string]interface{}{
			"value": credentials.ArgoCDPassword,
		},
		"grafanaUsername": map[string]interface{}{
			"value": credentials.GrafanaUsername,
		},
		"grafanaPassword": map[string]interface{}{
			"value": credentials.GrafanaPassword,
		},
	}

	return credentialsMap, nil
}
