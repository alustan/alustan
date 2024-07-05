package containers

import (
	"context"
	
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)



// CheckExistingJobs checks if there are existing jobs with the specified label selector.
func CheckExistingJobs(clientset kubernetes.Interface, namespace, labelSelector string) (bool, string, error) {
    jobs, err := clientset.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{
        LabelSelector: labelSelector,
    })
    if err != nil {
        return false, "", err
    }

    if len(jobs.Items) > 0 {
        return true, jobs.Items[0].Name, nil
    }
    return false, "", nil
}