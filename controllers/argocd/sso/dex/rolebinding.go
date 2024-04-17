package dex

import (
	"reflect"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileRoleBinding will ensure ArgoCD appset rolebinding is present
func (dr *DexReconciler) reconcileRB() error {
	req := permissions.RoleBindingRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, dr.Instance.Namespace, dr.Instance.Name, dr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      resourceName,
				Namespace: dr.Instance.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     common.RoleKind,
			Name:     resourceName,
		},
	}

	ignoreDrift := false
	updateFn := func(existing, desired *rbacv1.RoleBinding, changed *bool) error {
		// if roleRef differs, we must delete the rolebinding as kubernetes does not allow updation of roleRef
		if !reflect.DeepEqual(existing.RoleRef, desired.RoleRef) {
			dr.Logger.Debug("detected drift in roleRef for rolebinding", "name", existing.Name, "namespace", existing.Namespace)
			if err := dr.deleteRoleBinding(resourceName, dr.Instance.Namespace); err != nil {
				return errors.Wrapf(err, "reconcileRoleBinding: unable to delete obsolete rolebinding %s", existing.Name)
			}
			return nil
		}

		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			{Existing: &existing.Subjects, Desired: &desired.Subjects, ExtraAction: nil},
		}

		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}
	return dr.reconRoleBinding(req, argocdcommon.UpdateFnRb(updateFn), ignoreDrift)
}

func (dr *DexReconciler) reconRoleBinding(req permissions.RoleBindingRequest, updateFn interface{}, ignoreDrift bool) error {
	desired := permissions.RequestRoleBinding(req)

	if desired.Namespace == dr.Instance.Namespace {
		if err := controllerutil.SetControllerReference(dr.Instance, desired, dr.Scheme); err != nil {
			dr.Logger.Error(err, "reconRoleBinding: failed to set owner reference for RoleBinding", "name", desired.Name, "namespace", desired.Namespace)
		}
	}

	existing, err := permissions.GetRoleBinding(desired.Name, desired.Namespace, dr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconRoleBinding: failed to retrieve RoleBinding %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = permissions.CreateRoleBinding(desired, dr.Client); err != nil {
			return errors.Wrapf(err, "reconRoleBinding: failed to create RoleBinding %s in namespace %s", desired.Name, desired.Namespace)
		}
		dr.Logger.Info("role binding created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// RoleBinding found, no update required - nothing to do
	if ignoreDrift {
		return nil
	}

	changed := false

	// execute supplied update function
	if updateFn != nil {
		if fn, ok := updateFn.(argocdcommon.UpdateFnRb); ok {
			if err := fn(existing, desired, &changed); err != nil {
				return errors.Wrapf(err, "reconRoleBinding: failed to execute update function for %s in namespace %s", existing.Name, existing.Namespace)
			}
		}
	}

	if !changed {
		return nil
	}

	if err = permissions.UpdateRoleBinding(existing, dr.Client); err != nil {
		return errors.Wrapf(err, "reconRoleBinding: failed to update RoleBinding %s", existing.Name)
	}

	dr.Logger.Info("rolebinding updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteRoleBinding will delete rolebinding with given name.
func (dr *DexReconciler) deleteRoleBinding(name, namespace string) error {
	if err := permissions.DeleteRoleBinding(name, namespace, dr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteRoleBinding: failed to delete role %s in namespace %s", name, namespace)
	}
	dr.Logger.Info("roleBinding deleted", "name", name, "namespace", namespace)
	return nil
}
