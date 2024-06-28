package kubernetes

import (
	"context"
	"encoding/base64"
	"fmt"
	
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	
)

type Credentials struct {
	ArgoCDUsername  string `json:"argocdUsername"`
	ArgoCDPassword  string `json:"argocdPassword"`
	GrafanaUsername string `json:"grafanaUsername"`
	GrafanaPassword string `json:"grafanaPassword"`
}

func FetchCredentials(clientset *kubernetes.Clientset) (Credentials, error) {
	// Fetch ArgoCD password
	argoSecret, err := clientset.CoreV1().Secrets("argocd").Get(context.Background(), "argocd-secret", metav1.GetOptions{})
	if err != nil {
		return Credentials{}, fmt.Errorf("failed to get ArgoCD secret: %v", err)
	}
	argoCDPasswordEncoded := argoSecret.Data["admin.password"]
	argoCDPassword, err := base64.StdEncoding.DecodeString(string(argoCDPasswordEncoded))
	if err != nil {
		return Credentials{}, fmt.Errorf("failed to decode ArgoCD password: %v", err)
	}

	// Fetch Grafana password
	grafanaSecret, err := clientset.CoreV1().Secrets("monitoring").Get(context.Background(), "grafana", metav1.GetOptions{})
	if err != nil {
		return Credentials{}, fmt.Errorf("failed to get Grafana secret: %v", err)
	}
	grafanaPasswordEncoded := grafanaSecret.Data["admin-password"]
	grafanaPassword, err := base64.StdEncoding.DecodeString(string(grafanaPasswordEncoded))
	if err != nil {
		return Credentials{}, fmt.Errorf("failed to decode Grafana password: %v", err)
	}

	// Return the credentials
	return Credentials{
		ArgoCDUsername:  "admin",
		ArgoCDPassword:  string(argoCDPassword),
		GrafanaUsername: "admin",
		GrafanaPassword: string(grafanaPassword),
	}, nil
}