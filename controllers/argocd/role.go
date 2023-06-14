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

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// newRole returns a new Role instance.
func newRole(name string, rules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) *v1.Role {
	return &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateResourceName(name, cr),
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
		Rules: rules,
	}
}

func newRoleForApplicationSourceNamespaces(namespace string, rules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) *v1.Role {
	return &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getRoleNameForApplicationSourceNamespaces(namespace, cr),
			Namespace: namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
		Rules: rules,
	}
}

func generateResourceName(argoComponentName string, cr *argoprojv1a1.ArgoCD) string {
	return cr.Name + "-" + argoComponentName
}

// GenerateUniqueResourceName generates unique names for cluster scoped resources
func GenerateUniqueResourceName(argoComponentName string, cr *argoprojv1a1.ArgoCD) string {
	return cr.Name + "-" + cr.Namespace + "-" + argoComponentName
}

func newClusterRole(name string, rules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) *v1.ClusterRole {
	return &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:        GenerateUniqueResourceName(name, cr),
			Labels:      argoutil.LabelsForCluster(cr),
			Annotations: argoutil.AnnotationsForCluster(cr),
		},
		Rules: rules,
	}
}

// reconcileRoles will ensure that all ArgoCD Service Accounts are configured.
func (r *ReconcileArgoCD) reconcileRoles(ctx context.Context, cr *argoprojv1a1.ArgoCD) error {
	params := getPolicyRuleList(r.Client)

	for _, param := range params {
		if err := r.reconcileRole(param.name, param.policyRule, cr); err != nil {
			return err
		}
	}

	clusterParams := getPolicyRuleClusterRoleList()

	for _, clusterParam := range clusterParams {
		if _, err := r.reconcileClusterRole(clusterParam.name, clusterParam.policyRule, cr); err != nil {
			return err
		}
	}

	log.Info("reconciling roles for source namespaces")
	policyRuleForApplicationSourceNamespaces := policyRuleForServerApplicationSourceNamespaces()
	// reconcile roles is source namespaces for ArgoCD Server
	if err := r.reconcileRoleForApplicationSourceNamespaces(ctx, common.ArgoCDServerComponent, policyRuleForApplicationSourceNamespaces, cr); err != nil {
		return err
	}

	log.Info("performing cleanup for source namespaces")
	// remove resources for namespaces not part of SourceNamespaces
	if err := r.removeUnmanagedSourceNamespaceResources(ctx, cr); err != nil {
		return err
	}

	return nil
}

