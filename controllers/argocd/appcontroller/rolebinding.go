package appcontroller

import (
	"reflect"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (acr *AppControllerReconciler) reconcileRoleBindings() error {
	var reconcileErrs util.MultiError

	if acr.ClusterScoped {
		// delete namespaced RBAC
		err := acr.deleteRoleBinding(resourceName, acr.Instance.Namespace)
		reconcileErrs.Append(err)

		_, rbs, err := acr.getManagedRBACToBeDeleted()
		if err != nil {
			acr.Logger.Error(err, "failed to retrieve one or more namespaced rbac resources to be deleted")
		} else if len(rbs) > 0 {
			acr.Logger.Debug("reconcileRoleBindings: namespace scoped instance detected; deleting app management rbac resources")
			reconcileErrs.Append(acr.DeleteRoleBindings(rbs))
		}
	} else {
		err := acr.reconcileRB()
		reconcileErrs.Append(err)

		err = acr.reconcileManagedNsRB()
		reconcileErrs.Append(err)
	}

	return reconcileErrs.ErrOrNil()
}

func (acr *AppControllerReconciler) reconcileRB() error {
	req := permissions.RoleBindingRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, acr.Instance.Namespace, acr.Instance.Name, acr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      resourceName,
				Namespace: acr.Instance.Namespace,
			},
		},
	}

	// get custom role name if any
	customRoleName := getCustomRoleName()
	if customRoleName != "" {
		req.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     common.ClusterRoleKind,
			Name:     customRoleName,
		}
	} else {
		req.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     common.RoleKind,
			Name:     resourceName,
		}
	}

	ignoreDrift := false
	updateFn := func(existing, desired *rbacv1.RoleBinding, changed *bool) error {
		// if roleRef differs, we must delete the rolebinding as kubernetes does not allow updation of roleRef
		if !reflect.DeepEqual(existing.RoleRef, desired.RoleRef) {
			acr.Logger.Debug("detected drift in roleRef for rolebinding", "name", existing.Name, "namespace", existing.Namespace)
			if err := acr.deleteRoleBinding(resourceName, acr.Instance.Namespace); err != nil {
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
	return acr.reconRoleBinding(req, argocdcommon.UpdateFnRb(updateFn), ignoreDrift)
}

func (acr *AppControllerReconciler) reconcileManagedNsRB() error {
	var reconcileErrs util.MultiError

	for managedNs := range acr.ManagedNamespaces {
		// Skip namespace if can't be retrieved or in terminating state
		ns, err := cluster.GetNamespace(managedNs, acr.Client)
		if err != nil {
			acr.Logger.Error(err, "reconcileManagedRoles: unable to retrieve namesapce", "name", managedNs)
			continue
		}
		if ns.DeletionTimestamp != nil {
			acr.Logger.Debug("reconcileManagedRoles: skipping namespace in terminating state", "name", managedNs)
			continue
		}

		// Skip control plane namespace
		if managedNs == acr.Instance.Namespace {
			continue
		}

		req := permissions.RoleBindingRequest{
			ObjectMeta: argoutil.GetObjMeta(managedNsResourceName, managedNs, acr.Instance.Name, acr.Instance.Namespace, component, argocdcommon.GetResourceManagementLabel(), util.EmptyMap()),
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      resourceName,
					Namespace: acr.Instance.Namespace,
				},
			},
		}

		// get custom role name if any
		customRoleName := getCustomRoleName()
		if customRoleName != "" {
			req.RoleRef = rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     common.ClusterRoleKind,
				Name:     customRoleName,
			}
		} else {
			req.RoleRef = rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     common.RoleKind,
				Name:     managedNsResourceName,
			}
		}

		ignoreDrift := false
		updateFn := func(existing, desired *rbacv1.RoleBinding, changed *bool) error {
			// if roleRef differs, we must delete the rolebinding as kubernetes does not allow updation of roleRef
			if !reflect.DeepEqual(existing.RoleRef, desired.RoleRef) {
				acr.Logger.Debug("detected drift in roleRef for rolebinding", "name", existing.Name, "namespace", existing.Namespace)
				if err := acr.deleteRoleBinding(resourceName, acr.Instance.Namespace); err != nil {
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
		reconcileErrs.Append(acr.reconRoleBinding(req, argocdcommon.UpdateFnRb(updateFn), ignoreDrift))
	}
	return reconcileErrs.ErrOrNil()
}

func (acr *AppControllerReconciler) reconRoleBinding(req permissions.RoleBindingRequest, updateFn interface{}, ignoreDrift bool) error {
	desired := permissions.RequestRoleBinding(req)

	if desired.Namespace == acr.Instance.Namespace {
		if err := controllerutil.SetControllerReference(acr.Instance, desired, acr.Scheme); err != nil {
			acr.Logger.Error(err, "reconRoleBinding: failed to set owner reference for RoleBinding", "name", desired.Name, "namespace", desired.Namespace)
		}
	}

	existing, err := permissions.GetRoleBinding(desired.Name, desired.Namespace, acr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconRoleBinding: failed to retrieve RoleBinding %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = permissions.CreateRoleBinding(desired, acr.Client); err != nil {
			return errors.Wrapf(err, "reconRoleBinding: failed to create RoleBinding %s in namespace %s", desired.Name, desired.Namespace)
		}
		acr.Logger.Info("role binding created", "name", desired.Name, "namespace", desired.Namespace)
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

	if err = permissions.UpdateRoleBinding(existing, acr.Client); err != nil {
		return errors.Wrapf(err, "reconRoleBinding: failed to update RoleBinding %s", existing.Name)
	}

	acr.Logger.Info("rolebinding updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteRoleBinding deletes a RoleBinding with the given name and namespace.
func (acr *AppControllerReconciler) deleteRoleBinding(name, namespace string) error {
	if err := permissions.DeleteRoleBinding(name, namespace, acr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		acr.Logger.Error(err, "DeleteRoleBinding: failed to delete RoleBinding", "name", name, "namespace", namespace)
		return err
	}
	acr.Logger.Info("RoleBinding deleted", "name", name, "namespace", namespace)
	return nil
}

// DeleteRoleBindings deletes multiple RoleBindings based on the provided list of NamespacedName.
func (acr *AppControllerReconciler) DeleteRoleBindings(roleBindings []types.NamespacedName) error {
	var deletionErr util.MultiError
	for _, roleBinding := range roleBindings {
		deletionErr.Append(acr.deleteRoleBinding(roleBinding.Name, roleBinding.Namespace))
	}
	return deletionErr.ErrOrNil()
}
