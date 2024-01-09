package server

import (
	"reflect"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	amerr "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileRoles will ensure all ArgoCD Server roles are present
func (sr *ServerReconciler) reconcileRoles() error {
	sr.Logger.V(0).Info("reconciling role")

	var reconciliationErrors []error

	// reconcile Roles for managed namespaces
	if err := sr.reconcileManagedRoles(); err != nil {
		reconciliationErrors = append(reconciliationErrors, err)
	}

	// reconcile Roles for source namespaces
	if err := sr.reconcileSourceRoles(); err != nil {
		reconciliationErrors = append(reconciliationErrors, err)
	}

	return amerr.NewAggregate(reconciliationErrors)
}

// reconcileManagedRoles manages roles in ArgoCD managed namespaces
func (sr *ServerReconciler) reconcileManagedRoles() error {
	var reconciliationErrors []error

	for nsName := range sr.ManagedNamespaces {

		roleName := getRoleName(sr.Instance.Name)
		roleLabels := common.DefaultResourceLabels(roleName, sr.Instance.Name, ServerControllerComponent)

		_, err := cluster.GetNamespace(nsName, sr.Client)
		if err != nil {
			if !errors.IsNotFound(err) {
				sr.Logger.Error(err, "reconcileManagedRoles: failed to retrieve namespace", "name", nsName)
				reconciliationErrors = append(reconciliationErrors, err)
			}
			continue
		}

		roleRequest := permissions.RoleRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:        roleName,
				Labels:      roleLabels,
				Annotations: sr.Instance.Annotations,
			},
			Client:    sr.Client,
			Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		}

		roleRequest.ObjectMeta.Namespace = nsName

		if nsName == sr.Instance.Namespace {
			roleRequest.Rules = getPolicyRulesForArgoCDNamespace()
		} else {
			roleRequest.Rules = getPolicyRulesForManagedNamespace()
		}

		desiredRole, err := permissions.RequestRole(roleRequest)
		if err != nil {
			sr.Logger.Error(err, "reconcileManagedRoles: failed to request role", "name", desiredRole.Name, "namespace", desiredRole.Namespace)
			sr.Logger.V(1).Info("reconcileManagedRoles: one or more mutations could not be applied")
			reconciliationErrors = append(reconciliationErrors, err)
			continue
		}

		// create default role only if custom role is not set
		if getCustomRoleName() == "" {
			// role doesn't exist in the namespace, create it
			existingRole, err := permissions.GetRole(desiredRole.Name, desiredRole.Namespace, sr.Client)
			if err != nil {
				if !errors.IsNotFound(err) {
					sr.Logger.Error(err, "reconcileManagedRoles: failed to retrieve role", "name", desiredRole.Name, "namespace", desiredRole.Namespace)
					reconciliationErrors = append(reconciliationErrors, err)
					continue
				}

				// Only set ownerReferences for roles in same namespace as ArgoCD CR
				if sr.Instance.Namespace == desiredRole.Namespace {
					if err = controllerutil.SetControllerReference(sr.Instance, desiredRole, sr.Scheme); err != nil {
						sr.Logger.Error(err, "reconcileManagedRoles: failed to set owner reference for role", "name", desiredRole.Name, "namespace", desiredRole.Namespace)
						reconciliationErrors = append(reconciliationErrors, err)
					}
				}

				if err = permissions.CreateRole(desiredRole, sr.Client); err != nil {
					sr.Logger.Error(err, "reconcileManagedRoles: failed to create role", "name", desiredRole.Name, "namespace", desiredRole.Namespace)
					reconciliationErrors = append(reconciliationErrors, err)
					continue
				}
				sr.Logger.V(0).Info("reconcileManagedRoles: role created", "name", desiredRole.Name, "namespace", desiredRole.Namespace)
				continue
			}

			// difference in existing & desired role, reset it
			if !reflect.DeepEqual(existingRole.Rules, desiredRole.Rules) {
				existingRole.Rules = desiredRole.Rules
				if err = permissions.UpdateRole(existingRole, sr.Client); err != nil {
					sr.Logger.Error(err, "reconcileManagedRoles: failed to update role", "name", existingRole.Name, "namespace", existingRole.Namespace)
					reconciliationErrors = append(reconciliationErrors, err)
					continue
				}
				sr.Logger.V(0).Info("reconcileManagedRoles: role updated", "name", existingRole.Name, "namespace", existingRole.Namespace)
			}

			// role found, no changes detected
			continue
		} else {
			// custom role in use, cleanup default role
			err := sr.deleteRole(desiredRole.Name, desiredRole.Namespace)
			if err != nil {
				reconciliationErrors = append(reconciliationErrors, err)
			}
			continue
		}

	}

	return amerr.NewAggregate(reconciliationErrors)
}

