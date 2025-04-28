package argocd

import (
	"context"
	"fmt"
	"os"
	"strings"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj/argo-cd/v2/util/glob"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// reconcileNamespaceManagement ensures that ArgoCD managed namespaces are properly tracked
// and updated based on the NamespaceManagement CRs. It identifies the namespaces managed by
// ArgoCD, validates them against the management rules, and updates their status accordingly.
func (r *ReconcileArgoCD) reconcileNamespaceManagement(argocd *argoproj.ArgoCD) error {
	log.Info("Reconciling NamespaceManagement")
	ctx := context.TODO()

	var errorMessages []string
	var managedNamespaces []corev1.Namespace

	// Check if Namespace Management is explicitly enabled via the Subscription env variable.
	if !isNamespaceManagementEnabled() {
		if err := handleFeatureDisable(r.Client); err != nil {
			return err
		}
		return nil
	}

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

	// Process each NamespaceManagement CR
	for _, nm := range nmList.Items {
		nmCopy := nm.DeepCopy()
		namespace := nm.Namespace
		var message string

		// Check if the namespace is explicitly disallowed (allowManagedBy: false)
		allowed, exists := allowedNamespaces[namespace]
		if exists && !allowed {
			message = fmt.Sprintf("Namespace %s is not allowed to be managed by ArgoCD", namespace)
			errorMessages = append(errorMessages, message)
			if err := updateStatusConditionOfNamespaceManagement(ctx, createCondition(message), nmCopy, r.Client, log); err != nil {
				log.Error(err, "Failed to update status of NamespaceManagement CR", "namespace", namespace)
				errorMessages = append(errorMessages, fmt.Sprintf("status update failed for namespace %s: %v", namespace, err))
			}
			continue
		}

		// Validate namespace management rules
		if nmCopy.Spec.ManagedBy == argocd.Namespace && matchesNamespaceManagementRules(argocd, namespace) {
			managedNamespaces = append(managedNamespaces, corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: namespace},
			})
		} else {
			message = fmt.Sprintf("ArgoCD does not allow management of namespace: %s", namespace)
			errorMessages = append(errorMessages, message)
		}

		// Update NamespaceManagement status
		if err := updateStatusConditionOfNamespaceManagement(ctx, createCondition(message), nmCopy, r.Client, log); err != nil {
			log.Error(err, "Failed to update status of NamespaceManagement CR", "namespace", namespace)
			errorMessages = append(errorMessages, fmt.Sprintf("status update failed for namespace %s: %v", namespace, err))
		}
	}

	// Always include the ArgoCD namespace
	managedNamespaces = append(managedNamespaces, corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: argocd.Namespace},
	})

	// Store the list of managed namespaces
	r.ManagedNamespaces = &corev1.NamespaceList{Items: managedNamespaces}

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
		allowedPatterns = append(allowedPatterns, managedNs.Name) // Collect name patterns
	}

	return glob.MatchStringInList(allowedPatterns, namespace, glob.GLOB)
}

// Check if namespace management is explicitly enabled via Subscription
func isNamespaceManagementEnabled() bool {
	return os.Getenv(common.EnableManagedNamespace) == "true"
}

// If the EnableManagedNamespace feature is disabled, clean up the RBACs associated with the managed namespaces
// and remove the corresponding fields from the ArgoCD and NamespaceManagement CRs.
func handleFeatureDisable(client client.Client) error {
	ctx := context.TODO()

	// Check if NamespaceManagement CRs exist and if any ArgoCD instance has namespace management enabled.
	nsMgmtList := &argoproj.NamespaceManagementList{}
	if err := client.List(ctx, nsMgmtList); err != nil {
		return err
	}

	argoCDList := &argoproj.ArgoCDList{}
	if err := client.List(ctx, argoCDList); err != nil {
		return err
	}

	hasNamespaceManagement := false
	for _, argoCD := range argoCDList.Items {
		if argoCD.Spec.NamespaceManagement != nil {
			hasNamespaceManagement = true
			break // No need to check further, we found one
		}
	}

	// Exit if no NamespaceManagement CRs exist or no ArgoCD instance has namespace management enabled.
	if len(nsMgmtList.Items) == 0 && !hasNamespaceManagement {
		log.Info("No NamespaceManagement CRs and ArgoCD instances with namespace management found, skipping feature disable")
		return nil
	}

	k8sClient, err := initK8sClient()
	if err != nil {
		log.Error(err, "Failed to initialize Kubernetes client")
		return err
	}

	for _, argoCD := range argoCDList.Items {
		if argoCD.Spec.NamespaceManagement == nil {
			continue // Skip ArgoCD instances without namespaceManagement
		}

		for _, ns := range argoCD.Spec.NamespaceManagement {
			// First delete the RBACs associated with the namespaces present in .spec.namespaceManagement field
			if err := deleteRBACsForNamespace(ns.Name, k8sClient); err != nil {
				log.Error(err, fmt.Sprintf("failed to delete RBACs for namespace: %s", ns.Name))
			} else {
				log.Info(fmt.Sprintf("Successfully removed the RBACs for namespace: %s", ns.Name))
			}

			err = deleteManagedNamespaceFromClusterSecret(argoCD.Namespace, ns.Name, k8sClient)
			if err != nil {
				log.Error(err, fmt.Sprintf("unable to delete namespace %s from cluster secret", ns.Name))
			} else {
				log.Info(fmt.Sprintf("Successfully deleted namespace %s from cluster secret", ns.Name))
			}
		}

		// Remove .spec.namespaceManagement
		argoCDCopy := argoCD.DeepCopy()
		argoCDCopy.Spec.NamespaceManagement = nil
		if err := client.Update(ctx, argoCDCopy); err != nil {
			log.Error(err, "Failed to update ArgoCD CR", "namespace", argoCD.Namespace)
		} else {
			log.Info("Removed .spec.namespaceManagement from ArgoCD CR", "namespace", argoCD.Namespace)
		}
	}

	for _, nsMgmt := range nsMgmtList.Items {
		// First delete the RBACs associated with the namespaces present in NamespaceManagement
		if err := deleteRBACsForNamespace(nsMgmt.Namespace, k8sClient); err != nil {
			log.Error(err, fmt.Sprintf("failed to delete RBACs for namespace: %s", nsMgmt.Namespace))
		} else {
			log.Info(fmt.Sprintf("Successfully removed the RBACs for namespace: %s", nsMgmt.Namespace))
		}

		err = deleteManagedNamespaceFromClusterSecret(nsMgmt.Spec.ManagedBy, nsMgmt.Namespace, k8sClient)
		if err != nil {
			log.Error(err, fmt.Sprintf("unable to delete namespace %s from cluster secret", nsMgmt.Namespace))
		} else {
			log.Info(fmt.Sprintf("Successfully deleted namespace1 %s from cluster secret", nsMgmt.Namespace))
		}

		// Remove .spec.managedBy
		if nsMgmt.Spec.ManagedBy != "" {
			nsMgmtCopy := nsMgmt.DeepCopy()
			nsMgmtCopy.Spec.ManagedBy = ""
			if err := client.Update(ctx, nsMgmtCopy); err != nil {
				log.Error(err, "Failed to update NamespaceManagement CR", "namespace", nsMgmt.Namespace)
			} else {
				log.Info("Removed .spec.managedBy from NamespaceManagement CR", "namespace", nsMgmt.Namespace)
			}
		}

	}

	return nil
}
