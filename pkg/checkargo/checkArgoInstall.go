package checkargo

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"

)


func IsArgoCDInstalledAndReady(logger *zap.SugaredLogger, clientset kubernetes.Interface) (bool, bool, error) {
	_, err := clientset.CoreV1().Namespaces().Get(context.TODO(), "argocd", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, false, nil
		}
		return false, false, err
	}

	// Check for the presence and readiness of ArgoCD components
	deployments := []string{
		"argo-cd-argocd-applicationset-controller",
		"argo-cd-argocd-notifications-controller",
		"argo-cd-argocd-server",
		"argo-cd-argocd-repo-server",
		"argo-cd-argocd-redis",
		"argo-cd-argocd-dex-server",
	}

	for _, deployment := range deployments {
		deploy, err := clientset.AppsV1().Deployments("argocd").Get(context.TODO(), deployment, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				logger.Info("ArgoCD Components not found. Installing...")
				return false, false, nil
			}
			return false, false, err
		}

		// Check if the number of ready replicas matches the desired replicas
		if deploy.Status.ReadyReplicas != *deploy.Spec.Replicas {
			return true, false, nil // Components are installed but not ready
		}
	}

	return true, true, nil // All components are installed and ready
}