// reconcileRole, reconciles the policy rules for different ArgoCD components, for each namespace
// Managed by a single instance of ArgoCD.
func (r *ReconcileArgoCD) reconcileRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) error {
	log.Info(fmt.Sprintf("Starting reconcileRole with name: %s", name))

	// create policy rules for each namespace
	for _, namespace := range r.ManagedNamespaces.Items {
		// If encountering a terminating namespace remove managed-by label from it and skip reconciliation - This should trigger
		// clean-up of roles/rolebindings and removal of namespace from cluster secret
		if namespace.DeletionTimestamp != nil {
			log.Info("Found terminating namespace. Removing managed-by label and skipping reconciliation.")
			if _, ok := namespace.Labels[common.ArgoCDManagedByLabel]; ok {
				delete(namespace.Labels, common.ArgoCDManagedByLabel)
				_ = r.Client.Update(context.TODO(), &namespace)
			}
			continue
		}

		list := &argoprojv1a1.ArgoCDList{}
		listOption := &client.ListOptions{Namespace: namespace.Name}
		err := r.Client.List(context.TODO(), list, listOption)
		if err != nil {
			return err
		}
		log.Info(fmt.Sprintf("ArgoCD list for namespace '%s' contains '%d' items", namespace.Name, len(list.Items)))

		// only skip creation of dex and redisHa roles for namespaces that no argocd instance is deployed in
		if len(list.Items) < 1 {
			// only create dexServer and redisHa roles for the namespace where the argocd instance is deployed
			if cr.ObjectMeta.Namespace != namespace.Name && (name == common.ArgoCDDexServerComponent || name == common.ArgoCDRedisHAComponent) {
				continue
			}
		}

		role := newRole(name, policyRules, cr)
		if err := applyReconcilerHook(cr, role, ""); err != nil {
			return err
		}
		role.Namespace = namespace.Name
		existingRole := v1.Role{}
		err = r.Client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, &existingRole)
		roleExists := true
		if err != nil {
			if !errors.IsNotFound(err) {
				return fmt.Errorf("failed to reconcile the role for the service account associated with %s : %s", name, err)
			}

			if name == common.ArgoCDDexServerComponent && !UseDex(cr) {
				log.Info("Dex installation not requested. Skipping role creation.")
				continue // Dex installation not requested, do nothing
			}

			roleExists = false
		}

		customRole := getCustomRoleName(name)

		if roleExists {
			// Delete the existing default role if custom role is specified
			// or if there is an existing Role created for Dex but dex is disabled or not configured
			if customRole != "" || name == common.ArgoCDDexServerComponent && !UseDex(cr) {
				log.Info("deleting the existing Dex role because dex is not configured")
				if err := r.Client.Delete(context.TODO(), &existingRole); err != nil {
					log.Error(err, fmt.Sprintf("Failed to delete existing role %s in namespace %s: %v", existingRole.Name, existingRole.Namespace, err))
					return err
				}
			}

			// if the Rules differ, update the Role
			if !reflect.DeepEqual(existingRole.Rules, role.Rules) {
				existingRole.Rules = role.Rules
				if err := r.Client.Update(context.TODO(), &existingRole); err != nil {
					log.Error(err, fmt.Sprintf("Failed to update role %s in namespace %s: %v", existingRole.Name, existingRole.Namespace, err))
					return err
				}
			}
			continue
		}

		if customRole != "" {
			log.Info(fmt.Sprintf("Custom role '%s' found. Skipping default role creation.", customRole))
			continue // skip creating default role if custom cluster role is provided
		}

		// Only set ownerReferences for roles in same namespace as ArgoCD CR
		if cr.Namespace == role.Namespace {
			if err = controllerutil.SetControllerReference(cr, role, r.Scheme); err != nil {
				return fmt.Errorf("failed to set ArgoCD CR \"%s\" as owner for role \"%s\": %s", cr.Name, role.Name, err)
			}
		}

		log.Info(fmt.Sprintf("creating role %s for Argo CD instance %s in namespace %s", role.Name, cr.Name, cr.Namespace))
		if err := r.Client.Create(context.TODO(), role); err != nil {
			log.Error(err, fmt.Sprintf("Failed to create role %s in namespace %s: %v", role.Name, role.Namespace, err))
			return err
		}
		log.Info(fmt.Sprintf("Role %s created successfully in namespace %s", role.Name, role.Namespace))

	}
	return nil
}

