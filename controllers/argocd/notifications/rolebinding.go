package notifications

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (nr *NotificationsReconciler) reconcileRoleBinding() error {

	nr.Logger.Info("reconciling roleBinding")

	sa, err := permissions.GetServiceAccount(resourceName, nr.Instance.Namespace, nr.Client)

	if err != nil {
		nr.Logger.Error(err, "reconcileRoleBinding: failed to get serviceaccount", "name", resourceName, "namespace", nr.Instance.Namespace)
		return err
	}

	roleBindingRequest := permissions.RoleBindingRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        resourceName,
			Namespace:   nr.Instance.Namespace,
			Labels:      resourceLabels,
			Annotations: nr.Instance.Annotations,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     common.RoleKind,
			Name:     resourceName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
	}

	desiredRoleBinding := permissions.RequestRoleBinding(roleBindingRequest)

	namespace, err := cluster.GetNamespace(nr.Instance.Namespace, nr.Client)
	if err != nil {
		nr.Logger.Error(err, "reconcileRole: failed to retrieve namespace", "name", nr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := nr.deleteRole(desiredRoleBinding.Name, desiredRoleBinding.Namespace); err != nil {
			nr.Logger.Error(err, "reconcileRoleBinding: failed to delete roleBinding", "name", desiredRoleBinding.Name, "namespace", desiredRoleBinding.Namespace)
		}
		return err
	}

	existingRoleBinding, err := permissions.GetRoleBinding(desiredRoleBinding.Name, desiredRoleBinding.Namespace, nr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			nr.Logger.Error(err, "reconcileRoleBinding: failed to retrieve roleBinding", "name", desiredRoleBinding.Name, "namespace", desiredRoleBinding.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(nr.Instance, desiredRoleBinding, nr.Scheme); err != nil {
			nr.Logger.Error(err, "reconcileRole: failed to set owner reference for role", "name", desiredRoleBinding.Name, "namespace", desiredRoleBinding.Namespace)
		}

		if err = permissions.CreateRoleBinding(desiredRoleBinding, nr.Client); err != nil {
			nr.Logger.Error(err, "reconcileRoleBinding: failed to create roleBinding", "name", desiredRoleBinding.Name, "namespace", desiredRoleBinding.Namespace)
			return err
		}
		nr.Logger.Info("reconcileRoleBinding: roleBinding created", "name", desiredRoleBinding.Name, "namespace", desiredRoleBinding.Namespace)
		return nil
	}

	roleBindingChanged := false
	fieldsToCompare := []struct {
		existing, desired interface{}
	}{
		{
			&existingRoleBinding.RoleRef,
			&desiredRoleBinding.RoleRef,
		},
		{
			&existingRoleBinding.Subjects,
			&desiredRoleBinding.Subjects,
		},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, nil, &roleBindingChanged)
	}

	if roleBindingChanged {
		if err = permissions.UpdateRoleBinding(existingRoleBinding, nr.Client); err != nil {
			nr.Logger.Error(err, "reconcileRoleBinding: failed to update roleBinding", "name", existingRoleBinding.Name, "namespace", existingRoleBinding.Namespace)
			return err
		}
	}

	nr.Logger.Info("reconcileRoleBinding: roleBinding updated", "name", existingRoleBinding.Name, "namespace", existingRoleBinding.Namespace)

	return nil
}

func (nr *NotificationsReconciler) deleteRoleBinding(name, namespace string) error {
	if err := permissions.DeleteRoleBinding(name, namespace, nr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		nr.Logger.Error(err, "DeleteRole: failed to delete roleBinding", "name", name, "namespace", namespace)
		return err
	}
	nr.Logger.Info("DeleteRoleBinding: roleBinding deleted", "name", name, "namespace", namespace)
	return nil
}
