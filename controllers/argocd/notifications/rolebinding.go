package notifications

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

func (nr *NotificationsReconciler) reconcileRoleBinding() error {
	req := permissions.RoleBindingRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, nr.Instance.Namespace, nr.Instance.Name, nr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     common.RoleKind,
			Name:     resourceName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      resourceName,
				Namespace: nr.Instance.Namespace,
			},
		},
	}

	ignoreDrift := false
	updateFn := func(existing, desired *rbacv1.RoleBinding, changed *bool) error {
		// if roleRef differs, we must delete the rolebinding as kubernetes does not allow updation of roleRef
		if !reflect.DeepEqual(existing.RoleRef, desired.RoleRef) {
			nr.Logger.Debug("detected drift in roleRef for rolebinding", "name", existing.Name, "namespace", existing.Namespace)
			if err := nr.deleteRoleBinding(resourceName, nr.Instance.Namespace); err != nil {
				return errors.Wrapf(err, "unable to delete obsolete rolebinding %s", existing.Name)
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
	return nr.reconRoleBinding(req, argocdcommon.UpdateFnRb(updateFn), ignoreDrift)
}

func (nr *NotificationsReconciler) reconRoleBinding(req permissions.RoleBindingRequest, updateFn interface{}, ignoreDrift bool) error {
	desired := permissions.RequestRoleBinding(req)

	if desired.Namespace == nr.Instance.Namespace {
		if err := controllerutil.SetControllerReference(nr.Instance, desired, nr.Scheme); err != nil {
			nr.Logger.Error(err, "reconRoleBinding: failed to set owner reference for RoleBinding", "name", desired.Name, "namespace", desired.Namespace)
		}
	}

	existing, err := permissions.GetRoleBinding(desired.Name, desired.Namespace, nr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconRoleBinding: failed to retrieve RoleBinding %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = permissions.CreateRoleBinding(desired, nr.Client); err != nil {
			return errors.Wrapf(err, "reconRoleBinding: failed to create RoleBinding %s in namespace %s", desired.Name, desired.Namespace)
		}
		nr.Logger.Info("role binding created", "name", desired.Name, "namespace", desired.Namespace)
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

	if err = permissions.UpdateRoleBinding(existing, nr.Client); err != nil {
		return errors.Wrapf(err, "reconRoleBinding: failed to update RoleBinding %s", existing.Name)
	}

	nr.Logger.Info("rolebinding updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteRoleBinding will delete a RoleBinding with the given name and namespace.
func (nr *NotificationsReconciler) deleteRoleBinding(name, namespace string) error {
	if err := permissions.DeleteRoleBinding(name, namespace, nr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteRoleBinding: failed to delete RoleBinding %s in namespace %s", name, namespace)
	}
	nr.Logger.Info("RoleBinding deleted", "name", name, "namespace", namespace)
	return nil
}
