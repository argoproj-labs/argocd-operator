package argocd

import (
	"context"
	"fmt"
	"os"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// newClusterRoleBinding returns a new ClusterRoleBinding instance.
func newClusterRoleBinding(cr *argoproj.ArgoCD) *v1.ClusterRoleBinding {
	return &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cr.Name,
			Labels:      argoutil.LabelsForCluster(cr),
			Annotations: argoutil.AnnotationsForCluster(cr),
		},
	}
}

// newClusterRoleBindingWithname creates a new ClusterRoleBinding with the given name for the given ArgCD.
func newClusterRoleBindingWithname(name string, cr *argoproj.ArgoCD) *v1.ClusterRoleBinding {
	roleBinding := newClusterRoleBinding(cr)
	roleBinding.Name = GenerateUniqueResourceName(name, cr)

	labels := roleBinding.ObjectMeta.Labels
	labels[common.ArgoCDKeyName] = name
	roleBinding.ObjectMeta.Labels = labels

	return roleBinding
}

// newRoleBinding returns a new RoleBinding instance.
func newRoleBinding(cr *argoproj.ArgoCD) *v1.RoleBinding {
	return &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cr.Name,
			Labels:      argoutil.LabelsForCluster(cr),
			Annotations: argoutil.AnnotationsForCluster(cr),
			Namespace:   cr.Namespace,
		},
	}
}

// newRoleBindingForSupportNamespaces returns a new RoleBinding instance.
func newRoleBindingForSupportNamespaces(cr *argoproj.ArgoCD, namespace string) *v1.RoleBinding {
	return &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        getRoleBindingNameForSourceNamespaces(cr.Name, namespace),
			Labels:      argoutil.LabelsForCluster(cr),
			Annotations: argoutil.AnnotationsForCluster(cr),
			Namespace:   namespace,
		},
	}
}

func getRoleBindingNameForSourceNamespaces(argocdName, targetNamespace string) string {
	return fmt.Sprintf("%s_%s", argocdName, targetNamespace)
}

// newRoleBindingWithname creates a new RoleBinding with the given name for the given ArgCD.
func newRoleBindingWithname(name string, cr *argoproj.ArgoCD) *v1.RoleBinding {
	roleBinding := newRoleBinding(cr)
	roleBinding.ObjectMeta.Name = fmt.Sprintf("%s-%s", cr.Name, name)

	labels := roleBinding.ObjectMeta.Labels
	labels[common.ArgoCDKeyName] = name
	roleBinding.ObjectMeta.Labels = labels

	return roleBinding
}

// reconcileRoleBindings will ensure that all ArgoCD RoleBindings are configured.
func (r *ReconcileArgoCD) reconcileRoleBindings(cr *argoproj.ArgoCD) error {
	params := getPolicyRuleList(r.Client)

	for _, param := range params {
		if err := r.reconcileRoleBinding(param.name, param.policyRule, cr); err != nil {
			return fmt.Errorf("error reconciling roleBinding for %q: %w", param.name, err)
		}
	}

	return nil
}

