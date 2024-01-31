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
	req := permissions.RoleBindingRequest{
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

	desired := permissions.RequestRoleBinding(req)

	if err := controllerutil.SetControllerReference(rr.Instance, desired, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileRoleBinding: failed to set owner reference for role", "name", desired.Name)
	}

	existing, err := permissions.GetRoleBinding(desired.Name, desired.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileRoleBinding: failed to retrieve role %s", desired.Name)
		}

		if err = permissions.CreateRoleBinding(desired, rr.Client); err != nil {
			return errors.Wrapf(err, "reconcileRoleBinding: failed to create role %s", desired.Name)
		}
		rr.Logger.Info("rolebinding created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// if roleRef differs, we must delete the rolebinding as kubernetes does not allow updation of roleRef
	if !reflect.DeepEqual(existing.RoleRef, desired.RoleRef) {
		rr.Logger.Info("detected drift in roleRef for rolebinding", "name", existing.Name, "namespace", existing.Namespace)
		if err := rr.deleteRoleBinding(resourceName, rr.Instance.Namespace); err != nil {
			return errors.Wrapf(err, "reconcileRoleBinding: unable to delete obsolete rolebinding %s", existing.Name)
		}
		return nil
	}

	changed := false

	fieldsToCompare := []argocdcommon.FieldToCompare{
		{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
		{Existing: &existing.Annotations, Desired: &desired.Annotations, ExtraAction: nil},
		{Existing: &existing.Subjects, Desired: &desired.Subjects, ExtraAction: nil},
	}

	argocdcommon.UpdateIfChanged(fieldsToCompare, &changed)

	if !changed {
		return nil
	}

	if err = permissions.UpdateRoleBinding(existing, rr.Client); err != nil {
		return errors.Wrapf(err, "reconcileRoleBinding: failed to update role %s", existing.Name)
	}

	rr.Logger.Info("rolebinding updated", "name", existing.Name, "namespace", existing.Namespace)
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
