package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"

	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileRoles will ensure all ArgoCD Server roles are present
func (sr *ServerReconciler) reconcileRoles() error {

	var reconErrs util.MultiError

	// reconcile Roles for managed namespaces
	if err := sr.reconcileManagedRoles(); err != nil {
		reconErrs.Append(err)
	}

	// reconcile Roles for source namespaces
	if err := sr.reconcileSourceRoles(); err != nil {
		reconErrs.Append(err)
	}

	return reconErrs.ErrOrNil()
}

// reconcileManagedRoles manages roles in ArgoCD managed namespaces
func (sr *ServerReconciler) reconcileManagedRoles() error {
	var reconErrs util.MultiError

	for nsName := range sr.ManagedNamespaces {

		roleReq := permissions.RoleRequest{
			ObjectMeta: argoutil.GetObjMeta(resourceName, nsName, sr.Instance.Name, sr.Instance.Namespace, component),
			Rules: getPolicyRulesForArgoCDNamespace(),
			Instance:   sr.Instance,
			Client:    sr.Client,
			Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		}

		// managing non control-plane/ArgoCD namespace, use stricter rules & special resource management label
		if nsName != sr.Instance.Namespace {
			roleReq.Rules = getPolicyRulesForManagedNamespace()
			if len(roleReq.ObjectMeta.Labels) == 0 {
				roleReq.ObjectMeta.Labels = make(map[string]string)
			}
			roleReq.ObjectMeta.Labels[common.ArgoCDKeyRBACType] = common.ArgoCDRBACTypeResourceMananagement
		}

		desiredRole, err := permissions.RequestRole(roleReq)
		if err != nil {
			reconErrs.Append(errors.Wrapf(err, "reconcileManagedRoles: failed to request role %s in namespace %s", desiredRole.Name, desiredRole.Namespace))
			continue
		}

		// custom role in use, cleanup default role
		if getCustomRoleName() != "" {
			err := sr.deleteRole(desiredRole.Name, desiredRole.Namespace)
			if err != nil {
				reconErrs.Append(errors.Wrapf(err, "reconcileManagedRoles: failed to delete role %s in namespace %s", desiredRole.Name, desiredRole.Namespace))
			}
			continue
		}

		// Only set ownerReferences for roles in same namespace as ArgoCD CR
		if sr.Instance.Namespace == desiredRole.Namespace {
			if err = controllerutil.SetControllerReference(sr.Instance, desiredRole, sr.Scheme); err != nil {
				sr.Logger.Error(err, "reconcileManagedRoles: failed to set owner reference for role", "name", desiredRole.Name, "namespace", desiredRole.Namespace)
			}
		}

		// if custom role is not set & default role doesn't exist in the namespace, create it
		existingRole, err := permissions.GetRole(desiredRole.Name, desiredRole.Namespace, sr.Client)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				reconErrs.Append(errors.Wrapf(err, "reconcileManagedRoles: failed to retrieve role %s in namespace %s", desiredRole.Name, desiredRole.Namespace))
				continue
			}

			if err = permissions.CreateRole(desiredRole, sr.Client); err != nil {
				reconErrs.Append(errors.Wrapf(err, "reconcileManagedRoles: failed to create role %s in namespace %s", desiredRole.Name, desiredRole.Namespace))
				continue
			}

			sr.Logger.V(0).Info("role created", "name", desiredRole.Name, "namespace", desiredRole.Namespace)
			continue
		}

		// difference in existing & desired role, update it
		changed := false

		fieldsToCompare := []struct {
			existing, desired interface{}
			extraAction       func()
		}{
			{&existingRole.Rules, &desiredRole.Rules, nil},
		}

		for _, field := range fieldsToCompare {
			argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &changed)
		}
	

		// nothing changed
		if !changed {
			continue
		}
	
		if err = permissions.UpdateRole(existingRole, sr.Client); err != nil {
			reconErrs.Append(errors.Wrapf(err, "reconcileManagedRoles: failed to update role %s in namespace %s", existingRole.Name, existingRole.Namespace))
			continue
		}

		sr.Logger.V(0).Info("role updated", "name", existingRole.Name, "namespace", existingRole.Namespace)
		continue

	}

	return reconErrs.ErrOrNil()
}

