package argocd

import (
	"reflect"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// namespacePredicate defines how we filter events on namespaces to decide if a new round of reconciliation should be triggered or not.
func (r *ArgoCDReconciler) namespacePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			processEvent := false

			// for now only process namespace create events if namespace is created with the
			// managed by label. This behavior might change if in the future we change namespace
			// management to go through the CR instead of letting users directly label the ns
			if _, ok := e.Object.GetLabels()[common.ArgoCDArgoprojKeyManagedBy]; ok {
				processEvent = true
			}

			return processEvent
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// processEvent decides if this event reaches the event handler or not
			processEvent := false

			InitializeScheduledForRBACDeletion()

			// check for "argoproj.argocd.io/managed-by"
			l1 := shouldProcessLabelEventForUpdate(e, common.ArgoCDArgoprojKeyManagedBy, common.ArgoCDRBACTypeResourceMananagement)
			// check for "argoproj.argocd.io/apps-managed-by"
			l2 := shouldProcessLabelEventForUpdate(e, common.ArgoCDArgoprojKeyAppsManagedBy, common.ArgoCDRBACTypeAppManagement)
			// check for "argoproj.argocd.io/appsets-managed-by"
			l3 := shouldProcessLabelEventForUpdate(e, common.ArgoCDArgoprojKeyAppSetsManagedBy, common.ArgoCDRBACTypeAppSetManagement)

			// process event if even one of them returns true
			if l1 || l2 || l3 {
				processEvent = true
			}
			return processEvent
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// processEvent decides if this event reaches the event handler or not
			processEvent := false

			InitializeScheduledForRBACDeletion()

			// check for "argoproj.argocd.io/managed-by"
			l1 := r.shouldProcessLabelEventForDelete(e, common.ArgoCDArgoprojKeyManagedBy, common.ArgoCDRBACTypeResourceMananagement)
			// check for "argoproj.argocd.io/apps-managed-by"
			l2 := r.shouldProcessLabelEventForDelete(e, common.ArgoCDArgoprojKeyAppsManagedBy, common.ArgoCDRBACTypeAppManagement)
			// check for "argoproj.argocd.io/appsets-managed-by"
			l3 := r.shouldProcessLabelEventForDelete(e, common.ArgoCDArgoprojKeyAppSetsManagedBy, common.ArgoCDRBACTypeAppSetManagement)

			// process event if even one of them returns true
			if l1 || l2 || l3 {
				processEvent = true
			}
			return processEvent
		},
	}
}

// ignoreDeletionOrStatusUpdatePredicate filters out events that only update status of Argo CD CR, or confirm instance deletion
func ignoreDeletionOrStatusUpdatePredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Ignore updates to CR status in which case metadata.Generation does not change
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Evaluates to false if the object has been confirmed deleted.
			return !e.DeleteStateUnknown
		},
	}
}

// deleteSSOPredicate Triggers clean up of SSO resources if spec.SSO is removed entirely
func (r *ArgoCDReconciler) deleteSSOPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// if Update does not involve an Argo CD object, ignore
			newCR, ok := e.ObjectNew.(*argoproj.ArgoCD)
			if !ok {
				return false
			}
			oldCR, ok := e.ObjectOld.(*argoproj.ArgoCD)
			if !ok {
				return false
			}

			// Handle deletion of SSO from Argo CD custom resource
			if !reflect.DeepEqual(oldCR.Spec.SSO, newCR.Spec.SSO) && newCR.Spec.SSO == nil {
				err := r.SSOController.DeleteResources(newCR, oldCR)
				if err != nil {
					r.Logger.Error(err, "failed to delete SSO resources")
				}
				return false
			}

			// for other cases, trigger reconciliation
			return true
		},
	}
}
