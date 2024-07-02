package containers

import (
	"context"
	
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)


// CheckExistinPods checks for any existing pods with the specified label.
func CheckExistingPods(clientset  kubernetes.Interface, namespace, labelSelector string) (bool, error) {
    pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
        LabelSelector: labelSelector,
    })
    if err != nil {
        return false, err
    }

    // If any pod with the specified label exists, return true
    if len(pods.Items) > 0 {
        return true, nil
    }

    return false, nil
}
