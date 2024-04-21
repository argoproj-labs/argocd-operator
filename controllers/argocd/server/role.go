package server

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func (sr *ServerReconciler) reconcileRoles() error {
	var reconcileErrs util.MultiError

	if sr.ClusterScoped {
		// delete control plane role
		err := sr.deleteRole(resourceName, sr.Instance.Namespace)
		reconcileErrs.Append(err)

		// delete managed ns roles
		roles, _, err := sr.getManagedNsRBAC()
		if err != nil {
			sr.Logger.Error(err, "reconcileRoles: failed to list one or more resource management namespace rbac resources")
		} else if len(roles) > 0 {
			sr.Logger.Debug("reconcileRoles: namespace scoped instance detected; deleting resource management rbac resources")
			reconcileErrs.Append(sr.DeleteRoles(roles))
		}

		// reconcile app source ns roles
		err = sr.reconcileSourceNsRoles()
		reconcileErrs.Append(err)

		// reconcile appset source ns roles
		err = sr.reconcileAppsetSourceNsRoles()
		reconcileErrs.Append(err)

	} else {
		// delete source ns roles
		roles, _, err := sr.getSourceNsRBAC()
		if err != nil {
			sr.Logger.Error(err, "reconcileRoles: failed to list one or more app management namespace rbac resources")
		} else if len(roles) > 0 {
			sr.Logger.Debug("reconcileRoles: namespace scoped instance detected; deleting app management rbac resources")
			reconcileErrs.Append(sr.DeleteRoles(roles))
		}

		// delete appset ns roles
		roles, _, err = sr.getAppsetSourceNsRBAC()
		if err != nil {
			sr.Logger.Error(err, "reconcileRoles: failed to list one or more appset management namespace rbac resources")
		} else if len(roles) > 0 {
			sr.Logger.Debug("reconcileRoles: namespace scoped instance detected; deleting appset management rbac resources")
			reconcileErrs.Append(sr.DeleteRoles(roles))
		}

		// reconcile control plane role
		err = sr.reconcileRole()
		reconcileErrs.Append(err)

		// reconcile managed ns roles
		err = sr.reconcileManagedNsRoles()
		reconcileErrs.Append(err)
	}

	return reconcileErrs.ErrOrNil()
}

// reconcileRole will ensure ArgoCD Server role is present
func (sr *ServerReconciler) reconcileRole() error {
	req := permissions.RoleRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, sr.Instance.Namespace, sr.Instance.Name, sr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Rules:      getPolicyRules(),
		Instance:   sr.Instance,
		Client:     sr.Client,
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
	return sr.reconRole(req, argocdcommon.UpdateFnRole(updateFn), ignoreDrift)
}

