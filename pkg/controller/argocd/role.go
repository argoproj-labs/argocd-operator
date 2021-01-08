package argocd

import (
	"context"
	"fmt"
	"strings"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	applicationController = "argocd-application-controller"
	server                = "argocd-server"
	redisHa               = "argocd-redis-ha"
	dexServer             = "argocd-dex-server"
)

// newRole returns a new Role instance.
func newRole(name string, cr *argoprojv1a1.ArgoCD) *v1.Role {
	return &v1.Role{

		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}

func generateResourceName(argoComponentName string, cr *argoprojv1a1.ArgoCD) string {
	return cr.Name + "-" + argoComponentName
}

// newRoleWithName creates a new Role with the given name for the given ArgoCD.
func newRoleWithName(name string, cr *argoprojv1a1.ArgoCD) *v1.Role {
	sa := newRole(name, cr)
	sa.Name = fmt.Sprintf("%s-%s", cr.Name, name)

	lbls := sa.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	sa.ObjectMeta.Labels = lbls

	return sa
}

func newClusterRole(name string, rules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) *v1.ClusterRole {
	return &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:        generateResourceName(name, cr),
			Labels:      labelsForCluster(cr),
			Annotations: annotationsForCluster(cr),
		},
		Rules: rules,
	}
}

func allowedNamespace(current string, configuredList string) bool {
	isAllowedNamespace := false
	if configuredList != "" {
		if configuredList == "*" {
			isAllowedNamespace = true
		} else {
			namespaceList := strings.Split(configuredList, ",")
			for _, n := range namespaceList {
				if n == current {
					isAllowedNamespace = true
				}
			}
		}
	}
	return isAllowedNamespace
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

	if role, err := r.reconcileRole(redisHa, policyRuleForRedisHa(), cr); err != nil {
		return role, err
	}

	rules := policyRuleForApplicationControllerClusterRole()
	if cr.Spec.ManagementScope.Cluster != nil && *cr.Spec.ManagementScope.Cluster {
		if allowedNamespace(cr.Namespace, cr.Spec.ManagementScope.Namespaces) {
			rules = append(rules, policyRulesForClusterConfig()...)
		}
	}

	if _, err := r.reconcileClusterRole(applicationController, rules, cr); err != nil {
		return nil, err
	}

	if _, err := r.reconcileClusterRole(server, policyRuleForServerClusterRole(), cr); err != nil {
		return nil, err
	}

	return nil, nil
}

// reconcileRole
func (r *ReconcileArgoCD) reconcileRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) (*v1.Role, error) {
	role := newRoleWithName(name, cr)
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: cr.Namespace}, role) //rbacClient.Roles(cr.Namespace).Get(context.TODO(), role.Name, metav1.GetOptions{})
	roleExists := true
	if err != nil {
		if errors.IsNotFound(err) {
			roleExists = false
			role = newRoleWithName(name, cr)
		} else {
			return role, err
		}
	}

	role.Rules = policyRules

	controllerutil.SetControllerReference(cr, role, r.scheme)

	if err := applyRoleModifiers(cr, role); err != nil {
		return nil, err
	}
	if roleExists {
		return role, r.client.Update(context.TODO(), role)
	}
	return role, r.client.Create(context.TODO(), role)
}

func (r *ReconcileArgoCD) reconcileClusterRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) (*v1.ClusterRole, error) {
	clusterRole := newClusterRole(name, policyRules, cr)

	err := r.client.Get(context.TODO(), types.NamespacedName{Name: clusterRole.Name}, clusterRole)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the cluster role for the service account associated with %s : %s", name, err)
		}
		return clusterRole, r.client.Create(context.TODO(), newClusterRole(name, policyRules, cr))
	}

	clusterRole.Rules = policyRules
	return clusterRole, r.client.Update(context.TODO(), clusterRole)
}