func (r *ReconcileArgoCD) reconcileRoleForApplicationSourceNamespaces(ctx context.Context, name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) error {
	// create policy rules for each source namespace for ArgoCD Server
	for _, sourceNamespace := range cr.Spec.SourceNamespaces {

		namespace := &corev1.Namespace{}
		if err := r.Client.Get(ctx, types.NamespacedName{Name: sourceNamespace}, namespace); err != nil {
			return err
		}

		value, ok := namespace.Labels[common.ArgoCDManagedByLabel]
		// do not reconcile roles for namespaces already containing managed-by label
		// as it already contains roles with permissions to manipulate application resources
		// reconciled during reconcilation of ManagedNamespaces
		if ok && value != "" {
			log.Info(fmt.Sprintf("Skipping reconciling resources for namespace %s as it is already managed-by namespace %s.", namespace.Name, value))
			// if managed-by-cluster-argocd label is also present, remove the namespace from the ManagedSourceNamespaces.
			val, ok1 := namespace.Labels[common.ArgoCDManagedByClusterArgoCDLabel]
			if ok1 && val == cr.Namespace {
				delete(r.ManagedSourceNamespaces, namespace.Name)
				if err := r.cleanupUnmanagedSourceNamespaceResources(ctx, cr, namespace.Name); err != nil {
					log.Error(err, fmt.Sprintf("error cleaning up resources for namespace %s", namespace.Name))
				}
			}
		} else {
			log.Info(fmt.Sprintf("Reconciling role for %s", namespace.Name))

			role := newRoleForApplicationSourceNamespaces(namespace.Name, policyRules, cr)
			if err := applyReconcilerHook(cr, role, ""); err != nil {
				return err
			}
			role.Namespace = namespace.Name
			existingRole := v1.Role{}
			err := r.Client.Get(ctx, types.NamespacedName{Name: role.Name, Namespace: namespace.Name}, &existingRole)
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to reconcile the role for the service account associated with %s : %s", name, err)
			}

			// do not reconcile roles for namespaces already containing managed-by-cluster-argocd label
			// as it already contains roles reconciled during reconcilation of ManagedNamespaces
			if _, ok := r.ManagedSourceNamespaces[sourceNamespace]; ok {
				// If sourceNamespace includes the name but role is missing in the namespace, create the role
				if reflect.DeepEqual(existingRole, v1.Role{}) {
					log.Info(fmt.Sprintf("creating role %s for Argo CD instance %s in namespace %s", role.Name, cr.Name, namespace))
					if err := r.Client.Create(ctx, role); err != nil {
						return err
					}
				}
				continue
			}

			// reconcile roles only if another ArgoCD instance is not already set as value for managed-by-cluster-argocd label
			if value, ok := namespace.Labels[common.ArgoCDManagedByClusterArgoCDLabel]; ok && value != "" {
				log.Info(fmt.Sprintf("Namespace already has label set to argocd instance %s. Thus, skipping namespace %s", value, namespace.Name))
				continue
			}

			// Get the latest value of namespace before updating it
			if err := r.Client.Get(ctx, types.NamespacedName{Name: namespace.Name}, namespace); err != nil {
				return err
			}
			// Update namespace with managed-by-cluster-argocd label
			namespace.Labels[common.ArgoCDManagedByClusterArgoCDLabel] = cr.Namespace
			if err := r.Client.Update(ctx, namespace); err != nil {
				log.Error(err, fmt.Sprintf("failed to add label from namespace [%s]", namespace.Name))
			}
			// if the Rules differ, update the Role
			if !reflect.DeepEqual(existingRole.Rules, role.Rules) {
				existingRole.Rules = role.Rules
				if err := r.Client.Update(ctx, &existingRole); err != nil {
					return err
				}
			}

			if _, ok := r.ManagedSourceNamespaces[sourceNamespace]; !ok {
				r.ManagedSourceNamespaces[sourceNamespace] = ""
			}
		}
	}
	return nil
}

func (r *ReconcileArgoCD) reconcileClusterRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) (*v1.ClusterRole, error) {
	allowed := false
	if allowedNamespace(cr.Namespace, os.Getenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")) {
		allowed = true
	}
	clusterRole := newClusterRole(name, policyRules, cr)
	if err := applyReconcilerHook(cr, clusterRole, ""); err != nil {
		return nil, err
	}

	existingClusterRole := &v1.ClusterRole{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRole.Name}, existingClusterRole)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the cluster role for the service account associated with %s : %s", name, err)
		}
		if !allowed {
			// Do Nothing
			return nil, nil
		}
		return clusterRole, r.Client.Create(context.TODO(), clusterRole)
	}

	if !allowed {
		return nil, r.Client.Delete(context.TODO(), existingClusterRole)
	}

	// if the Rules differ, update the Role
	if !reflect.DeepEqual(existingClusterRole.Rules, clusterRole.Rules) {
		existingClusterRole.Rules = clusterRole.Rules
		if err := r.Client.Update(context.TODO(), existingClusterRole); err != nil {
			return nil, err
		}
	}
	return existingClusterRole, nil
}

func deleteClusterRoles(c client.Client, clusterRoleList *v1.ClusterRoleList) error {
	for _, clusterRole := range clusterRoleList.Items {
		if err := c.Delete(context.TODO(), &clusterRole); err != nil {
			return fmt.Errorf("failed to delete ClusterRole %q during cleanup: %w", clusterRole.Name, err)
		}
	}
	return nil
}
