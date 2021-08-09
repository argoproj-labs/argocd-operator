package argocd

import (
	"context"
	"fmt"
	"reflect"

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
	roleBinding.Name = GenerateUniqueResourceName(name, cr)

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

	if err := r.reconcileRoleBinding(redisHa, policyRuleForRedisHa(cr), cr); err != nil {
		return fmt.Errorf("error reconciling roleBinding for %q: %w", redisHa, err)
	}

	if err := r.reconcileRoleBinding(server, policyRuleForServer(), cr); err != nil {
		return fmt.Errorf("error reconciling roleBinding for %q: %w", server, err)
	}
	return nil
}

// reconcileRoleBinding, creates RoleBindings for every role and associates it with the right ServiceAccount.
// This would create RoleBindings for all the namespaces managed by the ArgoCD instance.
func (r *ReconcileArgoCD) reconcileRoleBinding(name string, rules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) error {
	var roles []*v1.Role
	var sa *corev1.ServiceAccount
	var error error

	if sa, error = r.reconcileServiceAccount(name, cr); error != nil {
		return error
	}

	if roles, error = r.reconcileRole(name, rules, cr); error != nil {
		return error
	}

	for _, role := range roles {
		// get expected name
		roleBinding := newRoleBindingWithname(name, cr)
		roleBinding.Namespace = role.Namespace

		// fetch existing rolebinding by name
		existingRoleBinding := &v1.RoleBinding{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: roleBinding.Namespace}, existingRoleBinding)
		roleBindingExists := true
		if err != nil {
			if !errors.IsNotFound(err) {
				return fmt.Errorf("failed to get the rolebinding associated with %s : %s", name, err)
			}
			roleBindingExists = false
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

		if roleBindingExists {
			// if the RoleRef changes, delete the existing role binding and create a new one
			if !reflect.DeepEqual(roleBinding.RoleRef, existingRoleBinding.RoleRef) {
				if err = r.client.Delete(context.TODO(), existingRoleBinding); err != nil {
					return err
				}
			} else {
				existingRoleBinding.Subjects = roleBinding.Subjects
				if err = r.client.Update(context.TODO(), existingRoleBinding); err != nil {
					return err
				}
				continue
			}
		}

		controllerutil.SetControllerReference(cr, roleBinding, r.scheme)
		if err = r.client.Create(context.TODO(), roleBinding); err != nil {
			return err
		}
	}
	return nil
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

	if roleBindingExists && role == nil {
		return r.client.Delete(context.TODO(), roleBinding)
	}

	if !roleBindingExists && role == nil {
		// DO Nothing
		return nil
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
		Name:     GenerateUniqueResourceName(name, cr),
	}

	controllerutil.SetControllerReference(cr, roleBinding, r.scheme)
	if roleBindingExists {
		return r.client.Update(context.TODO(), roleBinding)
	}
	return r.client.Create(context.TODO(), roleBinding)
}

func deleteClusterRoleBindings(c client.Client, clusterBindingList *v1.ClusterRoleBindingList) error {
	for _, clusterBinding := range clusterBindingList.Items {
		if err := c.Delete(context.TODO(), &clusterBinding); err != nil {
			return fmt.Errorf("failed to delete ClusterRoleBinding %q during cleanup: %w", clusterBinding.Name, err)
		}
	}
	return nil
}
