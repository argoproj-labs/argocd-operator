package applicationset

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

func (asr *ApplicationSetReconciler) reconcileRolebindings() error {
	var reconcileErrs util.MultiError

	if asr.ClusterScoped {
		// delete control plane rolebinding
		err := asr.deleteRoleBinding(resourceName, asr.Instance.Namespace)
		reconcileErrs.Append(err)

		// reconcile appset source ns rolebindings
		err = asr.reconcileAppsetSourceNsRB()
		reconcileErrs.Append(err)

	} else {
		// delete source ns rolebindings
		_, rbs, err := asr.getAppsetSourceNsRBAC()
		if err != nil {
			asr.Logger.Error(err, "reconcileRoles: failed to list one or more appset management namespace rbac resources")
		} else if len(rbs) > 0 {
			asr.Logger.Debug("reconcileRoles: namespace scoped instance detected; deleting appset management rbac resources")
			reconcileErrs.Append(asr.DeleteRoles(rbs))
		}

		// reconcile control plane rolebinding
		err = asr.reconcileRB()
		reconcileErrs.Append(err)
	}
	return reconcileErrs.ErrOrNil()
}

// reconcileRoleBinding will ensure ArgoCD appset rolebinding is present
func (asr *ApplicationSetReconciler) reconcileRB() error {
	req := permissions.RoleBindingRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, asr.Instance.Namespace, asr.Instance.Name, asr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      resourceName,
				Namespace: asr.Instance.Namespace,
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
			asr.Logger.Debug("detected drift in roleRef for rolebinding", "name", existing.Name, "namespace", existing.Namespace)
			if err := asr.deleteRoleBinding(resourceName, asr.Instance.Namespace); err != nil {
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
	return asr.reconRoleBinding(req, argocdcommon.UpdateFnRb(updateFn), ignoreDrift)
}

func (asr *ApplicationSetReconciler) reconcileAppsetSourceNsRB() error {
	var reconcileErrs util.MultiError

	for sourceNs := range asr.AppsetSourceNamespaces {
		// Skip namespace if can't be retrieved or in terminating state
		ns, err := cluster.GetNamespace(sourceNs, asr.Client)
		if err != nil {
			asr.Logger.Error(err, "reconcileManagedRoles: unable to retrieve namesapce", "name", sourceNs)
			continue
		}
		if ns.DeletionTimestamp != nil {
			asr.Logger.Debug("reconcileManagedRoles: skipping namespace in terminating state", "name", sourceNs)
			continue
		}

		req := permissions.RoleBindingRequest{
			ObjectMeta: argoutil.GetObjMeta(appsetSourceNsResourceName, sourceNs, asr.Instance.Name, asr.Instance.Namespace, component, argocdcommon.GetAppsetManagementLabel(), util.EmptyMap()),
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      resourceName,
					Namespace: asr.Instance.Namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     common.RoleKind,
				Name:     appsetSourceNsResourceName,
			},
		}

		ignoreDrift := false
		updateFn := func(existing, desired *rbacv1.RoleBinding, changed *bool) error {
			// if roleRef differs, we must delete the rolebinding as kubernetes does not allow updation of roleRef
			if !reflect.DeepEqual(existing.RoleRef, desired.RoleRef) {
				asr.Logger.Debug("detected drift in roleRef for rolebinding", "name", existing.Name, "namespace", existing.Namespace)
				if err := asr.deleteRoleBinding(resourceName, asr.Instance.Namespace); err != nil {
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
		reconcileErrs.Append(asr.reconRoleBinding(req, argocdcommon.UpdateFnRb(updateFn), ignoreDrift))
	}

	return reconcileErrs.ErrOrNil()
}

func (asr *ApplicationSetReconciler) reconRoleBinding(req permissions.RoleBindingRequest, updateFn interface{}, ignoreDrift bool) error {
	desired := permissions.RequestRoleBinding(req)

	if desired.Namespace == asr.Instance.Namespace {
		if err := controllerutil.SetControllerReference(asr.Instance, desired, asr.Scheme); err != nil {
			asr.Logger.Error(err, "reconRoleBinding: failed to set owner reference for RoleBinding", "name", desired.Name, "namespace", desired.Namespace)
		}
	}

	existing, err := permissions.GetRoleBinding(desired.Name, desired.Namespace, asr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconRoleBinding: failed to retrieve RoleBinding %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = permissions.CreateRoleBinding(desired, asr.Client); err != nil {
			return errors.Wrapf(err, "reconRoleBinding: failed to create RoleBinding %s in namespace %s", desired.Name, desired.Namespace)
		}
		asr.Logger.Info("role binding created", "name", desired.Name, "namespace", desired.Namespace)
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

	if err = permissions.UpdateRoleBinding(existing, asr.Client); err != nil {
		return errors.Wrapf(err, "reconRoleBinding: failed to update RoleBinding %s", existing.Name)
	}

	asr.Logger.Info("rolebinding updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteRoleBinding will delete rolebinding with given name.
func (asr *ApplicationSetReconciler) deleteRoleBinding(name, namespace string) error {
	if err := permissions.DeleteRoleBinding(name, namespace, asr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteRoleBinding: failed to delete role %s in namespace %s", name, namespace)
	}
	asr.Logger.Info("roleBinding deleted", "name", name, "namespace", namespace)
	return nil
}

// DeleteRoleBindings deletes multiple RoleBindings based on the provided list of NamespacedName.
func (asr *ApplicationSetReconciler) DeleteRoleBindings(roleBindings []types.NamespacedName) error {
	var deletionErr util.MultiError
	for _, roleBinding := range roleBindings {
		deletionErr.Append(asr.deleteRoleBinding(roleBinding.Name, roleBinding.Namespace))
	}
	return deletionErr.ErrOrNil()
}
