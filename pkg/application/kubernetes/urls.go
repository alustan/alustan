package kubernetes

import (
	"strings"
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetAllIngressURLs retrieves URLs of all Ingress resources in namespaces with the prefix "preview".
func GetAllIngressURLs(clientset kubernetes.Interface) (map[string]interface{}, error) {
	ingressURLs := make(map[string]interface{})

	namespaces, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %v", err)
	}

	for _, namespace := range namespaces.Items {
		if strings.HasPrefix(namespace.Name, "preview") {
			ingresses, err := clientset.NetworkingV1().Ingresses(namespace.Name).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to list Ingress resources in namespace %s: %v", namespace.Name, err)
			}

			var namespaceIngressURLs []interface{}

			for _, ingress := range ingresses.Items {
				for _, rule := range ingress.Spec.Rules {
					if rule.Host != "" {
						ingressURL := fmt.Sprintf("https://%s", rule.Host)
						namespaceIngressURLs = append(namespaceIngressURLs, ingressURL)
					}
				}
			}

			ingressURLs[namespace.Name] = namespaceIngressURLs
		}
	}

	return ingressURLs, nil
}

