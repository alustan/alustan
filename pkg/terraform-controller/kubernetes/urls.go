package kubernetes

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	
)


// GetAllIngressURLs retrieves URLs of all Ingress resources in all namespaces.
func GetAllIngressURLs(clientset *kubernetes.Clientset) (map[string][]string, error) {
	ingressURLs := make(map[string][]string)

	namespaces, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %v", err)
	}

	for _, namespace := range namespaces.Items {
		ingresses, err := clientset.NetworkingV1().Ingresses(namespace.Name).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list Ingress resources in namespace %s: %v", namespace.Name, err)
		}

		for _, ingress := range ingresses.Items {
			if len(ingress.Spec.Rules) > 0 {
				ingressURL := fmt.Sprintf("https://%s", ingress.Spec.Rules[0].Host)
				ingressURLs[namespace.Name] = append(ingressURLs[namespace.Name], ingressURL)
			}
		}
	}

	return ingressURLs, nil
}
