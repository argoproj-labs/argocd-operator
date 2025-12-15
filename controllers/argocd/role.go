package argocd

import (
	"context"
	"fmt"
	"reflect"

	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// newRole returns a new Role instance.
func newRole(name string, rules []v1.PolicyRule, cr *argoproj.ArgoCD) *v1.Role {
	return &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateResourceName(name, cr),
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
		Rules: rules,
	}
}

func generateResourceName(argoComponentName string, cr *argoproj.ArgoCD) string {
	return cr.Name + "-" + argoComponentName
}

// GenerateUniqueResourceName generates unique names for cluster scoped resources
func GenerateUniqueResourceName(argoComponentName string, cr *argoproj.ArgoCD) string {
	return cr.Name + "-" + cr.Namespace + "-" + argoComponentName
}

func newClusterRole(name string, rules []v1.PolicyRule, cr *argoproj.ArgoCD) *v1.ClusterRole {

	return &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:        GenerateUniqueResourceName(name, cr),
			Labels:      argoutil.LabelsForCluster(cr),
			Annotations: argoutil.AnnotationsForCluster(cr),
		},
		Rules: rules,
	}
}

// reconcileRoles will ensure that all ArgoCD Service Accounts are configured.
func (r *ReconcileArgoCD) reconcileRoles(cr *argoproj.ArgoCD) error {
	params := getPolicyRuleList(r.Client)

	for _, param := range params {
		if _, err := r.reconcileRole(param.name, param.policyRule, cr); err != nil {
			return err
		}
	}

	return nil
}

// reconcileRole, reconciles the policy rules for different ArgoCD components, for each namespace
// Managed by a single instance of ArgoCD.
func (r *ReconcileArgoCD) reconcileRole(name string, policyRules []v1.PolicyRule, cr *argoproj.ArgoCD) ([]*v1.Role, error) {
	var roles []*v1.Role

	// create policy rules for each namespace
	for _, namespace := range r.ManagedNamespaces.Items {
		// If encountering a terminating namespace remove managed-by label from it and skip reconciliation - This should trigger
		// clean-up of roles/rolebindings and removal of namespace from cluster secret
		if namespace.DeletionTimestamp != nil {
			if _, ok := namespace.Labels[common.ArgoCDManagedByLabel]; ok {
				delete(namespace.Labels, common.ArgoCDManagedByLabel)
				argoutil.LogResourceUpdate(log, &namespace, "namespace is terminating, removing 'managed-by' label")
				_ = r.Update(context.TODO(), &namespace)
			}
			continue
		}

		// Look for ArgoCD CRs in the managed namespace
		list := &argoproj.ArgoCDList{}
		listOption := &client.ListOptions{Namespace: namespace.Name}
		err := r.List(context.TODO(), list, listOption)
		if err != nil {
			return nil, err
		}
		// only skip creation of dex and redisHa roles for namespaces that no argocd instance is deployed in
		if len(list.Items) < 1 {
			// namespace doesn't contain argocd instance, so skip all the ArgoCD internal roles
			if cr.Namespace != namespace.Name && (name != common.ArgoCDApplicationControllerComponent && name != common.ArgoCDServerComponent) {
				continue
			}
		}
		customRole := getCustomRoleName(name)
		role := newRole(name, policyRules, cr)
		if err := applyReconcilerHook(cr, role, ""); err != nil {
			return nil, err
		}
		role.Namespace = namespace.Name
		existingRole := v1.Role{}
		err = r.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, &existingRole)
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, fmt.Errorf("failed to reconcile the role for the service account associated with %s : %s", name, err)
			}
			if customRole != "" {
				continue // skip creating default role if custom cluster role is provided
			}
			roles = append(roles, role)

			if (name == common.ArgoCDDexServerComponent && !UseDex(cr)) ||
				!UseApplicationController(name, cr) || !UseRedis(name, cr) || !UseServer(name, cr) {
				continue // Component installation is not requested, do nothing
			}

			// Only set ownerReferences for roles in same namespace as ArgoCD CR
			if cr.Namespace == role.Namespace {
				if err = controllerutil.SetControllerReference(cr, role, r.Scheme); err != nil {
					return nil, fmt.Errorf("failed to set ArgoCD CR \"%s\" as owner for role \"%s\": %s", cr.Name, role.Name, err)
				}
			}

			argoutil.LogResourceCreation(log, role)
			if err := r.Create(context.TODO(), role); err != nil {
				return nil, err
			}
			continue
		} else {
			shouldDelete := false
			explanation := ""
			if !UseApplicationController(name, cr) {
				shouldDelete = true
				explanation = "application controller is disabled"
			} else if !UseRedis(name, cr) {
				shouldDelete = true
				explanation = "redis is disabled"
			} else if !UseServer(name, cr) {
				shouldDelete = true
				explanation = "server is disabled"
			}
			if shouldDelete {
				argoutil.LogResourceDeletion(log, role, explanation)
				if err := r.Delete(context.TODO(), role); err != nil {
					return nil, err
				}
			}
		}

		// Delete the existing default role if custom role is specified
		// or if there is an existing Role created for Dex but dex is disabled or not configured
		if customRole != "" ||
			(name == common.ArgoCDDexServerComponent && !UseDex(cr)) {

			var explanation string
			if customRole != "" {
				explanation = "custom cluster role is provided"
			} else {
				explanation = "dex is disabled or not configured"
			}
			argoutil.LogResourceDeletion(log, &existingRole, explanation)
			if err := r.Delete(context.TODO(), &existingRole); err != nil {
				return nil, err
			}
			continue
		}

		// if the Rules differ, update the Role
		if !reflect.DeepEqual(existingRole.Rules, role.Rules) {
			existingRole.Rules = role.Rules
			argoutil.LogResourceUpdate(log, &existingRole, "updating policy rules")
			if err := r.Update(context.TODO(), &existingRole); err != nil {
				return nil, err
			}
		}
		roles = append(roles, &existingRole)
	}
	return roles, nil
}

