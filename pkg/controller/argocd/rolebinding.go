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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// newRoleBinding returns a new RoleBinding instance.
func newClusterRoleBinding(cr *argoprojv1a1.ArgoCD) *v1.ClusterRoleBinding {
	return &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Labels:    labelsForCluster(cr),
			Namespace: cr.Namespace,
		},
	}
}

// newRoleBindingWithname creates a new RoleBinding with the given name for the given ArgCD.
func newClusterRoleBindingWithname(name string, cr *argoprojv1a1.ArgoCD) *v1.ClusterRoleBinding {
	roleBinding := newClusterRoleBinding(cr)
	roleBinding.ObjectMeta.Name = fmt.Sprintf("%s-%s", cr.Name, name)

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

func (r *ReconcileArgoCD) reconcileRoleBinding(name string, role *v1.Role, sa *corev1.ServiceAccount, cr *argoprojv1a1.ArgoCD) error {

	rbacClient := r.kc.RbacV1()

	// get expectated name
	roleBinding := newRoleBindingWithname(name, cr)

	// fetch existing rolebinding by name
	roleBinding, err := rbacClient.RoleBindings(cr.Namespace).Get(context.TODO(), roleBinding.Name, metav1.GetOptions{})
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
		_, err = rbacClient.RoleBindings(cr.Namespace).Update(context.TODO(), roleBinding, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	} else {
		_, err = rbacClient.RoleBindings(cr.Namespace).Create(context.TODO(), roleBinding, metav1.CreateOptions{})
	}
	return err
}

func (r *ReconcileArgoCD) reconcileClusterRoleBinding(name string, role *v1.ClusterRole, sa *corev1.ServiceAccount, cr *argoprojv1a1.ArgoCD) error {

	rbacClient := r.kc.RbacV1()

	// get expectated name
	roleBinding := newClusterRoleBindingWithname(name, cr)

	// fetch existing rolebinding by name
	roleBinding, err := rbacClient.ClusterRoleBindings().Get(context.TODO(), roleBinding.Name, metav1.GetOptions{})
	roleBindingExists := true
	if err != nil {
		if errors.IsNotFound(err) {
			roleBindingExists = false
			roleBinding = newClusterRoleBindingWithname(name, cr)
		} else {
			return err
		}
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
		Kind:     "ClusterRole",
		Name:     role.Name,
	}

	controllerutil.SetControllerReference(cr, roleBinding, r.scheme)
	if roleBindingExists {
		_, err = rbacClient.ClusterRoleBindings().Update(context.TODO(), roleBinding, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	} else {
		_, err = rbacClient.ClusterRoleBindings().Create(context.TODO(), roleBinding, metav1.CreateOptions{})
	}
	return err
}
