package argocd

import (
	"context"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// newRoleWithName creates a new ServiceAccount with the given name for the given ArgCD.
func newRoleWithName(name string, cr *argoprojv1a1.ArgoCD) *v1.Role {
	sa := newRole(name, cr)
	sa.Name = name

	lbls := sa.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	sa.ObjectMeta.Labels = lbls

	return sa
}

// newRole returns a new ServiceAccount instance.
func newRole(name string, cr *argoprojv1a1.ArgoCD) *v1.Role {
	return &v1.Role{

		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}

// reconcileRoles will ensure that all ArgoCD Service Accounts are configured.
func (r *ReconcileArgoCD) reconcileRoles(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileRole(applicationController, getPolicyRuleApplicationController(), cr); err != nil {
		return err
	}

	if err := r.reconcileRole(dexServer, getPolicyRuleDexServer(), cr); err != nil {
		return err
	}

	if err := r.reconcileRole(server, getPolicyRuleServer(), cr); err != nil {
		return err
	}

	if err := r.reconcileRole(redisHa, getPolicyRuleRedisHa(), cr); err != nil {
		return err
	}

	return nil
}

// reconcileRole
func (r *ReconcileArgoCD) reconcileRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) error {
	rbacClient := r.kc.RbacV1()

	role, err := rbacClient.Roles(cr.Namespace).Get(context.TODO(), name, metav1.GetOptions{})
	roleExists := true
	if err != nil {
		if errors.IsNotFound(err) {
			roleExists = false
			role = newRoleWithName(name, cr)
		} else {
			return err
		}
	}

	role.Rules = policyRules

	if roleExists {
		_, err = rbacClient.Roles(cr.Namespace).Update(context.TODO(), role, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	} else {
		controllerutil.SetControllerReference(cr, role, r.scheme)
		_, err = rbacClient.Roles(cr.Namespace).Create(context.TODO(), role, metav1.CreateOptions{})
	}
	return err
}