func deleteClusterRoles(c client.Client, clusterRoleList *v1.ClusterRoleList) error {
	for _, clusterRole := range clusterRoleList.Items {
		argoutil.LogResourceDeletion(log, &clusterRole, "cleaning up cluster resources")
		if err := c.Delete(context.TODO(), &clusterRole); err != nil {
			return fmt.Errorf("failed to delete ClusterRole %q during cleanup: %w", clusterRole.Name, err)
		}
	}
	return nil
}

// checkCustomClusterRoleMode checks if custom ClusterRole mode is enabled and deletes default ClusterRoles if they exist
func checkCustomClusterRoleMode(r *ReconcileArgoCD, cr *argoproj.ArgoCD, componentName string, allowed bool) (bool, error) {
	// check if it is cluster-scoped instance namespace and user doesn't want to use default ClusterRole
	if allowed && cr.Spec.DefaultClusterScopedRoleDisabled {

		// in case DefaultClusterScopedRoleDisabled was false earlier and default ClusterRole was created, then delete it.
		existingClusterRole := &v1.ClusterRole{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: GenerateUniqueResourceName(componentName, cr)}, existingClusterRole); err == nil {
			// default ClusterRole exists, now delete it
			argoutil.LogResourceDeletion(log, existingClusterRole, "custom clusterrole mode is enabled, deleting default cluster role")
			if err := r.Delete(context.TODO(), existingClusterRole); err != nil {
				return true, fmt.Errorf("failed to delete existing cluster role for the service account associated with %s : %s", componentName, err)
			}
		} else {
			// if it is "Not Found" error then ignore it else return the error
			if !errors.IsNotFound(err) {
				return true, err
			}
		}

		// don't create a default ClusterRole and return
		return true, nil
	}

	// custom ClusterRole is not enabled, continue process
	return false, nil
}

// configureAggregatedClusterRole updates the ClusterRole and adds required fields for aggregated ClusterRole mode
func configureAggregatedClusterRole(cr *argoproj.ArgoCD, clusterRole *v1.ClusterRole, componentName string) {

	// if it is base ClusterRole then add AggregationRule, Annotations fields and remove default Rules
	if componentName == common.ArgoCDApplicationControllerComponent {
		clusterRole.AggregationRule = &v1.AggregationRule{
			ClusterRoleSelectors: []metav1.LabelSelector{
				{
					MatchLabels: map[string]string{
						common.ArgoCDAggregateToControllerLabelKey: "true",
						common.ArgoCDKeyManagedBy:                  cr.Name,
					},
				},
			},
		}
		clusterRole.Annotations[common.AutoUpdateAnnotationKey] = "true"
		clusterRole.Rules = []v1.PolicyRule{}
	}

	// if ClusterRole is for Admin permissions then add AggregationRule and Labels
	if componentName == common.ArgoCDApplicationControllerComponentAdmin {
		clusterRole.AggregationRule = &v1.AggregationRule{
			ClusterRoleSelectors: []metav1.LabelSelector{
				{
					MatchLabels: map[string]string{
						common.ArgoCDAggregateToAdminLabelKey: "true",
						common.ArgoCDKeyManagedBy:             cr.Name,
					},
				},
			},
		}
		clusterRole.Labels[common.ArgoCDAggregateToControllerLabelKey] = "true"
	}

	// if ClusterRole is for View permissions then add Labels
	if componentName == common.ArgoCDApplicationControllerComponentView {
		clusterRole.Labels[common.ArgoCDAggregateToControllerLabelKey] = "true"
	}
}

