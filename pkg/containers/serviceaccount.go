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

	// Define Service Account
	saIdentifier := fmt.Sprintf("terraform-%s", name)
	sa := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saIdentifier,
			Namespace: namespace,
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
	roleIdentifier := "terraform-manager"
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleIdentifier,
		},
		Rules: []rbacv1.PolicyRule{
			// API group: "" (core group)
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps", "pods", "persistentvolumeclaims", "secrets", "namespaces", "serviceaccounts", "events", "pods/log"},
				Verbs:     []string{"create", "get", "list", "watch", "update", "delete", "patch"},
			},
			// API group: "batch"
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs"},
				Verbs:     []string{"create", "get", "list", "watch", "update", "delete", "patch"},
			},
			// API group: "networking.k8s.io"
			{
				APIGroups: []string{"networking.k8s.io"},
				Resources: []string{"ingresses"},
				Verbs:     []string{"create", "get", "list", "watch", "update", "delete", "patch"},
			},
			// API group: "apps"
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "statefulsets"},
				Verbs:     []string{"create", "get", "list", "watch", "update", "delete"},
			},
			// API group: "rbac.authorization.k8s.io"
			{
				APIGroups: []string{"rbac.authorization.k8s.io"},
				Resources: []string{"roles", "rolebindings", "clusterroles", "clusterrolebindings"},
				Verbs:     []string{"create", "get", "list", "watch", "update", "delete"},
			},
			// API group: "argoproj.io"
			{
				APIGroups: []string{"argoproj.io"},
				Resources: []string{"applications", "applicationsets", "projects", "repositories", "workflows"},
				Verbs:     []string{"create", "get", "list", "watch", "update", "delete", "patch"},
			},
			// API group: "apiextensions.k8s.io"
			{
				APIGroups: []string{"apiextensions.k8s.io"},
				Resources: []string{"customresourcedefinitions"},
				Verbs:     []string{"create", "get", "list", "watch", "update", "delete"},
			},
			// Additional permissions to cover other needs
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"extensions"},
				Resources: []string{"deployments"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"get", "list", "delete", "patch"},
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

	return sa.Name, nil
}
