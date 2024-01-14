package redis

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (rr *RedisReconciler) reconcileRoleBinding() error {
	rbReq := permissions.RoleBindingRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component),
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     common.RoleKind,
			Name:     resourceName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      resourceName,
				Namespace: rr.Instance.Namespace,
			},
		},
	}

	desiredRb := permissions.RequestRoleBinding(rbReq)

	if err := controllerutil.SetControllerReference(rr.Instance, desiredRb, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileRoleBinding: failed to set owner reference for role", "name", desiredRb.Name)
	}

	existingRb, err := permissions.GetRoleBinding(desiredRb.Name, desiredRb.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileRoleBinding: failed to retrieve role %s", desiredRb.Name)
		}

		if err = permissions.CreateRoleBinding(desiredRb, rr.Client); err != nil {
			return errors.Wrapf(err, "reconcileRoleBinding: failed to create role %s", desiredRb.Name)
		}
		rr.Logger.V(0).Info("rolebinding created", "name", desiredRb.Name, "namespace", desiredRb.Namespace)
		return nil
	}

	rbChanged := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{existingRb.RoleRef, existingRb.RoleRef, nil},
		{existingRb.Subjects, existingRb.Subjects, nil},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &rbChanged)
	}

	if !rbChanged {
		return nil
	}

	if err = permissions.UpdateRoleBinding(existingRb, rr.Client); err != nil {
		return errors.Wrapf(err, "reconcileRoleBinding: failed to update role %s", existingRb.Name)
	}

	rr.Logger.V(0).Info("rolebinding updated", "name", existingRb.Name, "namespace", existingRb.Namespace)
	return nil
}

func (rr *RedisReconciler) deleteRoleBinding(name, namespace string) error {
	if err := permissions.DeleteRoleBinding(name, namespace, rr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteRoleBinding: failed to delete rolebinding %s", name)
	}
	rr.Logger.V(0).Info("DeleteRoleBinding: roleBinding deleted", "name", name, "namespace", namespace)
	return nil
}
