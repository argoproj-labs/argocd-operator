package server

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"

	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileRole will ensure ArgoCD Server role is present
func (sr *ServerReconciler) reconcileRole() error {

	req := permissions.RoleRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, sr.Instance.Namespace, sr.Instance.Name, sr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Rules:      getPolicyRulesForArgoCDNamespace(),
		Instance:   sr.Instance,
		Client:     sr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	desired, err := permissions.RequestRole(req)
	if err != nil {
		sr.Logger.Debug("reconcileRole: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconcileRole: failed to request Role %s in namespace %s", desired.Name, desired.Namespace)
	}

	// custom role in use, cleanup default role
	if getCustomRoleName() != "" {
		return sr.deleteRole(desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(sr.Instance, desired, sr.Scheme); err != nil {
		sr.Logger.Error(err, "reconcileRole: failed to set owner reference for role", "name", desired.Name, "namespace", desired.Namespace)
	}

	// if custom role is not set & default role doesn't exist in the namespace, create it
	existing, err := permissions.GetRole(desired.Name, desired.Namespace, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileRole: failed to retrieve Role %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = permissions.CreateRole(desired, sr.Client); err != nil {
			return errors.Wrapf(err, "reconcileRole: failed to create Role %s in namespace %s", desired.Name, desired.Namespace)
		}

		sr.Logger.Info("role created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// difference in existing & desired role, update it
	changed := false
	fieldsToCompare := []argocdcommon.FieldToCompare{
		{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
		{Existing: &existing.Rules, Desired: &desired.Rules, ExtraAction: nil},
	}
	argocdcommon.UpdateIfChanged(fieldsToCompare, &changed)

	// nothing changed
	if !changed {
		return nil
	}

	if err = permissions.UpdateRole(existing, sr.Client); err != nil {
		return errors.Wrapf(err, "reconcileRole: failed to update role %s in namespace %s", existing.Name, existing.Namespace)
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

// getPolicyRulesForArgoCDNamespace returns rules for argocd ns
func getPolicyRulesForArgoCDNamespace() []rbacv1.PolicyRule {
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
	}
}
