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

// newRoleBinding returns a new RoleBinding instance.
func newRoleBinding(cr *argoprojv1a1.ArgoCD) *v1.RoleBinding {
	return &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}

// newRoleBindingWithname creates a new RoleBinding with the given name for the given ArgCD.
func newRoleBindingWithname(name string, cr *argoprojv1a1.ArgoCD) *v1.RoleBinding {
	roleBinding := newRoleBinding(cr)
	roleBinding.ObjectMeta.Name = name

	labels := roleBinding.ObjectMeta.Labels
	labels[common.ArgoCDKeyName] = name
	roleBinding.ObjectMeta.Labels = labels

	return roleBinding
}

// reconcileRoleBindings will ensure that all ArgoCD RoleBindings are configured.
func (r *ReconcileArgoCD) reconcileRoleBindings(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileRoleBinding(applicationController, cr); err != nil {
		return err
	}

	if err := r.reconcileRoleBinding(dexServer, cr); err != nil {
		return err
	}

	if err := r.reconcileRoleBinding(redisHa, cr); err != nil {
		return err
	}

	if err := r.reconcileRoleBinding(server, cr); err != nil {
		return err
	}
	return nil
}

// reconcileRoleBinding ensures that a specific rolebinding is present
func (r *ReconcileArgoCD) reconcileRoleBinding(name string, cr *argoprojv1a1.ArgoCD) error {

	rbacClient := r.kc.RbacV1()

	roleBinding, err := rbacClient.RoleBindings(cr.Namespace).Get(context.TODO(), name, metav1.GetOptions{})
	roleBindingExists := true
	if err != nil {
		if errors.IsNotFound(err) {
			roleBindingExists = false
			roleBinding = newRoleBindingWithname(name, cr)
		} else {
			return err
		}
	}

	roleBinding.Subjects = []v1.Subject{
		{
			Kind: v1.ServiceAccountKind,
			Name: name,
		},
	}
	roleBinding.RoleRef = v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     "Role",
		Name:     name,
	}

	controllerutil.SetControllerReference(cr, roleBinding, r.scheme)
	if roleBindingExists {
		return r.client.Update(context.TODO(), roleBinding)
	}

	return r.client.Create(context.TODO(), roleBinding)
}
