package containers


import (
    "context"
    "log"
  
   v1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
   "k8s.io/apimachinery/pkg/api/resource"
    "k8s.io/client-go/kubernetes"
)

// EnsurePVC ensures that the specified Persistent Volume Claim exists.
func EnsurePVC(clientset  kubernetes.Interface, namespace, pvcName string) error {
    pvc, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), pvcName, metav1.GetOptions{})
    if err == nil && pvc != nil {
        log.Printf("PVC %s already exists in namespace %s", pvcName, namespace)
        return nil
    }

    log.Printf("Creating PVC %s in namespace %s", pvcName, namespace)
    pvc = &v1.PersistentVolumeClaim{
        ObjectMeta: metav1.ObjectMeta{
            Name: pvcName,
        },
        Spec: v1.PersistentVolumeClaimSpec{
            AccessModes: []v1.PersistentVolumeAccessMode{
                v1.ReadWriteOnce,
            },
            Resources: v1.ResourceRequirements{
                Requests: v1.ResourceList{
                    v1.ResourceStorage: resource.MustParse("5Gi"),
                },
            },
        },
    }

    _, err = clientset.CoreV1().PersistentVolumeClaims(namespace).Create(context.Background(), pvc, metav1.CreateOptions{})
    if err != nil {
        log.Printf("Failed to create PVC: %v", err)
        return err
    }

    log.Println("PVC created successfully.")
    return nil
}