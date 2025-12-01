/*
Copyright 2019, 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package argocd

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logr "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type lock struct {
	lock sync.Mutex
}

func (l *lock) protect(code func()) {
	l.lock.Lock()
	defer l.lock.Unlock()
	code()
}

type TokenRenewalTimer struct {
	timer   *time.Timer
	stopped bool
}

type LocalUsersInfo struct {
	// Stores the the timers that will auto-renew the API tokens for local users
	// after they expire. The key format is "argocd-namespace/user-name"
	TokenRenewalTimers map[string]*TokenRenewalTimer
	// Protects access to the token renewal timers and the K8S resources that
	// get updated as part of renewing the user tokens
	UserTokensLock lock
}

// blank assignment to verify that ReconcileArgoCD implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileArgoCD{}

// ArgoCDReconciler reconciles a ArgoCD object
// TODO(upgrade): rename to ArgoCDRecoonciler
type ReconcileArgoCD struct {
	client.Client
	Scheme            *runtime.Scheme
	ManagedNamespaces *corev1.NamespaceList
	// Stores a list of ApplicationSourceNamespaces as keys
	ManagedSourceNamespaces map[string]string
	// Stores a list of ApplicationSetSourceNamespaces as keys
	ManagedApplicationSetSourceNamespaces map[string]string

	// Stores a list of NotificationsSourceNamespaces as keys
	ManagedNotificationsSourceNamespaces map[string]string

	// Stores label selector used to reconcile a subset of ArgoCD
	LabelSelector string

	K8sClient  kubernetes.Interface
	LocalUsers *LocalUsersInfo
	// FipsConfigChecker checks if the deployment needs FIPS specific environment variables set.
	FipsConfigChecker argoutil.FipsConfigChecker
}

var log = logr.Log.WithName("controller_argocd")

// Map to keep track of running Argo CD instances using their namespaces as key and phase as value
// This map will be used for the performance metrics purposes
// Important note: This assumes that each instance only contains one Argo CD instance
// as, having multiple Argo CD instances in the same namespace is considered an anti-pattern
var ActiveInstanceMap = make(map[string]string)

//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=*
//+kubebuilder:rbac:groups="",resources=configmaps;endpoints;events;persistentvolumeclaims;pods;namespaces;secrets;serviceaccounts;services;services/finalizers,verbs=*
//+kubebuilder:rbac:groups=apps,resources=deployments;replicasets;daemonsets;statefulsets,verbs=*
//+kubebuilder:rbac:groups=apps,resourceNames=argocd-operator,resources=deployments/finalizers,verbs=update
//+kubebuilder:rbac:groups=argoproj.io,resources=argocds;argocds/finalizers;argocds/status,verbs=*
//+kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=*
//+kubebuilder:rbac:groups=batch,resources=cronjobs;jobs,verbs=*
//+kubebuilder:rbac:groups=config.openshift.io,resources=clusterversions,verbs=get;list;watch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=*
//+kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=create;delete;get;list;patch;update;watch;
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheuses;prometheusrules;servicemonitors,verbs=*
//+kubebuilder:rbac:groups=route.openshift.io,resources=routes;routes/custom-host,verbs=*
//+kubebuilder:rbac:groups=argoproj.io,resources=applications;appprojects,verbs=*
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=*,verbs=*
//+kubebuilder:rbac:groups="",resources=pods;pods/log,verbs=get
//+kubebuilder:rbac:groups=template.openshift.io,resources=templates;templateinstances;templateconfigs,verbs=*
//+kubebuilder:rbac:groups="oauth.openshift.io",resources=oauthclients,verbs=get;list;watch;create;delete;patch;update
//+kubebuilder:rbac:groups=argoproj.io,resources=notificationsconfigurations;notificationsconfigurations/finalizers,verbs=*
//+kubebuilder:rbac:groups="apiregistration.k8s.io",resources="apiservices",verbs=get;list
//+kubebuilder:rbac:groups=argoproj.io,resources=namespacemanagements;namespacemanagements/finalizers;namespacemanagements/status,verbs=*
//+kubebuilder:rbac:groups=argocd-image-updater.argoproj.io,resources=imageupdaters;imageupdaters/finalizers,verbs=*

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the ArgoCD object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *ReconcileArgoCD) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {

	result, argocd, argocdStatus, err := r.internalReconcile(ctx, request)

	message := ""
	if err != nil {
		message = err.Error()
		argocdStatus.Phase = "Failed" // Any error should reset phase back to Failed
	}

	log.Info("reconciling status")
	if reconcileStatusErr := r.reconcileStatus(argocd, argocdStatus); reconcileStatusErr != nil {
		log.Error(reconcileStatusErr, "Unable to reconcile status")
		argocdStatus.Phase = "Failed"
		message = "unable to reconcile ArgoCD CR .status field"
	}

	if updateStatusErr := updateStatusAndConditionsOfArgoCD(ctx, createCondition(message), argocd, argocdStatus, r.Client, log); updateStatusErr != nil {
		log.Error(updateStatusErr, "unable to update status of ArgoCD")
		return reconcile.Result{}, updateStatusErr
	}

	return result, err
}

func (r *ReconcileArgoCD) internalReconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, *argoproj.ArgoCD, *argoproj.ArgoCDStatus, error) {

	argoCDStatus := &argoproj.ArgoCDStatus{} // Start with a blank canvas

	reconcileStartTS := time.Now()
	defer func() {
		ReconcileTime.WithLabelValues(request.Namespace).Observe(time.Since(reconcileStartTS).Seconds())
	}()

	reqLogger := logr.FromContext(ctx, "namespace", request.Namespace, "name", request.Name)
	reqLogger.Info("Reconciling ArgoCD")

	argocd := &argoproj.ArgoCD{}
	err := r.Get(ctx, request.NamespacedName, argocd)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, argocd, argoCDStatus, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, argocd, argoCDStatus, err
	}

	// Redis TLS Checksum and Repo Server TLS Checksum should be preserved between reconcile calls (the lifecycle of these fields is greater than a single reconcile call, unlike the other fields in .status)
	argoCDStatus.RepoTLSChecksum = argocd.Status.RepoTLSChecksum
	argoCDStatus.RedisTLSChecksum = argocd.Status.RedisTLSChecksum

	// If the number of notification replicas is greater than 1, display a warning.
	if argocd.Spec.Notifications.Replicas != nil && *argocd.Spec.Notifications.Replicas > 1 {
		reqLogger.Info("WARNING: Argo CD Notification controller does not support multiple replicas. Notification replicas cannot be greater than 1.")
	}

	// Fetch labelSelector from r.LabelSelector (command-line option)
	labelSelector, err := labels.Parse(r.LabelSelector)
	if err != nil {
		message := fmt.Sprintf("error parsing the labelSelector '%s'.", labelSelector)
		reqLogger.Error(err, message)
		return reconcile.Result{}, argocd, argoCDStatus, fmt.Errorf("%s error: %w", message, err)
	}

	// Match the value of labelSelector from ReconcileArgoCD to labels from the argocd instance
	if !labelSelector.Matches(labels.Set(argocd.Labels)) {
		reqLogger.Error(nil, fmt.Sprintf("the ArgoCD instance '%s' does not match the label selector '%s' and skipping for reconciliation", request.NamespacedName, r.LabelSelector))
		return reconcile.Result{}, argocd, argoCDStatus, fmt.Errorf("the ArgoCD instance '%s' does not match the label selector '%s' and skipping for reconciliation", request.NamespacedName, r.LabelSelector)
	}

	newPhase := argocd.Status.Phase
	// If we discover a new Argo CD instance in a previously un-seen namespace
	// we add it to the map and increment active instance count by phase
	// as well as total active instance count
	if _, ok := ActiveInstanceMap[request.Namespace]; !ok {
		if newPhase != "" {
			ActiveInstanceMap[request.Namespace] = newPhase
			ActiveInstancesByPhase.WithLabelValues(newPhase).Inc()
			ActiveInstancesTotal.Inc()
		}
	} else {
		// If we discover an existing instance's phase has changed since we last saw it
		// increment instance count with new phase and decrement instance count with old phase
		// update the phase in corresponding map entry
		// total instance count remains the same
		if oldPhase := ActiveInstanceMap[argocd.Namespace]; oldPhase != newPhase {
			ActiveInstanceMap[argocd.Namespace] = newPhase
			ActiveInstancesByPhase.WithLabelValues(newPhase).Inc()
			ActiveInstancesByPhase.WithLabelValues(oldPhase).Dec()
		}
	}

	ActiveInstanceReconciliationCount.WithLabelValues(argocd.Namespace).Inc()

	if argocd.GetDeletionTimestamp() != nil {

		argoCDStatus.Phase = "Unknown" // Set to Unknown since we are in the process of deleting ArgoCD CR

		// Argo CD instance marked for deletion; remove entry from activeInstances map and decrement active instance count
		// by phase as well as total
		delete(ActiveInstanceMap, argocd.Namespace)
		ActiveInstancesByPhase.WithLabelValues(newPhase).Dec()
		ActiveInstancesTotal.Dec()
		ActiveInstanceReconciliationCount.DeleteLabelValues(argocd.Namespace)
		ReconcileTime.DeletePartialMatch(prometheus.Labels{"namespace": argocd.Namespace})

		// Remove any local user token renewal timers for the namespace
		r.cleanupNamespaceTokenTimers(argocd.Namespace)

		if argocd.IsDeletionFinalizerPresent() {
			if err := r.deleteClusterResources(argocd); err != nil {
				return reconcile.Result{}, argocd, argoCDStatus, fmt.Errorf("failed to delete ClusterResources: %w", err)
			}

			if isRemoveManagedByLabelOnArgoCDDeletion() {
				if err := r.removeManagedByLabelFromNamespaces(argocd.Namespace); err != nil {
					return reconcile.Result{}, argocd, argoCDStatus, fmt.Errorf("failed to remove label from namespace[%v], error: %w", argocd.Namespace, err)
				}
			}

			if err := r.removeUnmanagedSourceNamespaceResources(argocd); err != nil {
				return reconcile.Result{}, argocd, argoCDStatus, fmt.Errorf("failed to remove resources from sourceNamespaces, error: %w", err)
			}

			if err := r.removeUnmanagedApplicationSetSourceNamespaceResources(argocd); err != nil {
				return reconcile.Result{}, argocd, argoCDStatus, fmt.Errorf("failed to remove resources from applicationSetSourceNamespaces, error: %w", err)
			}
			if err := r.removeUnmanagedNotificationsSourceNamespaceResources(argocd); err != nil {
				return reconcile.Result{}, argocd, argoCDStatus, fmt.Errorf("failed to remove resources from notificationsSourceNamespaces, error: %w", err)
			}

			if err := r.removeDeletionFinalizer(argocd); err != nil {
				return reconcile.Result{}, argocd, argoCDStatus, err
			}

			// remove namespace of deleted Argo CD instance from deprecationEventEmissionTracker (if exists) so that if another instance
			// is created in the same namespace in the future, that instance is appropriately tracked
			delete(DeprecationEventEmissionTracker, argocd.Namespace)
		}

		return reconcile.Result{}, argocd, argoCDStatus, nil
	}

	if !argocd.IsDeletionFinalizerPresent() {
		if err := r.addDeletionFinalizer(argocd); err != nil {
			return reconcile.Result{}, argocd, argoCDStatus, err
		}
	}

	if err = r.setManagedNamespaces(argocd); err != nil {
		return reconcile.Result{}, argocd, argoCDStatus, err
	}

	if err = r.setManagedSourceNamespaces(argocd); err != nil {
		return reconcile.Result{}, argocd, argoCDStatus, err
	}

	if err = r.setManagedApplicationSetSourceNamespaces(argocd); err != nil {
		return reconcile.Result{}, argocd, argoCDStatus, err
	}
	if err = r.setManagedNotificationsSourceNamespaces(argocd); err != nil {
		return reconcile.Result{}, argocd, argoCDStatus, err
	}
	// Handle NamespaceManagement reconciliation and check if Namespace Management is enabled via the Subscription env variable.
	if isNamespaceManagementEnabled() {
		if err := r.reconcileNamespaceManagement(argocd); err != nil {
			return reconcile.Result{}, argocd, argoCDStatus, err
		}
	} else if argocd.Spec.NamespaceManagement != nil {
		k8sClient := r.K8sClient
		if err := r.disableNamespaceManagement(argocd, k8sClient); err != nil {
			log.Error(err, "Failed to disable NamespaceManagement feature")
			return reconcile.Result{}, argocd, argoCDStatus, err
		}
	} else if len(argocd.Spec.NamespaceManagement) == 0 {
		// Handle cleanup of NamespaceManagement RBAC when the feature is removed from the ArgoCD CR.
		nsMgmtList := &argoproj.NamespaceManagementList{}
		if err := r.List(context.TODO(), nsMgmtList); err != nil {
			return reconcile.Result{}, argocd, argoCDStatus, err
		}

		k8sClient := r.K8sClient
		for _, nsMgmt := range nsMgmtList.Items {
			// Skip the namespaceManagement CR which is not managed by the current Argo CD instance
			if nsMgmt.Spec.ManagedBy != argocd.Namespace {
				continue
			}

			// Check if the namespace has a "managed-by" label
			namespace := &corev1.Namespace{}
			if err := r.Get(ctx, types.NamespacedName{Name: nsMgmt.Namespace}, namespace); err != nil {
				log.Error(err, fmt.Sprintf("unable to fetch namespace %s", nsMgmt.Namespace))
				return reconcile.Result{}, argocd, argoCDStatus, err
			}

			// Skip RBAC deletion if the namespace has the "managed-by" label
			if namespace.Labels[common.ArgoCDManagedByLabel] == nsMgmt.Namespace {
				log.Info(fmt.Sprintf("Skipping RBAC deletion for namespace %s due to managed-by label", nsMgmt.Namespace))
				continue
			}

			// Remove roles and rolebindings
			if err := deleteRBACsForNamespace(nsMgmt.Namespace, k8sClient); err != nil {
				log.Error(err, fmt.Sprintf("Failed to delete RBACs for namespace: %s", nsMgmt.Namespace))
				return reconcile.Result{}, argocd, argoCDStatus, err
			}
			log.Info(fmt.Sprintf("Successfully removed RBACs for namespace: %s", nsMgmt.Namespace))

			if err := deleteManagedNamespaceFromClusterSecret(argocd.Namespace, nsMgmt.Namespace, k8sClient); err != nil {
				log.Error(err, fmt.Sprintf("Unable to delete namespace %s from cluster secret", nsMgmt.Namespace))
				return reconcile.Result{}, argocd, argoCDStatus, err
			}

		}
	}

	// Process DropMetadata for namespace-based label cleanup
	r.processDropMetadataForCleanup(argocd)

	if err := r.reconcileResources(argocd, argoCDStatus); err != nil {
		// Error reconciling ArgoCD sub-resources - requeue the request.
		return reconcile.Result{}, argocd, argoCDStatus, err
	}

	// Return and don't requeue
	return reconcile.Result{}, argocd, argoCDStatus, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReconcileArgoCD) SetupWithManager(mgr ctrl.Manager) error {
	bldr := ctrl.NewControllerManagedBy(mgr)
	r.setResourceWatches(bldr, r.clusterResourceMapper, r.tlsSecretMapper, r.namespaceResourceMapper, r.clusterSecretResourceMapper, r.applicationSetSCMTLSConfigMapMapper, r.nmMapper)
	return bldr.Complete(r)
}
