package argocd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/argoproj/argo-cd/v3/util/glob"
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
	if err := r.List(ctx, nmList); err != nil {
		return fmt.Errorf("failed to list NamespaceManagement resources: %w", err)
	}

	// Extract allowed patterns from ArgoCD spec (only those allowed to be managed)
	var allowedNsPatterns []string
	if argocd.Spec.NamespaceManagement != nil {
		for _, nm := range argocd.Spec.NamespaceManagement {
			if nm.AllowManagedBy {
				allowedNsPatterns = append(allowedNsPatterns, nm.Name)
			}
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

		if matchesNamespaceManagementRules(allowedNsPatterns, namespace) {
			managedNamespaces = append(managedNamespaces, corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: namespace},
			})
		} else {
			message = fmt.Sprintf("Namespace %s is not permitted for management by ArgoCD instance %s based on NamespaceManagement rules", namespace, argocd.Namespace)
			errorMessages = append(errorMessages, message)
			statusUpdates = append(statusUpdates, nmStatus{nm: nm, message: message})
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
func matchesNamespaceManagementRules(allowedPatterns []string, namespace string) bool {
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
	if err := r.List(ctx, nsMgmtList); err != nil {
		return err
	}

	// Build a list of namespaces managed by this ArgoCD instance
	var managedNamespaces []string
	for _, nsMgmt := range nsMgmtList.Items {
		if nsMgmt.Spec.ManagedBy == argocd.Namespace {
			managedNamespaces = append(managedNamespaces, nsMgmt.Namespace)
		}
	}

	// For each pattern in ArgoCD.spec.NamespaceManagement, find matching namespaces and process them
	for _, ns := range argocd.Spec.NamespaceManagement {
		nsPattern := ns.Name
		allowManaged := ns.AllowManagedBy

		// Find all actual namespaces that match the pattern
		var matchedNamespaces []string
		for _, actualNs := range managedNamespaces {
			if glob.MatchStringInList([]string{nsPattern}, actualNs, glob.GLOB) {
				matchedNamespaces = append(matchedNamespaces, actualNs)
			}
		}

		if len(matchedNamespaces) == 0 {
			log.Info(fmt.Sprintf("Skipping namespace %s: not managed by this ArgoCD instance or NamespaceManagement CR missing", nsPattern))
			continue
		}

		for _, nsName := range matchedNamespaces {
			// Get namespace object
			namespace := &corev1.Namespace{}
			if err := r.Get(ctx, types.NamespacedName{Name: nsName}, namespace); err != nil {
				log.Error(err, fmt.Sprintf("unable to fetch namespace %s", nsName))
				return err
			}

			// Skip RBAC deletion if the namespace has the "managed-by" label
			if namespace.Labels[common.ArgoCDManagedByLabel] == nsName {
				log.Info(fmt.Sprintf("Skipping RBAC deletion for namespace %s due to managed-by label", nsName))
				continue
			}

			if allowManaged {
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
	}

	return nil
}
