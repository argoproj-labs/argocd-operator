package applicationset

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (asr *ApplicationSetReconciler) reconcileRoleBinding() error {

	asr.Logger.Info("reconciling roleBinding")

	sa, err := permissions.GetServiceAccount(resourceName, asr.Instance.Namespace, asr.Client)

	if err != nil {
		asr.Logger.Error(err, "reconcileRoleBinding: failed to get serviceaccount", "name", resourceName, "namespace", asr.Instance.Namespace)
		return err
	}

	roleBindingRequest := permissions.RoleBindingRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        resourceName,
			Namespace:   asr.Instance.Namespace,
			Labels:      resourceLabels,
			Annotations: asr.Instance.Annotations,
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

	namespace, err := cluster.GetNamespace(asr.Instance.Namespace, asr.Client)
	if err != nil {
		asr.Logger.Error(err, "reconcileRole: failed to retrieve namespace", "name", asr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := asr.deleteRole(desiredRoleBinding.Name, desiredRoleBinding.Namespace); err != nil {
			asr.Logger.Error(err, "reconcileRoleBinding: failed to delete roleBinding", "name", desiredRoleBinding.Name, "namespace", desiredRoleBinding.Namespace)
		}
		return err
	}

	existingRoleBinding, err := permissions.GetRoleBinding(desiredRoleBinding.Name, desiredRoleBinding.Namespace, asr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			asr.Logger.Error(err, "reconcileRoleBinding: failed to retrieve roleBinding", "name", desiredRoleBinding.Name, "namespace", desiredRoleBinding.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(asr.Instance, desiredRoleBinding, asr.Scheme); err != nil {
			asr.Logger.Error(err, "reconcileRole: failed to set owner reference for role", "name", desiredRoleBinding.Name, "namespace", desiredRoleBinding.Namespace)
		}

		if err = permissions.CreateRoleBinding(desiredRoleBinding, asr.Client); err != nil {
			asr.Logger.Error(err, "reconcileRoleBinding: failed to create roleBinding", "name", desiredRoleBinding.Name, "namespace", desiredRoleBinding.Namespace)
			return err
		}
		asr.Logger.V(0).Info("reconcileRoleBinding: roleBinding created", "name", desiredRoleBinding.Name, "namespace", desiredRoleBinding.Namespace)
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
		if err = permissions.UpdateRoleBinding(existingRoleBinding, asr.Client); err != nil {
			asr.Logger.Error(err, "reconcileRoleBinding: failed to update roleBinding", "name", existingRoleBinding.Name, "namespace", existingRoleBinding.Namespace)
			return err
		}
	}

	asr.Logger.V(0).Info("reconcileRoleBinding: roleBinding updated", "name", existingRoleBinding.Name, "namespace", existingRoleBinding.Namespace)

	return nil
}

func (asr *ApplicationSetReconciler) deleteRoleBinding(name, namespace string) error {
	if err := permissions.DeleteRoleBinding(name, namespace, asr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		asr.Logger.Error(err, "DeleteRole: failed to delete roleBinding", "name", name, "namespace", namespace)
		return err
	}
	asr.Logger.V(0).Info("DeleteRoleBinding: roleBinding deleted", "name", name, "namespace", namespace)
	return nil
}
