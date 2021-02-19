package argocd

import (
	"context"
	"fmt"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// newClusterRoleBinding returns a new ClusterRoleBinding instance.
func newClusterRoleBinding(name string, cr *argoprojv1a1.ArgoCD) *v1.ClusterRoleBinding {
	return &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cr.Name,
			Labels:      labelsForCluster(cr),
			Annotations: annotationsForCluster(cr),
		},
	}
}

// newClusterRoleBindingWithname creates a new ClusterRoleBinding with the given name for the given ArgCD.
func newClusterRoleBindingWithname(name string, cr *argoprojv1a1.ArgoCD) *v1.ClusterRoleBinding {
	roleBinding := newClusterRoleBinding(name, cr)
	roleBinding.Name = "cluster-" + generateResourceName(name, cr)

	labels := roleBinding.ObjectMeta.Labels
	labels[common.ArgoCDKeyName] = name
	roleBinding.ObjectMeta.Labels = labels

	return roleBinding
}

// newRoleBinding returns a new RoleBinding instance.
func newRoleBinding(cr *argoprojv1a1.ArgoCD) *v1.RoleBinding {
	return &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cr.Name,
			Labels:      labelsForCluster(cr),
			Annotations: annotationsForCluster(cr),
			Namespace:   cr.Namespace,
		},
	}
}

// newRoleBindingWithname creates a new RoleBinding with the given name for the given ArgCD.
func newRoleBindingWithname(name string, cr *argoprojv1a1.ArgoCD) *v1.RoleBinding {
	roleBinding := newRoleBinding(cr)
	roleBinding.ObjectMeta.Name = fmt.Sprintf("%s-%s", cr.Name, name)

	labels := roleBinding.ObjectMeta.Labels
	labels[common.ArgoCDKeyName] = name
	roleBinding.ObjectMeta.Labels = labels

	return roleBinding
}

// reconcileRoleBindings will ensure that all ArgoCD RoleBindings are configured.
func (r *ReconcileArgoCD) reconcileRoleBindings(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileRoleBinding(applicationController, policyRuleForApplicationController(), cr); err != nil {
		return fmt.Errorf("error reconciling roleBinding for %q: %w", applicationController, err)
	}
	if err := r.reconcileRoleBinding(dexServer, policyRuleForDexServer(), cr); err != nil {
		return fmt.Errorf("error reconciling roleBinding for %q: %w", dexServer, err)
	}

	if err := r.reconcileRoleBinding(redisHa, policyRuleForRedisHa(), cr); err != nil {
		return fmt.Errorf("error reconciling roleBinding for %q: %w", redisHa, err)
	}

	if err := r.reconcileRoleBinding(server, policyRuleForServer(), cr); err != nil {
		return fmt.Errorf("error reconciling roleBinding for %q: %w", server, err)
	}
	return nil
}

func (r *ReconcileArgoCD) reconcileRoleBinding(name string, rules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) error {
	var role *v1.Role
	var sa *corev1.ServiceAccount
	var error error

	if role, error = r.reconcileRole(name, rules, cr); error != nil {
		return error
	}

	if sa, error = r.reconcileServiceAccount(name, cr); error != nil {
		return error
	}

	// get expected name
	roleBinding := newRoleBindingWithname(name, cr)

	// fetch existing rolebinding by name
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: cr.Namespace}, roleBinding)
	roleBindingExists := true
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s : %s", name, err)
		}
		if name == dexServer && isDexDisabled() {
			return nil // Dex is disabled, do nothing
		}
		roleBindingExists = false
		roleBinding = newRoleBindingWithname(name, cr)
	}

	roleBinding.Subjects = []v1.Subject{
		{
			Kind:      v1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
	}
	roleBinding.RoleRef = v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     "Role",
		Name:     role.Name,
	}

	controllerutil.SetControllerReference(cr, roleBinding, r.scheme)
	if roleBindingExists {
		if name == dexServer && isDexDisabled() {
			// Delete any existing RoleBinding created for Dex
			return r.client.Delete(context.TODO(), roleBinding)
		}
		return r.client.Update(context.TODO(), roleBinding)
	}

	return r.client.Create(context.TODO(), roleBinding)
}

func (r *ReconcileArgoCD) reconcileClusterRoleBinding(name string, role *v1.ClusterRole, sa *corev1.ServiceAccount, cr *argoprojv1a1.ArgoCD) error {

	// get expected name
	roleBinding := newClusterRoleBindingWithname(name, cr)
	// fetch existing rolebinding by name
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name}, roleBinding)
	roleBindingExists := true
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		roleBindingExists = false
		roleBinding = newClusterRoleBindingWithname(name, cr)
	}

	roleBinding.Subjects = []v1.Subject{
		{
			Kind:      v1.ServiceAccountKind,
			Name:      generateResourceName(name, cr),
			Namespace: cr.Namespace,
		},
	}
	roleBinding.RoleRef = v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     "ClusterRole",
		Name:     generateResourceName(name, cr),
	}

	controllerutil.SetControllerReference(cr, roleBinding, r.scheme)
	if roleBindingExists {
		return r.client.Update(context.TODO(), roleBinding)
	}
	return r.client.Create(context.TODO(), roleBinding)
}

// reconcileArgoApplier ensures that a specific rolebinding is present in a managedNamespace
func (r *ReconcileArgoCD) reconcileArgoApplier(controlPlaneServiceAccount string, cr *argoprojv1a1.ArgoCD, managedNamespace string) error {

	roleBinding := newRoleBindingWithname(controlPlaneServiceAccount, cr)
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: cr.Namespace}, roleBinding)
	roleBindingExists := true
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		roleBindingExists = false
		roleBinding = &v1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cr.Name,
				Namespace: managedNamespace,
			},
		}
	}
	roleBinding.Subjects = []v1.Subject{
		{
			Kind:      v1.ServiceAccountKind,
			Name:      controlPlaneServiceAccount,
			Namespace: cr.Namespace,
		},
	}
	roleBinding.RoleRef = v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     "ClusterRole",
		Name:     "admin",
	}

	if !roleBindingExists {
		return r.client.Create(context.TODO(), roleBinding)
	}
	return r.client.Update(context.TODO(), roleBinding)
}

func deleteClusterRoleBindings(c client.Client, clusterBindingList *v1.ClusterRoleBindingList) error {
	for _, clusterBinding := range clusterBindingList.Items {
		if err := c.Delete(context.TODO(), &clusterBinding); err != nil {
			return err
		}
	}
	return nil
}
