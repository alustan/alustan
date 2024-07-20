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

// CreateOrUpdateServiceAccountAndRoles creates or updates a namespace, ServiceAccounts in both namespaces, ClusterRole, and ClusterRoleBindings.
// It returns the ServiceAccount name and any error encountered.
func CreateOrUpdateServiceAccountAndRoles(logger *zap.SugaredLogger, clientset kubernetes.Interface, name string, namespace string) (string, error) {

	// Define Namespace
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "argocd",
		},
	}

	// Create Namespace if it doesn't exist
	_, err := clientset.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		logger.Infof("Failed to create Namespace: %v", err)
		return "", err
	}

	logger.Infof("Namespace %s created or already exists.", namespace)

	saIdentifier := fmt.Sprintf("terraform-%s", name)
	roleIdentifier := fmt.Sprintf("terraform-role-%s", name)
	roleBindingIdentifier := fmt.Sprintf("terraform-role-binding-%s", name)

	// Function to create or update Service Account
	createOrUpdateServiceAccount := func(namespace string) error {
		sa := &v1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      saIdentifier,
				Namespace: namespace,
			},
		}
		_, err := clientset.CoreV1().ServiceAccounts(namespace).Create(context.Background(), sa, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			logger.Infof("Failed to create Service Account in namespace %s: %v", namespace, err)
			return err
		}
		logger.Infof("Service Account %s created or already exists in namespace %s.", sa.Name, namespace)
		return nil
	}

	// Create or Update Service Account in both namespaces
	if err := createOrUpdateServiceAccount("argocd"); err != nil {
		return "", err
	}
	if err := createOrUpdateServiceAccount(namespace); err != nil {
		return "", err
	}

	// Define ClusterRole with expanded permissions
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleIdentifier,
		},
		Rules: []rbacv1.PolicyRule{
			// API group: "" (core group)
			{
				APIGroups: []string{""},
				Resources: []string{"secrets", "namespaces"},
				Verbs:     []string{"create", "get", "list", "watch", "update", "delete", "patch"},
			},
			// API group: "argoproj.io"
			{
				APIGroups: []string{"argoproj.io"},
				Resources: []string{"applications", "applicationsets", "projects", "repositories"},
				Verbs:     []string{"create", "get", "list", "watch", "update", "delete"},
			},
		},
	}

	// Create or Update ClusterRole
	_, err = clientset.RbacV1().ClusterRoles().Create(context.Background(), cr, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		logger.Infof("Failed to create ClusterRole: %v", err)
		return "", err
	}

	logger.Infof("ClusterRole %s created or already exists.", cr.Name)

	// Function to create or update ClusterRoleBinding
	createOrUpdateClusterRoleBinding := func(namespace string) error {
		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s-%s", roleBindingIdentifier, namespace),
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      saIdentifier,
					Namespace: namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     roleIdentifier,
				APIGroup: "rbac.authorization.k8s.io",
			},
		}
		_, err := clientset.RbacV1().ClusterRoleBindings().Create(context.Background(), crb, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			logger.Infof("Failed to create ClusterRoleBinding in namespace %s: %v", namespace, err)
			return err
		}
		logger.Infof("ClusterRoleBinding %s created or already exists in namespace %s.", crb.Name, namespace)
		return nil
	}

	// Create or Update ClusterRoleBinding in both namespaces
	if err := createOrUpdateClusterRoleBinding("argocd"); err != nil {
		return "", err
	}
	if err := createOrUpdateClusterRoleBinding(namespace); err != nil {
		return "", err
	}

	return saIdentifier, nil
}
