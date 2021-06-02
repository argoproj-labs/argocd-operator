package argocd

import (
	"context"
	"fmt"
	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	applicationController = "argocd-application-controller"
	server                = "argocd-server"
	redisHa               = "argocd-redis-ha"
	dexServer             = "argocd-dex-server"
)

// newRole returns a new Role instance.
func newRole(name string, rules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) *v1.Role {
	return &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateResourceName(name, cr),
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
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
			Labels:      labelsForCluster(cr),
			Annotations: annotationsForCluster(cr),
		},
		Rules: rules,
	}
}

// reconcileRoles will ensure that all ArgoCD Service Accounts are configured.
func (r *ReconcileArgoCD) reconcileRoles(cr *argoprojv1a1.ArgoCD) (role *v1.Role, err error) {
	if role, err := r.reconcileRole(applicationController, policyRuleForApplicationController(), cr); err != nil {
		return role, err
	}

	if role, err := r.reconcileRole(dexServer, policyRuleForDexServer(), cr); err != nil {
		return role, err
	}

	if role, err := r.reconcileRole(server, policyRuleForServer(), cr); err != nil {
		return role, err
	}

	if role, err := r.reconcileRole(redisHa, policyRuleForRedisHa(cr), cr); err != nil {
		return role, err
	}

	if _, err := r.reconcileClusterRole(applicationController, policyRuleForApplicationController(), cr); err != nil {
		return nil, err
	}

	if _, err := r.reconcileClusterRole(server, policyRuleForServerClusterRole(), cr); err != nil {
		return nil, err
	}

	return nil, nil
}

// reconcileRole
func (r *ReconcileArgoCD) reconcileRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) (*v1.Role, error) {
	role := newRole(name, policyRules, cr)
	if err := applyReconcilerHook(cr, role, ""); err != nil {
		return nil, err
	}
	if name == applicationController {
		log.Info("ROLE RULES: ", role.Rules)
	}
	existingRole := v1.Role{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: cr.Namespace}, &existingRole)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the role for the service account associated with %s : %s", name, err)
		}
		if name == dexServer && isDexDisabled() {
			return role, nil // Dex is disabled, do nothing
		}
		controllerutil.SetControllerReference(cr, role, r.scheme)
		return role, r.client.Create(context.TODO(), role)
	}

	if name == dexServer && isDexDisabled() {
		// Delete any existing Role created for Dex
		return role, r.client.Delete(context.TODO(), role)
	}
	existingRole.Rules = role.Rules
	controllerutil.SetControllerReference(cr, role, r.scheme)
	return role, r.client.Update(context.TODO(), role)
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
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: clusterRole.Name}, existingClusterRole)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the cluster role for the service account associated with %s : %s", name, err)
		}
		if !allowed {
			// Do Nothing
			return nil, nil
		}
		controllerutil.SetControllerReference(cr, clusterRole, r.scheme)
		return clusterRole, r.client.Create(context.TODO(), clusterRole)
	}

	if !allowed {
		return nil, r.client.Delete(context.TODO(), existingClusterRole)
	}

	existingClusterRole.Rules = clusterRole.Rules
	return existingClusterRole, r.client.Update(context.TODO(), existingClusterRole)
}

func deleteClusterRoles(c client.Client, clusterRoleList *v1.ClusterRoleList) error {
	for _, clusterRole := range clusterRoleList.Items {
		if err := c.Delete(context.TODO(), &clusterRole); err != nil {
			return fmt.Errorf("failed to delete ClusterRole %q during cleanup: %w", clusterRole.Name, err)
		}
	}
	return nil
}
