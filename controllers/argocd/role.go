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
	for _, sourceNamespace := range cr.Spec.SourceNamespaces {

		namespace := &corev1.Namespace{}
		if err := r.Client.Get(ctx, types.NamespacedName{Name: sourceNamespace}, namespace); err != nil {
			return err
		}

		managedByLabelval, isManagedByLabelPresent := namespace.Labels[common.ArgoCDManagedByLabel]
		managedByClusterArgocdVal, isManagedByClusterArgoCDLabelPresent := namespace.Labels[common.ArgoCDManagedByClusterArgoCDLabel]

		// If namespace has managed-by label, skip reconciling roles for namespace as it already contains roles with permissions to
		// manipulate application resources reconciled in the reconcilation of ManagedNamespaces
		if isManagedByLabelPresent && managedByLabelval != "" {
			log.Info("Namespace is already managed by another namespace, skipping", "namespace", namespace.Name, "managedBy", managedByLabelval)
			// If namespace also has managed-by-cluster-argocd label and matches Argo CD instance, clean up the resources
			if isManagedByClusterArgoCDLabelPresent && managedByClusterArgocdVal == cr.Namespace {
				// Remove the namespace from ManagedSourceNamespace
				delete(r.ManagedSourceNamespaces, namespace.Name)
				if err := r.cleanupUnmanagedSourceNamespaceResources(ctx, cr, namespace.Name); err != nil {
					log.Error(err, "Error cleaning up resources", "namespace", namespace.Name)
				}
			}

			// Skip the rest of the loop and continue with the next namespace
			continue
		}

		log.Info("Reconciling role", "namespace", namespace.Name)

		expectedRole := newRoleForApplicationSourceNamespaces(namespace.Name, policyRules, cr)
		if err := applyReconcilerHook(cr, expectedRole, ""); err != nil {
			return fmt.Errorf("failed to apply reconciler hook for %s: %w", name, err)
		}
		expectedRole.Namespace = namespace.Name

		// Check if the namespace is managed by current ArgoCD instance with managed-by-cluster-argocd label
		if _, ok := r.ManagedSourceNamespaces[sourceNamespace]; ok {
			// If it's managed by current ArgoCD instance, create role if does not exist
			if err := r.createRoleIfNotExists(ctx, expectedRole, namespace.Name, cr.Name); err != nil {
				return err
			}
			// Continue to the next sourceNamespace in the loop
			continue
		}

		// Check if another ArgoCD instance is already set as value of managed-by-cluster-argocd label
		if isManagedByClusterArgoCDLabelPresent && managedByClusterArgocdVal != "" {
			log.Info("Namespace already has managed-by-cluster-argocd label set to another ArgoCD instance, skipping", "namespace", namespace.Name, "ArgoCD", managedByClusterArgocdVal)
			continue
		}

		// Create/Update role and update the namespace label after successfully reconciled role
		if err := r.createOrUpdateRole(ctx, expectedRole, namespace, cr); err != nil {
			return err
		}

		// Save sourceNamespace to ManagedSourceNamespaces map as sourceNamespace label has been updated successfully
		if _, ok := r.ManagedSourceNamespaces[sourceNamespace]; !ok {
			r.ManagedSourceNamespaces[sourceNamespace] = ""
		}
	}
	return nil
}

func (r *ReconcileArgoCD) createRoleIfNotExists(ctx context.Context, role *v1.Role, namespace, crName string) error {
	existingRole := &v1.Role{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: role.Name, Namespace: namespace}, existingRole)
	switch {
	case err == nil:
		log.Info("Role already exists, skipping creation", "role", role.Name, "namespace", namespace)
	case errors.IsNotFound(err):
		log.Info("Creating new role", "role", role.Name, "namespace", namespace, "ArgoCD", crName)
		if err := r.Client.Create(ctx, role); err != nil {
			log.Error(err, "Failed to create role", "role", role.Name, "namespace", namespace)
			return err
		}
	default:
		log.Error(err, "Failed to get role", "role", role.Name, "namespace", namespace)
		return err
	}
	return nil
}

func (r *ReconcileArgoCD) createOrUpdateRole(ctx context.Context, role *v1.Role, namespace *corev1.Namespace, cr *argoprojv1a1.ArgoCD) error {
	existingRole := &v1.Role{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: role.Name, Namespace: namespace.Name}, existingRole)
	switch {
	case err == nil:
		if !reflect.DeepEqual(existingRole.Rules, role.Rules) {
			existingRole.Rules = role.Rules
			if err := r.Client.Update(ctx, existingRole); err != nil {
				log.Error(err, "Failed to update role", "role", role.Name, "namespace", namespace.Name)
				return err
			}
			log.Info("Role rules updated successfully", "role", role.Name, "namespace", namespace.Name)
		}
	case errors.IsNotFound(err):
		log.Info("Creating new role", "role", role.Name, "namespace", namespace.Name, "ArgoCD", cr.Name)
		if err := r.Client.Create(ctx, role); err != nil {
			log.Error(err, "Failed to create role", "role", role.Name, "namespace", namespace.Name)
			return err
		}
	default:
		log.Error(err, "Failed to get role", "role", role.Name, "namespace", namespace.Name)
		return err
	}

	// Update namespace with managed-by-cluster-argocd label as required role has been successfully created/updated
	namespace.Labels[common.ArgoCDManagedByClusterArgoCDLabel] = cr.Namespace
	if err := r.Client.Update(ctx, namespace); err != nil {
		log.Error(err, "Failed to update namespace label", "namespace", namespace.Name)
		return err
	}
	log.Info("Namespace label updated successfully", "namespace", namespace.Name)
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
