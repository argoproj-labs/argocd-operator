package argocd

import (
	"sync"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/monitoring"
	"github.com/argoproj-labs/argocd-operator/pkg/openshift"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	oappsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var lock sync.RWMutex

// For a given label, ManagedNsOpts tracks the following for a given namespace, that is either no longer managed by an instance, or is being managed by a different instance:
// 1. The namespace of the previously managing Argo CD instance if any
// 2. The namespace of the new managing Argo CD instance if any
// 2. The label value to determine type of rbac resources to be deleted from the affected namespace: "resource-management/app-management/appset-management" etc
type ManagedNsOpts struct {
	PrevManagingNs             string
	NewManagingNs              string
	ResourceDeletionLabelValue string
}

// scheduledForRBACDeletion tracks namespaces that need to have roles/rolebindings deleted from them
// when they are no longer being managed by a given Argo CD instance. These namespaces may also need to be
// removed from the cluster secret of the previously managing Argo CD instance.
// The key is the affected namespace, and the value is a list of namespace options associated with this namespace, one set of namespace options per label that was carried by this namespace
var ScheduledForRBACDeletion map[string][]ManagedNsOpts

func InitializeScheduledForRBACDeletion() {
	if len(ScheduledForRBACDeletion) == 0 {
		ScheduledForRBACDeletion = make(map[string][]ManagedNsOpts)
	}
}

// namespacePredicate defines how we filter events on namespaces to decide if a new round of reconciliation should be triggered or not.
func (r *ArgoCDReconciler) namespacePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(ce event.CreateEvent) bool {
			return true
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

func ignoreDeletionPredicate() predicate.Predicate {
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

// shouldProcessLabelEventForUpdate decides if a given namespace update event should be processed or not. Only update events involving
// specific labels are considered for processing.
// Processing of such events involves either cleaning up existing resources in the namespace, or triggering reconciliation
// to create new resources in a different namespace, or both (performed by the event handler)
// General logic for filetering label based namespace update events:
// check if label exists in newMeta, if yes, then
// 1. Check if oldMeta had the label or not. If not ==> new managed namespace! Return true to process event
// 2. if yes, check if the old and new values are different, if yes ==> namespace is now managed by a different instance.
// we must process event to schedule old rbac resources for deletion from this namespace, and potentially remove it from the cluster secret
// Event is then handled by the reconciler, which would create new RBAC resources appropriately in the next reconciliation cycle.
// If label does not exist in newMeta, then
// 1. Check if oldMeta had the label or not. If not ==> no change, ignore this event.
// 2. If yes, namespace is no longer managed. Schedule for rbac deletion and process event
func shouldProcessLabelEventForUpdate(e event.UpdateEvent, label, resourceDeletionLabelVal string) bool {
	// lock.Lock()
	// defer lock.Unlock()

	// processEvent decides if this event reaches the event handler or not
	processEvent := false

	if valNew, ok := e.ObjectNew.GetLabels()[label]; ok {
		if valOld, ok := e.ObjectOld.GetLabels()[label]; ok && valOld != valNew {
			affectedNs := e.ObjectOld.GetName()
			prevManagingNs := valOld
			newManagingNs := valNew

			// append ManagedNsOpt to rbac deletion map for current namespace
			newNsOpt := ManagedNsOpts{
				PrevManagingNs:             prevManagingNs,
				NewManagingNs:              newManagingNs,
				ResourceDeletionLabelValue: resourceDeletionLabelVal,
			}
			ScheduledForRBACDeletion[affectedNs] = append(ScheduledForRBACDeletion[affectedNs], newNsOpt)

			// process event for resource clean up and trigger reconciliation to create resources in new managing ns
			processEvent = true
		} else if !ok {
			// no old value => new managed ns. Trigger reconciliation for creation of resources
			processEvent = true
		} else if valOld == valNew {
			// no change in label, skip processing event
			processEvent = false
		}
	} else {
		if valOld, ok := e.ObjectOld.GetLabels()[label]; ok && valOld != "" {
			affectedNs := e.ObjectOld.GetName()
			prevManagingNs := valOld

			// append ManagedNsOpt to rbac deletion map for current namespace
			newNsOpt := ManagedNsOpts{
				PrevManagingNs: prevManagingNs,
				// no label in newMeta ==> namespace is no longer managed
				NewManagingNs:              "",
				ResourceDeletionLabelValue: resourceDeletionLabelVal,
			}
			ScheduledForRBACDeletion[affectedNs] = append(ScheduledForRBACDeletion[affectedNs], newNsOpt)

			// process event for resource clean up
			processEvent = true
		} else {
			// oldMeta either didn't carry label or it was not set. Either way, no change
			processEvent = false
		}
	}
	return processEvent
}

// shouldProcessLabelEventForDelete decides if a given namespace deletion event should be processed or not. Only deletion events involving namespaces carrying specific labels are considered for processing.
// Processing of such events involves cleaning up existing resources in the namespace, and/or removal from a previously managing instance's cluster secret
func (r *ArgoCDReconciler) shouldProcessLabelEventForDelete(e event.DeleteEvent, label, resourceDeletionLabelVal string) bool {
	processEvent := false
	affectedNs := e.Object.GetName()

	if valOld, ok := e.Object.GetLabels()[label]; ok && valOld != "" {
		prevManagingNs := valOld

		// append ManagedNsOpt to rbac deletion map for current namespace
		newNsOpt := ManagedNsOpts{
			PrevManagingNs: prevManagingNs,
			// no label in newMeta ==> namespace is no longer managed
			NewManagingNs:              "",
			ResourceDeletionLabelValue: resourceDeletionLabelVal,
		}
		ScheduledForRBACDeletion[affectedNs] = append(ScheduledForRBACDeletion[affectedNs], newNsOpt)

		// process event for resource clean up
		processEvent = true
	} else {
		// check if terminating namespace contains Argo CD instance to be deleted
		if objs, err := resource.ListObjects(affectedNs, &argoproj.ArgoCDList{}, r.Client, []client.ListOption{}); err == nil {
			if instances, ok := objs.(*argoproj.ArgoCDList); ok {
				if len(instances.Items) > 0 {
					// process event to delete Argo CD instance resources
					processEvent = true
				}
			}
		} else {
			// terminating namespace neither carries required labels, nor an Argo CD instance - ignore
			processEvent = false
		}
	}
	return processEvent
}

// TO DO: THIS IS INCOMPLETE
func (r *ArgoCDReconciler) setResourceWatches(bldr *builder.Builder, namespaceMapper handler.MapFunc) *builder.Builder {
	namespaceHandler := handler.EnqueueRequestsFromMapFunc(namespaceMapper)

	// Watch for changes to primary resource ArgoCD
	bldr.For(&argoproj.ArgoCD{}, builder.WithPredicates(ignoreDeletionPredicate()))

	// Watch for changes to ConfigMap sub-resources owned by ArgoCD instances.
	bldr.Owns(&corev1.ConfigMap{})

	// Watch for changes to Secret sub-resources owned by ArgoCD instances.
	bldr.Owns(&corev1.Secret{})

	// Watch for changes to Service sub-resources owned by ArgoCD instances.
	bldr.Owns(&corev1.Service{})

	// Watch for changes to Deployment sub-resources owned by ArgoCD instances.
	bldr.Owns(&appsv1.Deployment{})

	// Watch for changes to Secret sub-resources owned by ArgoCD instances.
	bldr.Owns(&appsv1.StatefulSet{})

	// Watch for changes to Ingress sub-resources owned by ArgoCD instances.
	bldr.Owns(&networkingv1.Ingress{})

	bldr.Owns(&rbacv1.Role{})

	bldr.Owns(&rbacv1.RoleBinding{})

	if openshift.IsRouteAPIAvailable() {
		// Watch OpenShift Route sub-resources owned by ArgoCD instances.
		bldr.Owns(&routev1.Route{})
	}

	if monitoring.IsPrometheusAPIAvailable() {
		// Watch Prometheus sub-resources owned by ArgoCD instances.
		bldr.Owns(&monitoringv1.Prometheus{})

		// Watch Prometheus ServiceMonitor sub-resources owned by ArgoCD instances.
		bldr.Owns(&monitoringv1.ServiceMonitor{})
	}

	if openshift.IsTemplateAPIAvailable() {
		// Watch for the changes to Deployment Config
		bldr.Owns(&oappsv1.DeploymentConfig{})

	}

	// Watch for changes to namespaces managed by Argo CD instances.
	bldr.Watches(&corev1.Namespace{}, namespaceHandler, builder.WithPredicates(r.namespacePredicate()))

	return bldr
}
