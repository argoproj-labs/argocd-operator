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

// newClusterRoleWithName creates a new ClusterRole with the given name for the given ArgCD.
func newClusterRoleWithName(name string, cr *argoprojv1a1.ArgoCD) *v1.ClusterRole {
	sa := newClusterRole(name, cr)
	sa.Name = name

	lbls := sa.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	sa.ObjectMeta.Labels = lbls

	return sa
}

// newClusterRole returns a new ClusterRole instance.
func newClusterRole(name string, cr *argoprojv1a1.ArgoCD) *v1.ClusterRole {
	return &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{},
		},
	}
}

// reconcileRoles will ensure that all ArgoCD Service Accounts are configured.
func (r *ReconcileArgoCD) reconcileRoles(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileRole(applicationController, policyRuleForApplicationController(), cr); err != nil {
		return err
	}

	if err := r.reconcileRole(dexServer, policyRuleForDexServer(), cr); err != nil {
		return err
	}

	if err := r.reconcileRole(server, policyRuleForServer(), cr); err != nil {
		return err
	}

	if err := r.reconcileRole(redisHa, policyRuleForRedisHa(), cr); err != nil {
		return err
	}

	if err := r.reconcileClusterRole(applicationController, policyRuleForApplicationControllerClusterRole(), cr); err != nil {
		return err
	}

	if err := r.reconcileClusterRole(server, policyRuleForServerClusterRole(), cr); err != nil {
		return err
	}

	return nil
}

// reconcileClusterRole
func (r *ReconcileArgoCD) reconcileClusterRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) error {
	rbacClient := r.kc.RbacV1()

	role, err := rbacClient.ClusterRoles().Get(context.TODO(), name, metav1.GetOptions{})
	roleExists := true
	if err != nil {
		if errors.IsNotFound(err) {
			roleExists = false
			role = newClusterRoleWithName(name, cr)
		} else {
			return err
		}
	}

	role.Rules = policyRules

	if roleExists {
		_, err = rbacClient.ClusterRoles().Update(context.TODO(), role, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	} else {
		controllerutil.SetControllerReference(cr, role, r.scheme)
		_, err = rbacClient.ClusterRoles().Create(context.TODO(), role, metav1.CreateOptions{})
	}
	return err
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