// reconcileSourceRoles manages roles for app in any namespaces feature
func (sr *ServerReconciler) reconcileSourceRoles() error {
	var reconErrs util.MultiError

	for nsName := range sr.SourceNamespaces {

		roleReq := permissions.RoleRequest{
			ObjectMeta: argoutil.GetObjMeta(uniqueResourceName, nsName, sr.Instance.Name, sr.Instance.Namespace, component),
			Rules: getPolicyRulesForSourceNamespace(),
			Instance:   sr.Instance,
			Client:    sr.Client,
			Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		}

		// for non control-plane namespace, add special app management label to role
		if nsName != sr.Instance.Namespace {
			if len(roleReq.ObjectMeta.Labels) == 0 {
				roleReq.ObjectMeta.Labels = make(map[string]string)
			}
			roleReq.ObjectMeta.Labels[common.ArgoCDKeyRBACType] = common.ArgoCDRBACTypeAppManagement
		}

		desiredRole, err := permissions.RequestRole(roleReq)
		if err != nil {
			reconErrs.Append(errors.Wrapf(err, "reconcileSourceRoles: failed to request role %s in namespace %s", desiredRole.Name, desiredRole.Namespace))
            continue
		}

		// role doesn't exist in the namespace, create it
		existingRole, err := permissions.GetRole(desiredRole.Name, desiredRole.Namespace, sr.Client)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				reconErrs.Append(errors.Wrapf(err, "reconcileSourceRoles: failed to retrieve role %s in namespace %s", desiredRole.Name, desiredRole.Namespace))
                continue
			}

			if err = permissions.CreateRole(desiredRole, sr.Client); err != nil {
				reconErrs.Append(errors.Wrapf(err, "reconcileSourceRoles: failed to create role %s in namespace %s", desiredRole.Name, desiredRole.Namespace))
                continue
			}

			sr.Logger.V(0).Info("role created", "name", desiredRole.Name, "namespace", desiredRole.Namespace)
			continue
		}

		// difference in existing & desired role, update it
		changed := false

		fieldsToCompare := []struct {
			existing, desired interface{}
			extraAction       func()
		}{
			{&existingRole.Rules, &desiredRole.Rules, nil},
		}

		for _, field := range fieldsToCompare {
			argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &changed)
		}
	
		// nothing changed
		if !changed {
			continue
		}
	
		if err = permissions.UpdateRole(existingRole, sr.Client); err != nil {
			reconErrs.Append(errors.Wrapf(err, "reconcileSourceRoles: failed to update role %s in namespace %s", existingRole.Name, existingRole.Namespace))
			continue
		}

		sr.Logger.V(0).Info("role updated", "name", existingRole.Name, "namespace", existingRole.Namespace)
		continue

	}

	return reconErrs.ErrOrNil()
}

// deleteRoles will delete all ArgoCD Server roles
func (sr *ServerReconciler) deleteRoles(mngNsRoleName, srcNsRoleName string) error {
	var reconErrs util.MultiError

	// delete managed ns roles
	for nsName := range sr.ManagedNamespaces {
		err := sr.deleteRole(mngNsRoleName, nsName)
		if err != nil {
			reconErrs.Append(err)
		}
	}

	// delete source ns roles
	for nsName := range sr.SourceNamespaces {
		err := sr.deleteRole(srcNsRoleName, nsName)
		if err != nil {
			reconErrs.Append(err)
		}
	}

	return reconErrs.ErrOrNil()
}

// deleteRole will delete role with given name.
func (sr *ServerReconciler) deleteRole(name, namespace string) error {
	if err := permissions.DeleteRole(name, namespace, sr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteRole: failed to delete role %s in namespace %s", name, namespace)
	}
	sr.Logger.V(0).Info("role deleted", "name", name, "namespace", namespace)
	return nil
}

// getPolicyRulesForManagedNamespace returns rules for managed ns
func getPolicyRulesForManagedNamespace() []rbacv1.PolicyRule {
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
	}
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

// getPolicyRulesForSourceNamespace returns rules for source ns
func getPolicyRulesForSourceNamespace() []rbacv1.PolicyRule {
	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"applications",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"patch",
				"update",
				"watch",
				"delete",
			},
		},
	}
}
