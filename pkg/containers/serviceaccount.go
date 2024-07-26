package containers

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// CreateOrUpdateServiceAccountAndRoles creates or updates a namespace, ServiceAccount, ClusterRole, and ClusterRoleBinding for the specified namespace.
// It returns the ServiceAccount name and any error encountered.
func CreateOrUpdateServiceAccountAndRoles(logger *zap.SugaredLogger, clientset kubernetes.Interface, name string, namespace string) (string, error) {
	saIdentifier := fmt.Sprintf("terraform-%s", name)
	roleIdentifier := fmt.Sprintf("terraform-role-%s", name)

	// Define Namespace
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}

	// Create Namespace if it doesn't exist
	_, err := clientset.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		logger.Infof("Failed to create Namespace: %v", err)
		return "", err
	}

	logger.Infof("Namespace %s created or already exists.", namespace)

	// Get annotations from existing ServiceAccount in the alustan namespace
	alustanSAName := "terraform-sa"
	alustanSANamespace := "alustan"
	alustanSA, err := clientset.CoreV1().ServiceAccounts(alustanSANamespace).Get(context.Background(), alustanSAName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Infof("Failed to get Service Account from namespace 'alustan': %v", err)
		return "", err
	}
	var annotations map[string]string
	if alustanSA != nil {
		annotations = alustanSA.Annotations
	}

	// Define Service Account
	sa := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        saIdentifier,
			Namespace:   namespace,
			Annotations: annotations,
		},
	}

	// Create or Update Service Account
	_, err = clientset.CoreV1().ServiceAccounts(namespace).Create(context.Background(), sa, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		logger.Infof("Failed to create Service Account: %v", err)
		return "", err
	}

	logger.Infof("Service Account %s created or already exists in namespace %s.", sa.Name, namespace)

	// Define ClusterRole with expanded permissions
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleIdentifier,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
			{
				NonResourceURLs: []string{"*"},
				Verbs:           []string{"*"},
			},
		},
	}

	// Create or Update ClusterRole
	_, err = clientset.RbacV1().ClusterRoles().Create(context.Background(), cr, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		logger.Infof("Failed to create ClusterRole: %v", err)
		return "", err
	}

	logger.Infof("ClusterRole %s created or already exists.", roleIdentifier)

	// Define ClusterRoleBinding
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-binding", roleIdentifier),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleIdentifier,
		},
	}

	// Create or Update ClusterRoleBinding
	_, err = clientset.RbacV1().ClusterRoleBindings().Create(context.Background(), crb, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		logger.Infof("Failed to create ClusterRoleBinding: %v", err)
		return "", err
	}

	logger.Infof("ClusterRoleBinding %s created or already exists.", roleIdentifier)

	// Return the ServiceAccount name
	return sa.Name, nil
}
