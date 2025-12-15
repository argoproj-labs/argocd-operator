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

	labels := roleBinding.Labels
	labels[common.ArgoCDKeyName] = name
	roleBinding.Labels = labels

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

// newRoleBindingWithname creates a new RoleBinding with the given name for the given ArgCD.
func newRoleBindingWithname(name string, cr *argoproj.ArgoCD) *v1.RoleBinding {
	roleBinding := newRoleBinding(cr)

	// Truncate the name to stay within 63 character limit
	fullName := fmt.Sprintf("%s-%s", cr.Name, name)
	roleBinding.Name = argoutil.TruncateWithHash(fullName, argoutil.GetMaxLabelLength())

	labels := roleBinding.Labels
	labels[common.ArgoCDKeyName] = name
	roleBinding.Labels = labels

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
				argoutil.LogResourceUpdate(log, &namespace, "namespace is terminating, removing 'managed-by' label")
				_ = r.Update(context.TODO(), &namespace)
			}
			continue
		}

		list := &argoproj.ArgoCDList{}
		listOption := &client.ListOptions{Namespace: namespace.Name}
		err := r.List(context.TODO(), list, listOption)
		if err != nil {
			return err
		}
		// only skip creation of dex and redisHa rolebindings for namespaces that no argocd instance is deployed in
		if len(list.Items) < 1 {
			// namespace doesn't contain argocd instance, so skipe all the ArgoCD internal roles
			if cr.Namespace != namespace.Name && (name != common.ArgoCDApplicationControllerComponent && name != common.ArgoCDServerComponent) {
				continue
			}
		}
		// get expected name
		roleBinding := newRoleBindingWithname(name, cr)
		roleBinding.Namespace = namespace.Name

		// fetch existing rolebinding by name
		existingRoleBinding := &v1.RoleBinding{}
		err = r.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: roleBinding.Namespace}, existingRoleBinding)
		roleBindingExists := true
		if err != nil {
			if !errors.IsNotFound(err) {
				return fmt.Errorf("failed to get the rolebinding associated with %s : %s", name, err)
			}

			if (name == common.ArgoCDDexServerComponent && !UseDex(cr)) ||
				!UseApplicationController(name, cr) || !UseRedis(name, cr) || !UseServer(name, cr) {
				continue // Component installation is not requested, do nothing
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
			if (name == common.ArgoCDDexServerComponent && !UseDex(cr)) || !UseApplicationController(name, cr) || !UseRedis(name, cr) || !UseServer(name, cr) {
				// Delete any existing RoleBinding created for Dex since dex uninstallation is requested
				argoutil.LogResourceDeletion(log, existingRoleBinding, "dex is being uninstalled")
				if err = r.Delete(context.TODO(), existingRoleBinding); err != nil {
					return err
				}
				continue
			}

			// if the RoleRef changes, delete the existing role binding and create a new one
			if !reflect.DeepEqual(roleBinding.RoleRef, existingRoleBinding.RoleRef) {
				argoutil.LogResourceDeletion(log, existingRoleBinding, "role ref changed, deleting role binding in order to recreate it")
				if err = r.Delete(context.TODO(), existingRoleBinding); err != nil {
					return err
				}
			} else {
				// if the Subjects differ, update the role bindings
				if !reflect.DeepEqual(roleBinding.Subjects, existingRoleBinding.Subjects) {
					existingRoleBinding.Subjects = roleBinding.Subjects
					argoutil.LogResourceUpdate(log, existingRoleBinding, "updating subjects")
					if err = r.Update(context.TODO(), existingRoleBinding); err != nil {
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

		argoutil.LogResourceCreation(log, roleBinding)
		if err = r.Create(context.TODO(), roleBinding); err != nil {
			return err
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

func deleteClusterRoleBindings(c client.Client, clusterBindingList *v1.ClusterRoleBindingList) error {
	for _, clusterBinding := range clusterBindingList.Items {
		argoutil.LogResourceDeletion(log, &clusterBinding, "cleaning up cluster resources")
		if err := c.Delete(context.TODO(), &clusterBinding); err != nil {
			return fmt.Errorf("failed to delete ClusterRoleBinding %q during cleanup: %w", clusterBinding.Name, err)
		}
	}
	return nil
}
