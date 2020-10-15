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

const (
	openshiftConfigNamespace = "openshift-config"
)

// newClusterRoleBinding returns a new ClusterRoleBinding instance.
func newClusterRoleBinding(name string, cr *argoprojv1a1.ArgoCD) *v1.ClusterRoleBinding {
	return &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-" + generateResourceName(name, cr),
			Namespace: cr.Namespace,
			Labels:    map[string]string{},
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
			Name:      cr.Name,
			Labels:    labelsForCluster(cr),
			Namespace: cr.Namespace,
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

	if err := r.reconcileClusterRoleBinding(server, cr); err != nil {
		return err
	}

	if err := r.reconcileClusterRoleBinding(applicationController, cr); err != nil {
		return err
	}

	if err := r.reconcileManagedNamespaces(cr); err != nil {
		return err
	}

	return nil
}

// reconcileClusterRoleBinding ensures that a specific clusterrolebinding is present
func (r *ReconcileArgoCD) reconcileClusterRoleBinding(name string, cr *argoprojv1a1.ArgoCD) error {

	rbacClient := r.kc.RbacV1()

	roleBinding, err := rbacClient.ClusterRoleBindings().Get(context.TODO(), "cluster-"+generateResourceName(name, cr), metav1.GetOptions{})
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
			Name:      name,
			Namespace: cr.Namespace,
		},
	}
	roleBinding.RoleRef = v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     "ClusterRole",
		Name:     generateResourceName(name, cr),
	}

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

// generateNameOfRoleBindingForCrossNamespace ensures that a specific rolebinding is present
func (r *ReconcileArgoCD) reconcileManagedNamespaces(cr *argoprojv1a1.ArgoCD) error {

	var namespacesToBeManaged []string
	if cr.Spec.ManagementScope.Namespaces != nil {
		namespacesToBeManaged = *cr.Spec.ManagementScope.Namespaces
	}
	if cr.Spec.ManagementScope.ClusterConfig != nil && *cr.Spec.ManagementScope.ClusterConfig == true {
		namespacesToBeManaged = append(namespacesToBeManaged, openshiftConfigNamespace)
	}

	for _, namespace := range namespacesToBeManaged {
		err := r.reconcileArgoApplier(applicationController, cr, namespace)
		if err != nil {
			return err
		}
	}
	return nil
}

// reconcileArgoApplier ensures that a specific rolebinding is present in a managedNamespace
func (r *ReconcileArgoCD) reconcileArgoApplier(controlPlaneServiceAccount string, cr *argoprojv1a1.ArgoCD, managedNamespace string) error {
	rbacClient := r.kc.RbacV1()

	roleBinding, err := rbacClient.RoleBindings(managedNamespace).Get(context.TODO(), cr.Name, metav1.GetOptions{})
	roleBindingExists := true
	if err != nil {
		if errors.IsNotFound(err) {
			roleBindingExists = false
			roleBinding = &v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cr.Name,
					Namespace: managedNamespace,
				},
			}
		} else {
			return err
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

	if roleBindingExists {
		_, err = rbacClient.RoleBindings(managedNamespace).Update(context.TODO(), roleBinding, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	} else {
		_, err = rbacClient.RoleBindings(managedNamespace).Create(context.TODO(), roleBinding, metav1.CreateOptions{})
	}
	return err
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
			Kind:      v1.ServiceAccountKind,
			Name:      name,
			Namespace: cr.Namespace,
		},
	}
	roleBinding.RoleRef = v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     "Role",
		Name:     name,
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