func (sr *ServerReconciler) reconcileManagedNsRoles() error {
	var reconcileErrs util.MultiError

	for managedNs := range sr.ManagedNamespaces {
		// Skip namespace if can't be retrieved or in terminating state
		ns, err := cluster.GetNamespace(managedNs, sr.Client)
		if err != nil {
			sr.Logger.Error(err, "reconcileManagedRoles: unable to retrieve namesapce", "name", managedNs)
			continue
		}
		if ns.DeletionTimestamp != nil {
			sr.Logger.Debug("reconcileManagedRoles: skipping namespace in terminating state", "name", managedNs)
			continue
		}

		// Skip control plane namespace
		if managedNs == sr.Instance.Namespace {
			continue
		}

		req := permissions.RoleRequest{
			ObjectMeta: argoutil.GetObjMeta(managedNsResourceName, managedNs, sr.Instance.Name, sr.Instance.Namespace, component, argocdcommon.GetResourceManagementLabel(), util.EmptyMap()),
			Rules:      getManagedNsPolicyRules(),
			Instance:   sr.Instance,
			Client:     sr.Client,
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
		err = sr.reconRole(req, argocdcommon.UpdateFnRole(updateFn), ignoreDrift)
		reconcileErrs.Append(err)
	}

	return reconcileErrs.ErrOrNil()
}

func (sr *ServerReconciler) reconcileSourceNsRoles() error {
	var reconcileErrs util.MultiError

	for sourceNs := range sr.SourceNamespaces {
		// Skip namespace if can't be retrieved or in terminating state
		ns, err := cluster.GetNamespace(sourceNs, sr.Client)
		if err != nil {
			sr.Logger.Error(err, "reconcileSourceNsRoles: unable to retrieve namesapce", "name", sourceNs)
			continue
		}
		if ns.DeletionTimestamp != nil {
			sr.Logger.Debug("reconcileSourceNsRoles: skipping namespace in terminating state", "name", sourceNs)
			continue
		}

		// Skip control plane namespace
		if sourceNs == sr.Instance.Namespace {
			continue
		}

		req := permissions.RoleRequest{
			ObjectMeta: argoutil.GetObjMeta(sourceNsResourceName, sourceNs, sr.Instance.Name, sr.Instance.Namespace, component, argocdcommon.GetAppManagementLabel(), util.EmptyMap()),
			Rules:      getSourceNsPolicyRules(),
			Instance:   sr.Instance,
			Client:     sr.Client,
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
		err = sr.reconRole(req, argocdcommon.UpdateFnRole(updateFn), ignoreDrift)
		reconcileErrs.Append(err)
	}

	return reconcileErrs.ErrOrNil()
}

func (sr *ServerReconciler) reconcileAppsetSourceNsRoles() error {
	var reconcileErrs util.MultiError

	for appsetSourceNs := range sr.AppsetSourceNamespaces {
		// Skip namespace if can't be retrieved or in terminating state
		ns, err := cluster.GetNamespace(appsetSourceNs, sr.Client)
		if err != nil {
			sr.Logger.Error(err, "reconcileAppsetSourceNsRoles: unable to retrieve namesapce", "name", appsetSourceNs)
			continue
		}
		if ns.DeletionTimestamp != nil {
			sr.Logger.Debug("reconcileAppsetSourceNsRoles: skipping namespace in terminating state", "name", appsetSourceNs)
			continue
		}

		// Skip control plane namespace
		if appsetSourceNs == sr.Instance.Namespace {
			continue
		}

		req := permissions.RoleRequest{
			ObjectMeta: argoutil.GetObjMeta(appsetSourceNsResourceName, appsetSourceNs, sr.Instance.Name, sr.Instance.Namespace, component, argocdcommon.GetAppsetManagementLabel(), util.EmptyMap()),
			Rules:      getAppsetSourceNsPolicyRules(),
			Instance:   sr.Instance,
			Client:     sr.Client,
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
		err = sr.reconRole(req, argocdcommon.UpdateFnRole(updateFn), ignoreDrift)
		reconcileErrs.Append(err)
	}

	return reconcileErrs.ErrOrNil()
}

func (sr *ServerReconciler) reconRole(req permissions.RoleRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := permissions.RequestRole(req)
	if err != nil {
		sr.Logger.Debug("reconRole: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconRole: failed to request Role %s in namespace %s", desired.Name, desired.Namespace)
	}

	if desired.Namespace == sr.Instance.Namespace {
		if err = controllerutil.SetControllerReference(sr.Instance, desired, sr.Scheme); err != nil {
			sr.Logger.Error(err, "reconRole: failed to set owner reference for Role", "name", desired.Name, "namespace", desired.Namespace)
		}
	}

	// get custom role name if any
	customRoleName := getCustomRoleName()

	existing, err := permissions.GetRole(desired.Name, desired.Namespace, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconRole: failed to retrieve Role %s in namespace %s", desired.Name, desired.Namespace)
		}

		// skip role creation if custom role specified
		if customRoleName != "" {
			sr.Logger.Debug("reconcileManagedRoles: custom role specified; skipping creation of role", "name", desired.Name)
			return nil
		}

		if err = permissions.CreateRole(desired, sr.Client); err != nil {
			return errors.Wrapf(err, "reconRole: failed to create Role %s in namespace %s", desired.Name, desired.Namespace)
		}
		sr.Logger.Info("role created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// delete any existing role if custom role specified
	if customRoleName != "" {
		// delete any existing role
		sr.Logger.Debug("reconRole: custom role specified; deleting existing role", "name", desired.Name)
		return sr.deleteRole(desired.Name, desired.Namespace)
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

	if err = permissions.UpdateRole(existing, sr.Client); err != nil {
		return errors.Wrapf(err, "reconRole: failed to update Role %s", existing.Name)
	}

	sr.Logger.Info("role updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteRole will delete role with given name.
func (sr *ServerReconciler) deleteRole(name, namespace string) error {
	if err := permissions.DeleteRole(name, namespace, sr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteRole: failed to delete role %s in namespace %s", name, namespace)
	}
	sr.Logger.Info("role deleted", "name", name, "namespace", namespace)
	return nil
}

func (sr *ServerReconciler) DeleteRoles(roles []types.NamespacedName) error {
	var deletionErr util.MultiError
	for _, role := range roles {
		deletionErr.Append(sr.deleteRole(role.Name, role.Namespace))
	}
	return deletionErr.ErrOrNil()
}

// getPolicyRules returns rules for control plane ns
func getPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"*",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"get",
				"patch",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"secrets",
				"configmaps",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"patch",
				"delete",
			},
		}, {
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"applications",
				"appprojects",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"delete",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"create",
				"list",
			},
		},
		{
			APIGroups: []string{
				"batch",
			},
			Resources: []string{
				"jobs",
				"cronjobs",
				"cronjobs/finalizers",
			},
			Verbs: []string{
				"create",
				"update",
			},
		},
	}
}

// getPolicyRules returns rules for non control plane ns resource management
func getManagedNsPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"*",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"get",
				"patch",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"secrets",
				"configmaps",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"patch",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"create",
				"list",
			},
		},
		{
			APIGroups: []string{
				"batch",
			},
			Resources: []string{
				"jobs",
				"cronjobs",
				"cronjobs/finalizers",
			},
			Verbs: []string{
				"create",
				"update",
			},
		},
	}
}

// getPolicyRules returns rules for non control plane ns application management
func getSourceNsPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"applications",
				"appprojects",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"delete",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"create",
				"list",
			},
		},
		{
			APIGroups: []string{
				"batch",
			},
			Resources: []string{
				"jobs",
				"cronjobs",
				"cronjobs/finalizers",
			},
			Verbs: []string{
				"create",
				"update",
			},
		},
	}
}

// getPolicyRules returns rules for non control plane ns application management
func getAppsetSourceNsPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"applicationsets",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"delete",
				"patch",
			},
		},
	}
}