// reconcileRoleBinding, creates RoleBindings for every role and associates it with the right ServiceAccount.
// This would create RoleBindings for all the namespaces managed by the ArgoCD instance.
func (r *ReconcileArgoCD) reconcileRoleBinding(name string, rules []v1.PolicyRule, cr *argoproj.ArgoCD) error {
	var sa *corev1.ServiceAccount
	var error error

	if sa, error = r.reconcileServiceAccount(name, cr); error != nil {
		return error
	}

	if _, error = r.reconcileRole(name, rules, cr); error != nil {
		return error
	}

	for _, namespace := range r.ManagedNamespaces.Items {
		// If encountering a terminating namespace remove managed-by label from it and skip reconciliation - This should trigger
		// clean-up of roles/rolebindings and removal of namespace from cluster secret
		if namespace.DeletionTimestamp != nil {
			if _, ok := namespace.Labels[common.ArgoCDManagedByLabel]; ok {
				delete(namespace.Labels, common.ArgoCDManagedByLabel)
				_ = r.Client.Update(context.TODO(), &namespace)
			}
			continue
		}

		list := &argoproj.ArgoCDList{}
		listOption := &client.ListOptions{Namespace: namespace.Name}
		err := r.Client.List(context.TODO(), list, listOption)
		if err != nil {
			return err
		}
		// only skip creation of dex and redisHa rolebindings for namespaces that no argocd instance is deployed in
		if len(list.Items) < 1 {
			// namespace doesn't contain argocd instance, so skipe all the ArgoCD internal roles
			if cr.ObjectMeta.Namespace != namespace.Name && (name != common.ArgoCDApplicationControllerComponent && name != common.ArgoCDServerComponent) {
				continue
			}
		}
		// get expected name
		roleBinding := newRoleBindingWithname(name, cr)
		roleBinding.Namespace = namespace.Name

		// fetch existing rolebinding by name
		existingRoleBinding := &v1.RoleBinding{}
		err = r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: roleBinding.Namespace}, existingRoleBinding)
		roleBindingExists := true
		if err != nil {
			if !errors.IsNotFound(err) {
				return fmt.Errorf("failed to get the rolebinding associated with %s : %s", name, err)
			}

			if name == common.ArgoCDDexServerComponent && !UseDex(cr) {
				continue // Dex installation is not requested, do nothing
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

		customRoleName := getCustomRoleName(name)
		if customRoleName != "" {
			roleBinding.RoleRef = v1.RoleRef{
				APIGroup: v1.GroupName,
				Kind:     "ClusterRole",
				Name:     customRoleName,
			}
		} else {
			roleBinding.RoleRef = v1.RoleRef{
				APIGroup: v1.GroupName,
				Kind:     "Role",
				Name:     generateResourceName(name, cr),
			}
		}

		if roleBindingExists {
			if name == common.ArgoCDDexServerComponent && !UseDex(cr) {
				// Delete any existing RoleBinding created for Dex since dex uninstallation is requested
				log.Info("deleting the existing Dex roleBinding because dex uninstallation is requested")
				if err = r.Client.Delete(context.TODO(), existingRoleBinding); err != nil {
					return err
				}
				continue
			}

			// if the RoleRef changes, delete the existing role binding and create a new one
			if !reflect.DeepEqual(roleBinding.RoleRef, existingRoleBinding.RoleRef) {
				if err = r.Client.Delete(context.TODO(), existingRoleBinding); err != nil {
					return err
				}
			} else {
				// if the Subjects differ, update the role bindings
				if !reflect.DeepEqual(roleBinding.Subjects, existingRoleBinding.Subjects) {
					existingRoleBinding.Subjects = roleBinding.Subjects
					if err = r.Client.Update(context.TODO(), existingRoleBinding); err != nil {
						return err
					}
				}
				continue
			}
		}

		// Only set ownerReferences for role bindings in same namespaces as Argo CD CR
		if cr.Namespace == roleBinding.Namespace {
			if err = controllerutil.SetControllerReference(cr, roleBinding, r.Scheme); err != nil {
				return fmt.Errorf("failed to set ArgoCD CR \"%s\" as owner for roleBinding \"%s\": %s", cr.Name, roleBinding.Name, err)
			}
		}

		log.Info(fmt.Sprintf("creating rolebinding %s for Argo CD instance %s in namespace %s", roleBinding.Name, cr.Name, cr.Namespace))
		if err = r.Client.Create(context.TODO(), roleBinding); err != nil {
			return err
		}
	}

	// reconcile rolebindings only for ArgoCDServerComponent
	if name == common.ArgoCDServerComponent {

		// reconcile rolebindings for all source namespaces for argocd-server
		for _, sourceNamespace := range cr.Spec.SourceNamespaces {
			namespace := &corev1.Namespace{}
			if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: sourceNamespace}, namespace); err != nil {
				return err
			}

			// do not reconcile rolebindings for namespaces already containing managed-by label
			// as it already contains rolebindings with permissions to manipulate application resources
			// reconciled during reconcilation of ManagedNamespaces
			if value, ok := namespace.Labels[common.ArgoCDManagedByLabel]; ok {
				log.Info(fmt.Sprintf("Skipping reconciling resources for namespace %s as it is already managed-by namespace %s.", namespace.Name, value))
				continue
			}

			list := &argoproj.ArgoCDList{}
			listOption := &client.ListOptions{Namespace: namespace.Name}
			err := r.Client.List(context.TODO(), list, listOption)
			if err != nil {
				log.Info(err.Error())
				return err
			}

			// get expected name
			roleBinding := newRoleBindingWithNameForApplicationSourceNamespaces(namespace.Name, cr)
			roleBinding.Namespace = namespace.Name

			roleBinding.RoleRef = v1.RoleRef{
				APIGroup: v1.GroupName,
				Kind:     "Role",
				Name:     getRoleNameForApplicationSourceNamespaces(namespace.Name, cr),
			}

			// fetch existing rolebinding by name
			existingRoleBinding := &v1.RoleBinding{}
			err = r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: roleBinding.Namespace}, existingRoleBinding)
			roleBindingExists := true
			if err != nil {
				if !errors.IsNotFound(err) {
					return fmt.Errorf("failed to get the rolebinding associated with %s : %s", name, err)
				}
				log.Info(fmt.Sprintf("Existing rolebinding %s", err.Error()))
				roleBindingExists = false
			}

			roleBinding.Subjects = []v1.Subject{
				{
					Kind:      v1.ServiceAccountKind,
					Name:      getServiceAccountName(cr.Name, common.ArgoCDServerComponent),
					Namespace: sa.Namespace,
				},
				{
					Kind:      v1.ServiceAccountKind,
					Name:      getServiceAccountName(cr.Name, common.ArgoCDApplicationControllerComponent),
					Namespace: sa.Namespace,
				},
			}

			if roleBindingExists {
				// reconcile role bindings for namespaces already containing managed-by-cluster-argocd label only
				if n, ok := namespace.Labels[common.ArgoCDManagedByClusterArgoCDLabel]; !ok || n == cr.Namespace {
					continue
				}
				// if the RoleRef changes, delete the existing role binding and create a new one
				if !reflect.DeepEqual(roleBinding.RoleRef, existingRoleBinding.RoleRef) {
					if err = r.Client.Delete(context.TODO(), existingRoleBinding); err != nil {
						return err
					}
				} else {
					// if the Subjects differ, update the role bindings
					if !reflect.DeepEqual(roleBinding.Subjects, existingRoleBinding.Subjects) {
						existingRoleBinding.Subjects = roleBinding.Subjects
						if err = r.Client.Update(context.TODO(), existingRoleBinding); err != nil {
							return err
						}
					}
					continue
				}
			}

			log.Info(fmt.Sprintf("creating rolebinding %s for Argo CD instance %s in namespace %s", roleBinding.Name, cr.Name, namespace))
			if err = r.Client.Create(context.TODO(), roleBinding); err != nil {
				return err
			}
		}
	}
	return nil
}

