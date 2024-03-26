package server

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

func (sr *ServerReconciler) reconcileRoleBindings() error {
	var reconcileErrs util.MultiError

	if sr.ClusterScoped {
		// delete control plane rolebindings
		err := sr.deleteRoleBinding(resourceName, sr.Instance.Namespace)
		reconcileErrs.Append(err)

		// delete managed ns rolebindings
		_, rbs, err := sr.getManagedNsRBAC()
		if err != nil {
			sr.Logger.Error(err, "reconcileRoleBindings: failed to list one or more resource management namespace rbac resources")
		} else if len(rbs) > 0 {
			sr.Logger.Debug("reconcileRoleBindings: namespace scoped instance detected; deleting resource management rbac resources")
			reconcileErrs.Append(sr.DeleteRoleBindings(rbs))
		}

		// reconcile app source ns rolebindings
		err = sr.reconcileSourceNsRB()
		reconcileErrs.Append(err)

		// reconcile appset source ns rolebindings
		err = sr.reconcileAppsetSourceNsRB()
		reconcileErrs.Append(err)

	} else {
		// delete source ns rolebindings
		_, rbs, err := sr.getSourceNsRBAC()
		if err != nil {
			sr.Logger.Error(err, "reconcileRoleBindings: failed to list one or more app management namespace rbac resources")
		} else if len(rbs) > 0 {
			sr.Logger.Debug("reconcileRoleBindings: namespace scoped instance detected; deleting app management rbac resources")
			reconcileErrs.Append(sr.DeleteRoleBindings(rbs))
		}

		// delete appset ns rolebindings
		_, rbs, err = sr.getAppsetSourceNsRBAC()
		if err != nil {
			sr.Logger.Error(err, "reconcileRoleBindings: failed to list one or more appset management namespace rbac resources")
		} else if len(rbs) > 0 {
			sr.Logger.Debug("reconcileRoleBindings: namespace scoped instance detected; deleting appset management rbac resources")
			reconcileErrs.Append(sr.DeleteRoleBindings(rbs))
		}

		// reconcile namespace scoped rolebindings
		err = sr.reconcileRB()
		reconcileErrs.Append(err)

		err = sr.reconcileManagedNsRB()
		reconcileErrs.Append(err)
	}

	return reconcileErrs.ErrOrNil()
}