// matchAggregatedClusterRoleFields compares field values of expected and existing ClusterRoles for aggregated ClusterRole
func matchAggregatedClusterRoleFields(expectedClusterRole *v1.ClusterRole, existingClusterRole *v1.ClusterRole, name string) (bool, string) {
	changed := false
	aggregatedClusterRoleExists := true
	var explanation string

	// if it is base ClusterRole then compare AggregationRule, Annotations and Rules
	if name == common.ArgoCDApplicationControllerComponent {

		if !reflect.DeepEqual(existingClusterRole.AggregationRule, expectedClusterRole.AggregationRule) {
			aggregatedClusterRoleExists = false
			existingClusterRole.AggregationRule = expectedClusterRole.AggregationRule
			explanation = "aggregation rule"
			changed = true
		}

		if !reflect.DeepEqual(existingClusterRole.Annotations, expectedClusterRole.Annotations) {
			existingClusterRole.Annotations = expectedClusterRole.Annotations
			if changed {
				explanation += ", "
			}
			explanation += "annotations"
			changed = true
		}

		// if existing ClusterRole is not Aggregated ClusterRole then only make Rules empty
		if !aggregatedClusterRoleExists {
			existingClusterRole.Rules = []v1.PolicyRule{}
		}
	}

	// if ClusterRole is for View permissions then compare Labels
	if name == common.ArgoCDApplicationControllerComponentView {
		if !reflect.DeepEqual(existingClusterRole.Labels, expectedClusterRole.Labels) {
			existingClusterRole.Labels = expectedClusterRole.Labels
			if changed {
				explanation += ", "
			}
			explanation += "labels"
			changed = true
		}
	}

	// if ClusterRole is for Admin permissions then compare AggregationRule and Labels
	if name == common.ArgoCDApplicationControllerComponentAdmin {
		if !reflect.DeepEqual(existingClusterRole.AggregationRule, expectedClusterRole.AggregationRule) {
			existingClusterRole.AggregationRule = expectedClusterRole.AggregationRule
			if changed {
				explanation += ", "
			}
			explanation += "aggregation rule"
			changed = true
		}

		if !reflect.DeepEqual(existingClusterRole.Labels, expectedClusterRole.Labels) {
			existingClusterRole.Labels = expectedClusterRole.Labels
			if changed {
				explanation += ", "
			}
			explanation += "labels"
			changed = true
		}
	}

	return changed, explanation
}

// matchDefaultClusterRoleFields compares field values of expected and existing ClusterRoles for default ClusterRole
func matchDefaultClusterRoleFields(expectedClusterRole *v1.ClusterRole, existingClusterRole *v1.ClusterRole, name string) (bool, string) {
	changed := false
	var explanation string

	// if it is base ClusterRole then compare AggregationRule and Annotations
	if name == common.ArgoCDApplicationControllerComponent {
		if !reflect.DeepEqual(existingClusterRole.AggregationRule, expectedClusterRole.AggregationRule) {
			existingClusterRole.AggregationRule = expectedClusterRole.AggregationRule
			explanation = "aggregation rule"
			changed = true
		}

		if !reflect.DeepEqual(existingClusterRole.Annotations, expectedClusterRole.Annotations) {
			existingClusterRole.Annotations = expectedClusterRole.Annotations
			if changed {
				explanation += ", "
			}
			explanation += "annotations"
			changed = true
		}
	}

	// for all default ClusterRoles compare Rules
	if !reflect.DeepEqual(existingClusterRole.Rules, expectedClusterRole.Rules) {
		existingClusterRole.Rules = expectedClusterRole.Rules
		if changed {
			explanation += ", "
		}
		explanation += "policy rules"
		changed = true
	}

	return changed, explanation
}

func verifyInstallationMode(cr *argoproj.ArgoCD, allowed bool) error {
	if allowed && cr.Spec.DefaultClusterScopedRoleDisabled && cr.Spec.AggregatedClusterRoles {
		return fmt.Errorf("custom Cluster Roles and Aggregated Cluster Roles can not be used together")
	}
	return nil
}
