package appcontroller

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

func (acr *AppControllerReconciler) reconcileRoles() error {
	var reconcileErrs util.MultiError

	if acr.ClusterScoped {
		// delete namespaced RBAC
		err := acr.deleteRole(resourceName, acr.Instance.Namespace)
		reconcileErrs.Append(err)

		roles, _, err := acr.getManagedRBACToBeDeleted()
		if err != nil {
			acr.Logger.Error(err, "failed to retrieve one or more namespaced rbac resources to be deleted")
		} else if len(roles) > 0 {
			acr.Logger.Debug("reconcileRoles: namespace scoped instance detected; deleting app management rbac resources")
			reconcileErrs.Append(acr.DeleteRoles(roles))
		}
	} else {
		err := acr.reconcileRole()
		reconcileErrs.Append(err)

		err = acr.reconcileManagedNsRoles()
		reconcileErrs.Append(err)
	}

	return reconcileErrs.ErrOrNil()
}

func (acr *AppControllerReconciler) reconcileRole() error {
	req := permissions.RoleRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, acr.Instance.Namespace, acr.Instance.Name, acr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Rules:      getPolicyRules(),
		Client:     acr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Instance:   acr.Instance,
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
	return acr.reconRole(req, argocdcommon.UpdateFnRole(updateFn), ignoreDrift)
}

func (acr *AppControllerReconciler) reconcileManagedNsRoles() error {
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

		req := permissions.RoleRequest{
			ObjectMeta: argoutil.GetObjMeta(managedNsResourceName, managedNs, acr.Instance.Name, acr.Instance.Namespace, component, argocdcommon.GetResourceManagementLabel(), util.EmptyMap()),
			Rules:      getManagedNsPolicyRules(),
			Client:     acr.Client,
			Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
			Instance:   acr.Instance,
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
		err = acr.reconRole(req, argocdcommon.UpdateFnRole(updateFn), ignoreDrift)
		reconcileErrs.Append(err)
	}

	return reconcileErrs.ErrOrNil()
}

func (acr *AppControllerReconciler) reconRole(req permissions.RoleRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := permissions.RequestRole(req)
	if err != nil {
		acr.Logger.Debug("reconRole: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconRole: failed to request Role %s in namespace %s", desired.Name, desired.Namespace)
	}

	if desired.Namespace == acr.Instance.Namespace {
		if err = controllerutil.SetControllerReference(acr.Instance, desired, acr.Scheme); err != nil {
			acr.Logger.Error(err, "reconRole: failed to set owner reference for Role", "name", desired.Name, "namespace", desired.Namespace)
		}
	}

	// get custom role name if any
	customRoleName := getCustomRoleName()

	existing, err := permissions.GetRole(desired.Name, desired.Namespace, acr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconRole: failed to retrieve Role %s in namespace %s", desired.Name, desired.Namespace)
		}

		// skip role creation if custom role specified
		if customRoleName != "" {
			acr.Logger.Debug("reconcileManagedRoles: custom role specified; skipping creation of role", "name", desired.Name)
			return nil
		}

		if err = permissions.CreateRole(desired, acr.Client); err != nil {
			return errors.Wrapf(err, "reconRole: failed to create Role %s in namespace %s", desired.Name, desired.Namespace)
		}
		acr.Logger.Info("role created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// delete any existing role if custom role specified
	if customRoleName != "" {
		// delete any existing role
		acr.Logger.Debug("reconRole: custom role specified; deleting existing role", "name", desired.Name)
		return acr.deleteRole(desired.Name, desired.Namespace)
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

	if err = permissions.UpdateRole(existing, acr.Client); err != nil {
		return errors.Wrapf(err, "reconRole: failed to update Role %s", existing.Name)
	}

	acr.Logger.Info("role updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

func (acr *AppControllerReconciler) deleteRole(name, namespace string) error {
	if err := permissions.DeleteRole(name, namespace, acr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		acr.Logger.Error(err, "DeleteRole: failed to delete role", "name", name, "namespace", namespace)
		return err
	}
	acr.Logger.Info("role deleted", "name", name, "namespace", namespace)
	return nil
}

func (acr *AppControllerReconciler) DeleteRoles(roles []types.NamespacedName) error {
	var deletionErr util.MultiError
	for _, role := range roles {
		deletionErr.Append(acr.deleteRole(role.Name, role.Namespace))
	}
	return deletionErr.ErrOrNil()
}

// control-plane ns role
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
				"*",
			},
		},
	}
}

// don't give permissions to manage applications in "managed" namespaces in namespace scoped mode
// based off https://github.com/argoproj/argo-cd/blob/master/manifests/install.yaml#L20738
func getManagedNsPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
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
				"apps",
			},
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
