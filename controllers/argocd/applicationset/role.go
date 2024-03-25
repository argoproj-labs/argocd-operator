package applicationset

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/pkg/errors"

	rbacv1 "k8s.io/api/rbac/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (asr *ApplicationSetReconciler) reconcileRoles() error {
	var reconcileErrs util.MultiError

	if asr.ClusterScoped {
		// delete control plane role
		err := asr.deleteRole(resourceName, asr.Instance.Namespace)
		reconcileErrs.Append(err)

		// reconcile appset source ns roles
		err = asr.reconcileAppsetSourceNsRoles()
		reconcileErrs.Append(err)

	} else {
		// delete source ns roles
		_, rbs, err := asr.getAppsetSourceNsRBAC()
		if err != nil {
			asr.Logger.Error(err, "reconcileRoles: failed to list one or more appset management namespace rbac resources")
		} else if len(rbs) > 0 {
			asr.Logger.Debug("reconcileRoles: namespace scoped instance detected; deleting appset management rbac resources")
			reconcileErrs.Append(asr.DeleteRoles(rbs))
		}

		// reconcile control plane role
		err = asr.reconcileRole()
		reconcileErrs.Append(err)

	}

	return reconcileErrs.ErrOrNil()
}

// reconcileRole will ensure ArgoCD Server role is present
func (asr *ApplicationSetReconciler) reconcileRole() error {
	req := permissions.RoleRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, asr.Instance.Namespace, asr.Instance.Name, asr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Rules:      getPolicyRules(),
		Instance:   asr.Instance,
		Client:     asr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	ignoreDrift := false
	updateFn := func(existing, desired *rbacv1.Role, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},

			{Existing: &existing.Rules, Desired: &desired.Rules, ExtraAction: nil},
		}

		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}
	return asr.reconRole(req, argocdcommon.UpdateFnRole(updateFn), ignoreDrift)
}

func (asr *ApplicationSetReconciler) reconcileAppsetSourceNsRoles() error {
	var reconcileErrs util.MultiError

	for sourceNs := range asr.AppsetSourceNamespaces {
		// Skip namespace if can't be retrieved or in terminating state
		ns, err := cluster.GetNamespace(sourceNs, asr.Client)
		if err != nil {
			asr.Logger.Error(err, "reconcileAppsetSourceNsRoles: unable to retrieve namesapce", "name", sourceNs)
			continue
		}
		if ns.DeletionTimestamp != nil {
			asr.Logger.Debug("reconcileAppsetSourceNsRoles: skipping namespace in terminating state", "name", sourceNs)
			continue
		}

		// Skip control plane namespace
		if sourceNs == asr.Instance.Namespace {
			continue
		}

		req := permissions.RoleRequest{
			ObjectMeta: argoutil.GetObjMeta(appsetSourceNsResourceName, sourceNs, asr.Instance.Name, asr.Instance.Namespace, component, argocdcommon.GetAppsetManagementLabel(), util.EmptyMap()),
			Rules:      getPolicyRules(),
			Instance:   asr.Instance,
			Client:     asr.Client,
			Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		}

		ignoreDrift := false
		updateFn := func(existing, desired *rbacv1.Role, changed *bool) error {
			fieldsToCompare := []argocdcommon.FieldToCompare{
				{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
				{Existing: &existing.Rules, Desired: &desired.Rules, ExtraAction: nil},
			}

			argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
			return nil
		}
		err = asr.reconRole(req, argocdcommon.UpdateFnRole(updateFn), ignoreDrift)
		reconcileErrs.Append(err)
	}

	return reconcileErrs.ErrOrNil()
}

func (asr *ApplicationSetReconciler) reconRole(req permissions.RoleRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := permissions.RequestRole(req)
	if err != nil {
		asr.Logger.Debug("reconRole: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconRole: failed to request Role %s in namespace %s", desired.Name, desired.Namespace)
	}

	if desired.Namespace == asr.Instance.Namespace {
		if err = controllerutil.SetControllerReference(asr.Instance, desired, asr.Scheme); err != nil {
			asr.Logger.Error(err, "reconRole: failed to set owner reference for Role", "name", desired.Name, "namespace", desired.Namespace)
		}
	}

	existing, err := permissions.GetRole(desired.Name, desired.Namespace, asr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconRole: failed to retrieve Role %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = permissions.CreateRole(desired, asr.Client); err != nil {
			return errors.Wrapf(err, "reconRole: failed to create Role %s in namespace %s", desired.Name, desired.Namespace)
		}
		asr.Logger.Info("role created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// Role found, no update required - nothing to do
	if ignoreDrift {
		return nil
	}

	changed := false

	// execute supplied update function
	if updateFn != nil {
		if fn, ok := updateFn.(argocdcommon.UpdateFnRole); ok {
			if err := fn(existing, desired, &changed); err != nil {
				return errors.Wrapf(err, "reconRole: failed to execute update function for %s in namespace %s", existing.Name, existing.Namespace)
			}
		}
	}

	if !changed {
		return nil
	}

	if err = permissions.UpdateRole(existing, asr.Client); err != nil {
		return errors.Wrapf(err, "reconRole: failed to update Role %s", existing.Name)
	}

	asr.Logger.Info("role updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteRole will delete role with given name.
func (asr *ApplicationSetReconciler) deleteRole(name, namespace string) error {
	if err := permissions.DeleteRole(name, namespace, asr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteRole: failed to delete role %s in namespace %s", name, namespace)
	}
	asr.Logger.Info("role deleted", "name", name, "namespace", namespace)
	return nil
}

func (asr *ApplicationSetReconciler) DeleteRoles(roles []types.NamespacedName) error {
	var deletionErr util.MultiError
	for _, role := range roles {
		deletionErr.Append(asr.deleteRole(role.Name, role.Namespace))
	}
	return deletionErr.ErrOrNil()
}

func getPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		// ApplicationSet
		{
			APIGroups: []string{"argoproj.io"},
			Resources: []string{
				"applications",
				"applicationsets",
				"applicationsets/finalizers",
			},
			Verbs: []string{
				"create",
				"delete",
				"get",
				"list",
				"patch",
				"update",
				"watch",
			},
		},
		// Appprojects
		{
			APIGroups: []string{"argoproj.io"},
			Resources: []string{
				"appprojects",
			},
			Verbs: []string{
				"get",
			},
		},
		// ApplicationSet Status
		{
			APIGroups: []string{"argoproj.io"},
			Resources: []string{
				"applicationsets/status",
			},
			Verbs: []string{
				"get",
				"patch",
				"update",
			},
		},
		// Events
		{
			APIGroups: []string{""},
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"patch",
				"watch",
			},
		},
		// Read Secrets/ConfigMaps
		{
			APIGroups: []string{""},
			Resources: []string{
				"secrets",
				"configmaps",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		// Read Deployments
		{
			APIGroups: []string{"apps", "extensions"},
			Resources: []string{
				"deployments",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
	}
}