// reconcileSourceRoles manages roles for app in namespaces feature
func (sr *ServerReconciler) reconcileSourceRoles() error {
	var reconciliationErrors []error

	for nsName := range sr.SourceNamespaces {

		roleName := getRoleNameForSourceNamespace(sr.Instance.Name, nsName)
		roleLabels := common.DefaultResourceLabels(roleName, sr.Instance.Name, ServerControllerComponent)

		ns, err := cluster.GetNamespace(nsName, sr.Client)
		if err != nil {
			if !errors.IsNotFound(err) {
				sr.Logger.Error(err, "reconcileSourceRoles: failed to retrieve namespace", "name", nsName)
				reconciliationErrors = append(reconciliationErrors, err)
			}
			continue
		}

		// do not reconcile roles for namespaces already containing managed-by label
		// as it already contain roles with permissions to manipulate application resources
		// reconciled during reconcilation of ManagedNamespaces
		if _, ok := ns.Labels[common.ArgoCDArgoprojKeyManagedBy]; ok {
			err := sr.deleteRole(roleName, nsName)
			if err != nil {
				reconciliationErrors = append(reconciliationErrors, err)
			}
			continue
		}

		roleRequest := permissions.RoleRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:        roleName,
				Labels:      roleLabels,
				Annotations: sr.Instance.Annotations,
				Namespace:   nsName,
			},
			Client:    sr.Client,
			Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		}

		roleRequest.Rules = getPolicyRulesForSourceNamespace()

		desiredRole, err := permissions.RequestRole(roleRequest)
		if err != nil {
			sr.Logger.Error(err, "reconcileSourceRoles: failed to request role", "name", desiredRole.Name, "namespace", desiredRole.Namespace)
			sr.Logger.V(1).Info("reconcileSourceRoles: one or more mutations could not be applied")
			reconciliationErrors = append(reconciliationErrors, err)
			continue
		}

		// role doesn't exist in the namespace, create it
		existingRole, err := permissions.GetRole(desiredRole.Name, desiredRole.Namespace, sr.Client)
		if err != nil {
			if !errors.IsNotFound(err) {
				sr.Logger.Error(err, "reconcileSourceRoles: failed to retrieve role", "name", desiredRole.Name, "namespace", desiredRole.Namespace)
				reconciliationErrors = append(reconciliationErrors, err)
				continue
			}

			if err = permissions.CreateRole(desiredRole, sr.Client); err != nil {
				sr.Logger.Error(err, "reconcileSourceRoles: failed to create role", "name", desiredRole.Name, "namespace", desiredRole.Namespace)
				reconciliationErrors = append(reconciliationErrors, err)
				continue
			}
			sr.Logger.V(0).Info("reconcileSourceRoles: role created", "name", desiredRole.Name, "namespace", desiredRole.Namespace)
			continue
		}

		// difference in existing & desired role, reset it
		if !reflect.DeepEqual(existingRole.Rules, desiredRole.Rules) {
			existingRole.Rules = desiredRole.Rules
			if err = permissions.UpdateRole(existingRole, sr.Client); err != nil {
				sr.Logger.Error(err, "reconcileSourceRoles: failed to update role", "name", existingRole.Name, "namespace", existingRole.Namespace)
				reconciliationErrors = append(reconciliationErrors, err)
				continue
			}
			sr.Logger.V(0).Info("reconcileSourceRoles: role updated", "name", existingRole.Name, "namespace", existingRole.Namespace)
		}

		// role found, no changes detected
		continue
	}

	return amerr.NewAggregate(reconciliationErrors)
}

// deleteRoles will delete all ArgoCD Server roles
func (sr *ServerReconciler) deleteRoles(argoCDName, namespace string) error {
	var reconciliationErrors []error

	// delete managed ns roles
	for nsName := range sr.ManagedNamespaces {
		err := sr.deleteRole(getRoleName(argoCDName), nsName)
		if err != nil {
			reconciliationErrors = append(reconciliationErrors, err)
		}
	}

	// delete source ns roles
	for nsName := range sr.SourceNamespaces {
		err := sr.deleteRole(getRoleNameForSourceNamespace(argoCDName, nsName), nsName)
		if err != nil {
			reconciliationErrors = append(reconciliationErrors, err)
		}
	}

	return amerr.NewAggregate(reconciliationErrors)
}

// deleteRole will delete role with given name.
func (sr *ServerReconciler) deleteRole(name, namespace string) error {
	if err := permissions.DeleteRole(name, namespace, sr.Client); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		sr.Logger.Error(err, "deleteRole: failed to delete role", "name", name, "namespace", namespace)
		return err
	}
	sr.Logger.V(0).Info("deleteRole: role deleted", "name", name, "namespace", namespace)
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
