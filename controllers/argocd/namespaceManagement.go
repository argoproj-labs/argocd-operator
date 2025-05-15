package argocd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/argoproj/argo-cd/v2/util/glob"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
)

// reconcileNamespaceManagement ensures that ArgoCD managed namespaces are properly tracked
// and updated based on the NamespaceManagement CRs. It identifies the namespaces managed by
// ArgoCD, validates them against the management rules, and updates their status accordingly.
func (r *ReconcileArgoCD) reconcileNamespaceManagement(argocd *argoproj.ArgoCD) error {
	log.Info("Reconciling NamespaceManagement")
	ctx := context.TODO()

	var errorMessages []string
	var managedNamespaces []corev1.Namespace

	// List all NamespaceManagement CRs
	nmList := &argoproj.NamespaceManagementList{}
	if err := r.Client.List(ctx, nmList); err != nil {
		return fmt.Errorf("failed to list NamespaceManagement resources: %w", err)
	}

	// Convert NamespaceManagement spec into a lookup map
	allowedNamespaces := make(map[string]bool)
	if argocd.Spec.NamespaceManagement != nil {
		for _, nm := range argocd.Spec.NamespaceManagement {
			allowedNamespaces[nm.Name] = nm.AllowManagedBy
		}
	}

	// Store conditions to update at the end
	type nmStatus struct {
		nm      argoproj.NamespaceManagement
		message string
	}
	var statusUpdates []nmStatus

	// Process each NamespaceManagement CR
	for _, nm := range nmList.Items {
		namespace := nm.Namespace
		var message string

		if nm.Spec.ManagedBy != argocd.Namespace {
			log.Info("Skipping NamespaceManagement CR as it targets a different ArgoCD instance", "namespace", namespace)
			continue
		}

		// Check if the namespace is explicitly disallowed (allowManagedBy: false)
		allowed, exists := allowedNamespaces[namespace]
		if exists && !allowed {
			message = fmt.Sprintf("Namespace %s is not allowed to be managed by ArgoCD", namespace)
			errorMessages = append(errorMessages, message)
			continue
		}

		// Validate namespace management rules
		if nm.Spec.ManagedBy == argocd.Namespace && matchesNamespaceManagementRules(argocd, namespace) {
			managedNamespaces = append(managedNamespaces, corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: namespace},
			})
		} else {
			message = fmt.Sprintf("Namespace %s is not permitted for management by ArgoCD instance %s based on NamespaceManagement rules", namespace, argocd.Namespace)
			errorMessages = append(errorMessages, message)
		}

		statusUpdates = append(statusUpdates, nmStatus{nm: nm, message: message})
	}

	// Always include the ArgoCD namespace
	managedNamespaces = append(managedNamespaces, corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: argocd.Namespace},
	})

	// Initializing to avoid panics
	if r.ManagedNamespaces == nil {
		r.ManagedNamespaces = &corev1.NamespaceList{}
	}

	// Avoid duplicates before appending to r.ManagedNamespaces.Items
	existingNamespaces := make(map[string]bool)

	for _, ns := range r.ManagedNamespaces.Items {
		existingNamespaces[ns.Name] = true
	}

	for _, ns := range managedNamespaces {
		if !existingNamespaces[ns.Name] {
			r.ManagedNamespaces.Items = append(r.ManagedNamespaces.Items, ns)
			existingNamespaces[ns.Name] = true
		}
	}

	// update status conditions once for each NamespaceManagement CR
	for _, update := range statusUpdates {
		if err := updateStatusConditionOfNamespaceManagement(ctx, createCondition(update.message), &update.nm, r.Client, log); err != nil {
			log.Error(err, "Failed to update status of NamespaceManagement CR", "namespace", update.nm.Namespace)
			errorMessages = append(errorMessages, fmt.Sprintf("status update failed for namespace %s: %v", update.nm.Namespace, err))
		}
	}

	// Return aggregated errors, if any
	if len(errorMessages) > 0 {
		return fmt.Errorf("namespace management errors: %s", strings.Join(errorMessages, "; "))
	}
	return nil
}

// Helper function to check if a namespace matches ArgoCD namespace management rules
func matchesNamespaceManagementRules(argocd *argoproj.ArgoCD, namespace string) bool {
	if argocd.Spec.NamespaceManagement == nil {
		return false
	}

	var allowedPatterns []string
	for _, managedNs := range argocd.Spec.NamespaceManagement {
		if managedNs.AllowManagedBy {
			allowedPatterns = append(allowedPatterns, managedNs.Name) // Collect name patterns only if allowed
		}
	}

	return glob.MatchStringInList(allowedPatterns, namespace, glob.GLOB)
}

// Check if namespace management is explicitly enabled via Subscription
func isNamespaceManagementEnabled() bool {
	return os.Getenv(common.EnableManagedNamespace) == "true"
}

// If the EnableManagedNamespace feature is disabled, clean up the RBACs associated with the managed namespaces
// and remove the corresponding fields from the ArgoCD and NamespaceManagement CRs.
func (r *ReconcileArgoCD) disableNamespaceManagement(argocd *argoproj.ArgoCD, k8sClient kubernetes.Interface) error {
	ctx := context.TODO()

	// List all NamespaceManagement CRs
	nsMgmtList := &argoproj.NamespaceManagementList{}
	if err := r.Client.List(ctx, nsMgmtList); err != nil {
		return err
	}

	// Build a lookup of namespaces managed by this ArgoCD instance
	managedNamespaces := map[string]bool{}
	for _, nsMgmt := range nsMgmtList.Items {
		if nsMgmt.Spec.ManagedBy == argocd.Namespace {
			managedNamespaces[nsMgmt.Namespace] = true
		}
	}

	// Only act on namespaces in ArgoCD.spec.namespaceManagement that are truly managed by this ArgoCD
	for _, ns := range argocd.Spec.NamespaceManagement {
		nsName := ns.Name
		if !managedNamespaces[nsName] {
			log.Info(fmt.Sprintf("Skipping namespace %s: not managed by this ArgoCD instance or NamespaceManagement CR missing", nsName))
			continue
		}

		// Check if the namespace has a "managed-by" label
		namespace := &corev1.Namespace{}
		if err := r.Client.Get(ctx, types.NamespacedName{Name: nsName}, namespace); err != nil {
			log.Error(err, fmt.Sprintf("unable to fetch namespace %s", nsName))
			return err
		}

		// Skip RBAC deletion if the namespace has the "managed-by" label
		if namespace.Labels[common.ArgoCDManagedByLabel] == nsName {
			log.Info(fmt.Sprintf("Skipping RBAC deletion for namespace %s due to managed-by label", nsName))
			continue
		}

		if ns.AllowManagedBy {
			if err := deleteRBACsForNamespace(nsName, k8sClient); err != nil {
				log.Error(err, fmt.Sprintf("Failed to delete RBACs for namespace: %s", nsName))
				return err
			}
			log.Info(fmt.Sprintf("Successfully removed RBACs for namespace: %s", nsName))

			if err := deleteManagedNamespaceFromClusterSecret(argocd.Namespace, nsName, k8sClient); err != nil {
				log.Error(err, fmt.Sprintf("Unable to delete namespace %s from cluster secret", nsName))
				return err
			}
		}
	}
	return nil
}
