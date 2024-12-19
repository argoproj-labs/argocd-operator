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
	"time"

	"github.com/prometheus/client_golang/prometheus"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logr "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

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
	// Stores label selector used to reconcile a subset of ArgoCD
	LabelSelector string
}

var log = logr.Log.WithName("controller_argocd")

// Map to keep track of running Argo CD instances using their namespaces as key and phase as value
// This map will be used for the performance metrics purposes
// Important note: This assumes that each instance only contains one Argo CD instance
// as, having multiple Argo CD instances in the same namespace is considered an anti-pattern
var ActiveInstanceMap = make(map[string]string)

//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=*
//+kubebuilder:rbac:groups="",resources=configmaps;endpoints;events;persistentvolumeclaims;pods;namespaces;secrets;serviceaccounts;services;services/finalizers,verbs=*
//+kubebuilder:rbac:groups=apps.openshift.io,resources=deploymentconfigs,verbs=*
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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the ArgoCD object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *ReconcileArgoCD) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {

	result, argocd, err := r.internalReconcile(ctx, request)

	message := ""
	if err != nil {
		message = err.Error()
	}

	if error := updateStatusConditionOfArgoCD(ctx, createCondition(message), argocd, r.Client, log); error != nil {
		log.Error(error, "unable to update status of ArgoCD")
		return reconcile.Result{}, error
	}

	return result, err
}

func (r *ReconcileArgoCD) internalReconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, *argoproj.ArgoCD, error) {

	reconcileStartTS := time.Now()
	defer func() {
		ReconcileTime.WithLabelValues(request.Namespace).Observe(time.Since(reconcileStartTS).Seconds())
	}()

	reqLogger := logr.FromContext(ctx, "namespace", request.Namespace, "name", request.Name)
	reqLogger.Info("Reconciling ArgoCD")

	argocd := &argoproj.ArgoCD{}
	err := r.Client.Get(ctx, request.NamespacedName, argocd)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, argocd, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, argocd, err
	}

	// Fetch labelSelector from r.LabelSelector (command-line option)
	labelSelector, err := labels.Parse(r.LabelSelector)
	if err != nil {
		message := fmt.Sprintf("error parsing the labelSelector '%s'.", labelSelector)
		reqLogger.Info(message)
		return reconcile.Result{}, argocd, fmt.Errorf("%s error: %w", message, err)
	}

	// Match the value of labelSelector from ReconcileArgoCD to labels from the argocd instance
	if !labelSelector.Matches(labels.Set(argocd.Labels)) {
		reqLogger.Info(fmt.Sprintf("the ArgoCD instance '%s' does not match the label selector '%s' and skipping for reconciliation", request.NamespacedName, r.LabelSelector))
		return reconcile.Result{}, argocd, fmt.Errorf("the ArgoCD instance '%s' does not match the label selector '%s' and skipping for reconciliation", request.NamespacedName, r.LabelSelector)
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

		// Argo CD instance marked for deletion; remove entry from activeInstances map and decrement active instance count
		// by phase as well as total
		delete(ActiveInstanceMap, argocd.Namespace)
		ActiveInstancesByPhase.WithLabelValues(newPhase).Dec()
		ActiveInstancesTotal.Dec()
		ActiveInstanceReconciliationCount.DeleteLabelValues(argocd.Namespace)
		ReconcileTime.DeletePartialMatch(prometheus.Labels{"namespace": argocd.Namespace})

		if argocd.IsDeletionFinalizerPresent() {
			if err := r.deleteClusterResources(argocd); err != nil {
				return reconcile.Result{}, argocd, fmt.Errorf("failed to delete ClusterResources: %w", err)
			}

			if isRemoveManagedByLabelOnArgoCDDeletion() {
				if err := r.removeManagedByLabelFromNamespaces(argocd.Namespace); err != nil {
					return reconcile.Result{}, argocd, fmt.Errorf("failed to remove label from namespace[%v], error: %w", argocd.Namespace, err)
				}
			}

			if err := r.removeUnmanagedSourceNamespaceResources(argocd); err != nil {
				return reconcile.Result{}, argocd, fmt.Errorf("failed to remove resources from sourceNamespaces, error: %w", err)
			}

			if err := r.removeUnmanagedApplicationSetSourceNamespaceResources(argocd); err != nil {
				return reconcile.Result{}, argocd, fmt.Errorf("failed to remove resources from applicationSetSourceNamespaces, error: %w", err)
			}

			if err := r.removeDeletionFinalizer(argocd); err != nil {
				return reconcile.Result{}, argocd, err
			}

			// remove namespace of deleted Argo CD instance from deprecationEventEmissionTracker (if exists) so that if another instance
			// is created in the same namespace in the future, that instance is appropriately tracked
			delete(DeprecationEventEmissionTracker, argocd.Namespace)
		}

		return reconcile.Result{}, argocd, nil
	}

	if !argocd.IsDeletionFinalizerPresent() {
		if err := r.addDeletionFinalizer(argocd); err != nil {
			return reconcile.Result{}, argocd, err
		}
	}

	// get the latest version of argocd instance before reconciling
	if err = r.Client.Get(ctx, request.NamespacedName, argocd); err != nil {
		return reconcile.Result{}, argocd, err
	}

	if err = r.setManagedNamespaces(argocd); err != nil {
		return reconcile.Result{}, argocd, err
	}

	if err = r.setManagedSourceNamespaces(argocd); err != nil {
		return reconcile.Result{}, argocd, err
	}

	if err = r.setManagedApplicationSetSourceNamespaces(argocd); err != nil {
		return reconcile.Result{}, argocd, err
	}

	if err := r.reconcileResources(argocd); err != nil {
		// Error reconciling ArgoCD sub-resources - requeue the request.
		return reconcile.Result{}, argocd, err
	}

	// Return and don't requeue
	return reconcile.Result{}, argocd, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReconcileArgoCD) SetupWithManager(mgr ctrl.Manager) error {
	bldr := ctrl.NewControllerManagedBy(mgr)
	r.setResourceWatches(bldr, r.clusterResourceMapper, r.tlsSecretMapper, r.namespaceResourceMapper, r.clusterSecretResourceMapper, r.applicationSetSCMTLSConfigMapMapper)
	return bldr.Complete(r)
}
