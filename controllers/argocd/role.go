package argocd

import (
	"context"
	"fmt"
	"os"
	"reflect"

	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// newRole returns a new Role instance.
func newRole(name string, rules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) *v1.Role {
	return &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateResourceName(name, cr),
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
		Rules: rules,
	}
}

func generateResourceName(argoComponentName string, cr *argoprojv1a1.ArgoCD) string {
	return cr.Name + "-" + argoComponentName
}

// GenerateUniqueResourceName generates unique names for cluster scoped resources
func GenerateUniqueResourceName(argoComponentName string, cr *argoprojv1a1.ArgoCD) string {
	return cr.Name + "-" + cr.Namespace + "-" + argoComponentName
}

func newClusterRole(name string, rules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) *v1.ClusterRole {
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
func (r *ReconcileArgoCD) reconcileRoles(cr *argoprojv1a1.ArgoCD) error {
	params := getPolicyRuleList(r.Client)

	for _, param := range params {
		if _, err := r.reconcileRole(param.name, param.policyRule, cr); err != nil {
			return err
		}
	}

	clusterParams := getPolicyRuleClusterRoleList()

	for _, clusterParam := range clusterParams {
		if _, err := r.reconcileClusterRole(clusterParam.name, clusterParam.policyRule, cr); err != nil {
			return err
		}
	}

	return nil
}

// reconcileRole, reconciles the policy rules for different ArgoCD components, for each namespace
// Managed by a single instance of ArgoCD.
func (r *ReconcileArgoCD) reconcileRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) ([]*v1.Role, error) {
	var roles []*v1.Role

	// create policy rules for each namespace
	for _, namespace := range r.ManagedNamespaces.Items {
		list := &argoprojv1a1.ArgoCDList{}
		listOption := &client.ListOptions{Namespace: namespace.Name}
		err := r.Client.List(context.TODO(), list, listOption)
		if err != nil {
			return nil, err
		}
		// only skip creation of dex and redisHa roles for namespaces that no argocd instance is deployed in
		if len(list.Items) < 1 {
			// only create dexServer and redisHa roles for the namespace where the argocd instance is deployed
			if cr.ObjectMeta.Namespace != namespace.Name && (name == common.ArgoCDDexServerComponent || name == common.ArgoCDRedisHAComponent) {
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
		err = r.Client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, &existingRole)
		if err != nil {
			if !errors.IsNotFound(err) {
				return nil, fmt.Errorf("failed to reconcile the role for the service account associated with %s : %s", name, err)
			}
			if customRole != "" {
				continue // skip creating default role if custom cluster role is provided
			}
			roles = append(roles, role)

			if name == common.ArgoCDDexServerComponent && !UseDex(cr) {

				continue // Dex installation not requested, do nothing
			}

			// Only set ownerReferences for roles in same namespace as ArgoCD CR
			if cr.Namespace == role.Namespace {
				if err = controllerutil.SetControllerReference(cr, role, r.Scheme); err != nil {
					return nil, fmt.Errorf("failed to set ArgoCD CR \"%s\" as owner for role \"%s\": %s", cr.Name, role.Name, err)
				}
			}

			log.Info(fmt.Sprintf("creating role %s for Argo CD instance %s in namespace %s", role.Name, cr.Name, cr.Namespace))
			if err := r.Client.Create(context.TODO(), role); err != nil {
				return nil, err
			}
			continue
		}

		// Delete the existing default role if custom role is specified
		// or if there is an existing Role created for Dex but dex is disabled or not configured
		if customRole != "" ||
			(name == common.ArgoCDDexServerComponent && !UseDex(cr)) {

			log.Info("deleting the existing Dex role because dex is not configured")
			if err := r.Client.Delete(context.TODO(), &existingRole); err != nil {
				return nil, err
			}
			continue
		}

		// if the Rules differ, update the Role
		if !reflect.DeepEqual(existingRole.Rules, role.Rules) {
			existingRole.Rules = role.Rules
			if err := r.Client.Update(context.TODO(), &existingRole); err != nil {
				return nil, err
			}
		}
		roles = append(roles, &existingRole)
	}
	return roles, nil
}

func (r *ReconcileArgoCD) reconcileClusterRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) (*v1.ClusterRole, error) {
	allowed := false
	if allowedNamespace(cr.Namespace, os.Getenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")) {
		allowed = true
	}
	clusterRole := newClusterRole(name, policyRules, cr)
	if err := applyReconcilerHook(cr, clusterRole, ""); err != nil {
		return nil, err
	}

	existingClusterRole := &v1.ClusterRole{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRole.Name}, existingClusterRole)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the cluster role for the service account associated with %s : %s", name, err)
		}
		if !allowed {
			// Do Nothing
			return nil, nil
		}
		return clusterRole, r.Client.Create(context.TODO(), clusterRole)
	}

	if !allowed {
		return nil, r.Client.Delete(context.TODO(), existingClusterRole)
	}

	// if the Rules differ, update the Role
	if !reflect.DeepEqual(existingClusterRole.Rules, clusterRole.Rules) {
		existingClusterRole.Rules = clusterRole.Rules
		if err := r.Client.Update(context.TODO(), existingClusterRole); err != nil {
			return nil, err
		}
	}
	return existingClusterRole, nil
}

func deleteClusterRoles(c client.Client, clusterRoleList *v1.ClusterRoleList) error {
	for _, clusterRole := range clusterRoleList.Items {
		if err := c.Delete(context.TODO(), &clusterRole); err != nil {
			return fmt.Errorf("failed to delete ClusterRole %q during cleanup: %w", clusterRole.Name, err)
		}
	}
	return nil
}