// reconcileRoleBinding will ensure ArgoCD Server rolebinding is present
func (sr *ServerReconciler) reconcileRB() error {
	req := permissions.RoleBindingRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, sr.Instance.Namespace, sr.Instance.Name, sr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      resourceName,
				Namespace: sr.Instance.Namespace,
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
			sr.Logger.Debug("detected drift in roleRef for rolebinding", "name", existing.Name, "namespace", existing.Namespace)
			if err := sr.deleteRoleBinding(resourceName, sr.Instance.Namespace); err != nil {
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
	return sr.reconRoleBinding(req, argocdcommon.UpdateFnRb(updateFn), ignoreDrift)
}

func (sr *ServerReconciler) reconcileManagedNsRB() error {
	var reconcileErrs util.MultiError

	for managedNs := range sr.ManagedNamespaces {
		// Skip namespace if can't be retrieved or in terminating state
		ns, err := cluster.GetNamespace(managedNs, sr.Client)
		if err != nil {
			sr.Logger.Error(err, "reconcileManagedNsRB: unable to retrieve namesapce", "name", managedNs)
			continue
		}
		if ns.DeletionTimestamp != nil {
			sr.Logger.Debug("reconcileManagedNsRB: skipping namespace in terminating state", "name", managedNs)
			continue
		}

		// Skip control plane namespace
		if managedNs == sr.Instance.Namespace {
			continue
		}

		req := permissions.RoleBindingRequest{
			ObjectMeta: argoutil.GetObjMeta(managedNsResourceName, managedNs, sr.Instance.Name, sr.Instance.Namespace, component, argocdcommon.GetResourceManagementLabel(), util.EmptyMap()),
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      resourceName,
					Namespace: sr.Instance.Namespace,
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
				sr.Logger.Debug("detected drift in roleRef for rolebinding", "name", existing.Name, "namespace", existing.Namespace)
				if err := sr.deleteRoleBinding(resourceName, sr.Instance.Namespace); err != nil {
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
		reconcileErrs.Append(sr.reconRoleBinding(req, argocdcommon.UpdateFnRb(updateFn), ignoreDrift))
	}
	return reconcileErrs.ErrOrNil()
}

func (sr *ServerReconciler) reconcileSourceNsRB() error {
	var reconcileErrs util.MultiError

	for sourceNs := range sr.SourceNamespaces {
		// Skip namespace if can't be retrieved or in terminating state
		ns, err := cluster.GetNamespace(sourceNs, sr.Client)
		if err != nil {
			sr.Logger.Error(err, "reconcileSourceNsRB: unable to retrieve namesapce", "name", sourceNs)
			continue
		}
		if ns.DeletionTimestamp != nil {
			sr.Logger.Debug("reconcileSourceNsRB: skipping namespace in terminating state", "name", sourceNs)
			continue
		}

		req := permissions.RoleBindingRequest{
			ObjectMeta: argoutil.GetObjMeta(sourceNsResourceName, sourceNs, sr.Instance.Name, sr.Instance.Namespace, component, argocdcommon.GetAppManagementLabel(), util.EmptyMap()),
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      resourceName,
					Namespace: sr.Instance.Namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     common.RoleKind,
				Name:     sourceNsResourceName,
			},
		}

		ignoreDrift := false
		updateFn := func(existing, desired *rbacv1.RoleBinding, changed *bool) error {
			// if roleRef differs, we must delete the rolebinding as kubernetes does not allow updation of roleRef
			if !reflect.DeepEqual(existing.RoleRef, desired.RoleRef) {
				sr.Logger.Debug("detected drift in roleRef for rolebinding", "name", existing.Name, "namespace", existing.Namespace)
				if err := sr.deleteRoleBinding(resourceName, sr.Instance.Namespace); err != nil {
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
		reconcileErrs.Append(sr.reconRoleBinding(req, argocdcommon.UpdateFnRb(updateFn), ignoreDrift))
	}

	return reconcileErrs.ErrOrNil()
}

func (sr *ServerReconciler) reconcileAppsetSourceNsRB() error {
	var reconcileErrs util.MultiError

	for appsetSourceNs := range sr.AppsetSourceNamespaces {
		// Skip namespace if can't be retrieved or in terminating state
		ns, err := cluster.GetNamespace(appsetSourceNs, sr.Client)
		if err != nil {
			sr.Logger.Error(err, "reconcileAppsetSourceNsRB: unable to retrieve namesapce", "name", appsetSourceNs)
			continue
		}
		if ns.DeletionTimestamp != nil {
			sr.Logger.Debug("reconcileAppsetSourceNsRB: skipping namespace in terminating state", "name", appsetSourceNs)
			continue
		}

		req := permissions.RoleBindingRequest{
			ObjectMeta: argoutil.GetObjMeta(appsetSourceNsResourceName, appsetSourceNs, sr.Instance.Name, sr.Instance.Namespace, component, argocdcommon.GetAppsetManagementLabel(), util.EmptyMap()),
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      resourceName,
					Namespace: sr.Instance.Namespace,
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
				sr.Logger.Debug("detected drift in roleRef for rolebinding", "name", existing.Name, "namespace", existing.Namespace)
				if err := sr.deleteRoleBinding(resourceName, sr.Instance.Namespace); err != nil {
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
		reconcileErrs.Append(sr.reconRoleBinding(req, argocdcommon.UpdateFnRb(updateFn), ignoreDrift))
	}

	return reconcileErrs.ErrOrNil()
}

func (sr *ServerReconciler) reconRoleBinding(req permissions.RoleBindingRequest, updateFn interface{}, ignoreDrift bool) error {
	desired := permissions.RequestRoleBinding(req)

	if desired.Namespace == sr.Instance.Namespace {
		if err := controllerutil.SetControllerReference(sr.Instance, desired, sr.Scheme); err != nil {
			sr.Logger.Error(err, "reconRoleBinding: failed to set owner reference for RoleBinding", "name", desired.Name, "namespace", desired.Namespace)
		}
	}

	existing, err := permissions.GetRoleBinding(desired.Name, desired.Namespace, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconRoleBinding: failed to retrieve RoleBinding %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = permissions.CreateRoleBinding(desired, sr.Client); err != nil {
			return errors.Wrapf(err, "reconRoleBinding: failed to create RoleBinding %s in namespace %s", desired.Name, desired.Namespace)
		}
		sr.Logger.Info("role binding created", "name", desired.Name, "namespace", desired.Namespace)
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

	if err = permissions.UpdateRoleBinding(existing, sr.Client); err != nil {
		return errors.Wrapf(err, "reconRoleBinding: failed to update RoleBinding %s", existing.Name)
	}

	sr.Logger.Info("rolebinding updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteRoleBinding will delete rolebinding with given name.
func (sr *ServerReconciler) deleteRoleBinding(name, namespace string) error {
	if err := permissions.DeleteRoleBinding(name, namespace, sr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteRoleBinding: failed to delete role %s in namespace %s", name, namespace)
	}
	sr.Logger.Info("roleBinding deleted", "name", name, "namespace", namespace)
	return nil
}

// DeleteRoleBindings deletes multiple RoleBindings based on the provided list of NamespacedName.
func (sr *ServerReconciler) DeleteRoleBindings(roleBindings []types.NamespacedName) error {
	var deletionErr util.MultiError
	for _, roleBinding := range roleBindings {
		deletionErr.Append(sr.deleteRoleBinding(roleBinding.Name, roleBinding.Namespace))
	}
	return deletionErr.ErrOrNil()
}