func getCustomRoleName(name string) string {
	if name == common.ArgoCDApplicationControllerComponent {
		return os.Getenv(common.ArgoCDControllerClusterRoleEnvName)
	}
	if name == common.ArgoCDServerComponent {
		return os.Getenv(common.ArgoCDServerClusterRoleEnvName)
	}
	return ""
}

// Returns the name of the role for the source namespaces for ArgoCDServer in the format of "sourceNamespace_targetNamespace_argocd-server"
func getRoleNameForApplicationSourceNamespaces(targetNamespace string, cr *argoproj.ArgoCD) string {
	return fmt.Sprintf("%s_%s", cr.Name, targetNamespace)
}

// newRoleBindingWithNameForApplicationSourceNamespaces creates a new RoleBinding with the given name for the source namespaces of ArgoCD Server.
func newRoleBindingWithNameForApplicationSourceNamespaces(namespace string, cr *argoproj.ArgoCD) *v1.RoleBinding {
	roleBinding := newRoleBindingForSupportNamespaces(cr, namespace)

	labels := roleBinding.ObjectMeta.Labels
	labels[common.ArgoCDKeyName] = roleBinding.ObjectMeta.Name
	roleBinding.ObjectMeta.Labels = labels

	return roleBinding
}

func (r *ReconcileArgoCD) reconcileClusterRoleBinding(name string, role *v1.ClusterRole, cr *argoproj.ArgoCD) error {

	// get expected name
	roleBinding := newClusterRoleBindingWithname(name, cr)
	// fetch existing rolebinding by name
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name}, roleBinding)
	roleBindingExists := true
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		roleBindingExists = false
		roleBinding = newClusterRoleBindingWithname(name, cr)
	}

	if roleBindingExists && role == nil {
		return r.Client.Delete(context.TODO(), roleBinding)
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

	if cr.Namespace == roleBinding.Namespace {
		if err = controllerutil.SetControllerReference(cr, roleBinding, r.Scheme); err != nil {
			return fmt.Errorf("failed to set ArgoCD CR \"%s\" as owner for roleBinding \"%s\": %s", cr.Name, roleBinding.Name, err)
		}
	}

	if roleBindingExists {
		return r.Client.Update(context.TODO(), roleBinding)
	}
	return r.Client.Create(context.TODO(), roleBinding)
}

func deleteClusterRoleBindings(c client.Client, clusterBindingList *v1.ClusterRoleBindingList) error {
	for _, clusterBinding := range clusterBindingList.Items {
		if err := c.Delete(context.TODO(), &clusterBinding); err != nil {
			return fmt.Errorf("failed to delete ClusterRoleBinding %q during cleanup: %w", clusterBinding.Name, err)
		}
	}
	return nil
}
