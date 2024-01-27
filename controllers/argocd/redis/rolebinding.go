package redis

import (
	"reflect"

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
			Name:     rr.getRoleRefName(),
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
		rr.Logger.Info("rolebinding created", "name", desiredRb.Name, "namespace", desiredRb.Namespace)
		return nil
	}

	// if roleRef differs, we must delete the rolebinding as kubernetes does not allow updation of roleRef
	if !reflect.DeepEqual(existingRb.RoleRef, desiredRb.RoleRef) {
		rr.Logger.Info("detected drift in roleRef for rolebinding", "name", existingRb.Name, "namespace", existingRb.Namespace)
		if err := rr.deleteRoleBinding(resourceName, rr.Instance.Namespace); err != nil {
			return errors.Wrapf(err, "reconcileRoleBinding: unable to delete obsolete rolebinding %s", existingRb.Name)
		}
		return nil
	}

	rbChanged := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{&existingRb.Subjects, &desiredRb.Subjects, nil},
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

	rr.Logger.Info("rolebinding updated", "name", existingRb.Name, "namespace", existingRb.Namespace)
	return nil
}

func (rr *RedisReconciler) deleteRoleBinding(name, namespace string) error {
	if err := permissions.DeleteRoleBinding(name, namespace, rr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteRoleBinding: failed to delete rolebinding %s", name)
	}
	rr.Logger.Info("roleBinding deleted", "name", name, "namespace", namespace)
	return nil
}

func (rr *RedisReconciler) getRoleRefName() string {
	if rr.Instance.Spec.HA.Enabled {
		return HAResourceName
	}
	return resourceName
}
