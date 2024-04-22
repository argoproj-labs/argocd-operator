package argocd

import (
	"sync"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
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
	ScheduledForRBACDeletion = make(map[string][]ManagedNsOpts)
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
func (r *ArgoCDReconciler) setResourceWatches(bldr *builder.Builder, namespaceMapper, clusterResourceMapper, tlsSecretMapper, clusterSecretMapper, applicationSetGitlabSCMTLSConfigMapMapper handler.MapFunc) *builder.Builder {
	namespaceHandler := handler.EnqueueRequestsFromMapFunc(namespaceMapper)
	clusterResourceHandler := handler.EnqueueRequestsFromMapFunc(clusterResourceMapper)
	clusterSecretHandler := handler.EnqueueRequestsFromMapFunc(clusterSecretMapper)
	appSetGitlabSCMTLSConfigMapHandler := handler.EnqueueRequestsFromMapFunc(applicationSetGitlabSCMTLSConfigMapMapper)
	tlsSecretHandler := handler.EnqueueRequestsFromMapFunc(tlsSecretMapper)

	// Watch for changes to primary resource ArgoCD
	bldr.For(&argoproj.ArgoCD{}, builder.WithPredicates(ignoreDeletionOrStatusUpdatePredicate(), r.deleteSSOPredicate()))

	// Watch for changes to ConfigMap resources owned by ArgoCD instances.
	bldr.Owns(&corev1.ConfigMap{})

	// Watch for changes to Secret resources owned by ArgoCD instances.
	bldr.Owns(&corev1.Secret{})

	// Watch for changes to Service resources owned by ArgoCD instances.
	bldr.Owns(&corev1.Service{})

	// Watch for changes to Deployment resources owned by ArgoCD instances.
	bldr.Owns(&appsv1.Deployment{})

	// Watch for changes to Secret resources owned by ArgoCD instances.
	bldr.Owns(&appsv1.StatefulSet{})

	// Watch for changes to Ingress resources owned by ArgoCD instances.
	bldr.Owns(&networkingv1.Ingress{})

	// Watch for changes to Role resources owned by ArgoCD instances.
	bldr.Owns(&rbacv1.Role{})

	// Watch for changes to RoleBinding resources owned by ArgoCD instances.
	bldr.Owns(&rbacv1.RoleBinding{})

	// Watch for changes to NotificationsConfiguration CR
	bldr.Owns(&v1alpha1.NotificationsConfiguration{})

	// Inspect cluster to verify availability of extra features
	// This sets the flags that are used in subsequent checks
	if err := VerifyClusterAPIs(); err != nil {
		r.Logger.Debug("unable to inspect cluster for all APIs")
	}

	if openshift.IsRouteAPIAvailable() {
		// Watch OpenShift Route resources owned by ArgoCD instances.
		bldr.Owns(&routev1.Route{})
	}

	if monitoring.IsPrometheusAPIAvailable() {
		// Watch Prometheus resources owned by ArgoCD instances.
		bldr.Owns(&monitoringv1.Prometheus{})

		// Watch Prometheus ServiceMonitor resources owned by ArgoCD instances.
		bldr.Owns(&monitoringv1.ServiceMonitor{})
	}

	if openshift.IsTemplateAPIAvailable() {
		// Watch for the changes to Deployment Config
		bldr.Owns(&oappsv1.DeploymentConfig{})

	}

	// Watch for changes to ClusterRole resources managed by Argo CD instances
	bldr.Watches(&rbacv1.ClusterRoleBinding{}, clusterResourceHandler)

	// Watch for changes to ClusterRoleBinding resources managed by Argo CD instances
	bldr.Watches(&rbacv1.ClusterRole{}, clusterResourceHandler)

	// Watch for changes to namespaces managed by Argo CD instances
	bldr.Watches(&corev1.Namespace{}, namespaceHandler, builder.WithPredicates(r.namespacePredicate()))

	// Watch for changes to the appset gitlab SCM TLS certs config maps
	bldr.Watches(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: common.ArgoCDAppSetGitlabSCMTLSCertsConfigMapName,
	}}, appSetGitlabSCMTLSConfigMapHandler)

	// Watch for secrets of type TLS that might be created by external processes
	bldr.Watches(&corev1.Secret{Type: corev1.SecretTypeTLS}, tlsSecretHandler)

	// Watch for cluster secrets added to the argocd instance
	bldr.Watches(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Labels: map[string]string{
			common.ArgoCDArgoprojKeySecretType: common.ArgoCDSecretTypeCluster,
		}}}, clusterSecretHandler)

	return bldr
}
