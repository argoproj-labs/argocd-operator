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

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/appcontroller"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/applicationset"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/configmap"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/notifications"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/redis"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/reposerver"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/secret"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/server"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/sso"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// blank assignment to verify that ArgoCDReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &ArgoCDReconciler{}

// ArgoCDReconciler reconciles a ArgoCD object
type ArgoCDReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Instance      *argoproj.ArgoCD
	ClusterScoped bool
	Logger        logr.Logger

	ResourceManagedNamespaces map[string]string
	AppManagedNamespaces      map[string]string

	SecretController        *secret.SecretReconciler
	ConfigMapController     *configmap.ConfigMapReconciler
	RedisController         *redis.RedisReconciler
	ReposerverController    *reposerver.RepoServerReconciler
	ServerController        *server.ServerReconciler
	NotificationsController *notifications.NotificationsReconciler
	AppController           *appcontroller.AppControllerReconciler
	AppsetController        *applicationset.ApplicationSetReconciler
	SSOController           *sso.SSOReconciler
}

var log = ctrl.Log.WithName("controller_argocd")

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
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheuses;prometheusrules;servicemonitors,verbs=*
//+kubebuilder:rbac:groups=route.openshift.io,resources=routes;routes/custom-host,verbs=*
//+kubebuilder:rbac:groups=argoproj.io,resources=applications;appprojects,verbs=*
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=*,verbs=*
//+kubebuilder:rbac:groups="",resources=pods;pods/log,verbs=get
//+kubebuilder:rbac:groups=template.openshift.io,resources=templates;templateinstances;templateconfigs,verbs=*
//+kubebuilder:rbac:groups="oauth.openshift.io",resources=oauthclients,verbs=get;list;watch;create;delete;patch;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the ArgoCD object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *ArgoCDReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	argocdControllerLog := ctrl.Log.WithName("argocd-controller")

	reconcileStartTS := time.Now()
	defer func() {
		ReconcileTime.WithLabelValues(request.Namespace).Observe(time.Since(reconcileStartTS).Seconds())
	}()

	argocd := &argoproj.ArgoCD{}
	err := r.Client.Get(ctx, request.NamespacedName, argocd)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
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

	r.Instance = argocd
	r.ClusterScoped = IsClusterConfigNs(r.Instance.Namespace)
	r.Logger = argocdControllerLog.WithValues("instance", r.Instance.Name, "instance-namespace", r.Instance.Namespace)

	if r.Instance.GetDeletionTimestamp() != nil {

		// Argo CD instance marked for deletion; remove entry from activeInstances map and decrement active instance count
		// by phase as well as total
		delete(ActiveInstanceMap, r.Instance.Namespace)
		ActiveInstancesByPhase.WithLabelValues(newPhase).Dec()
		ActiveInstancesTotal.Dec()
		ActiveInstanceReconciliationCount.DeleteLabelValues(r.Instance.Namespace)
		ReconcileTime.DeletePartialMatch(prometheus.Labels{"namespace": r.Instance.Namespace})

		if r.Instance.IsDeletionFinalizerPresent() {
			if err := r.deleteClusterResources(r.Instance); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to delete ClusterResources: %w", err)
			}

			if isRemoveManagedByLabelOnArgoCDDeletion() {
				if err := r.removeManagedByLabelFromNamespaces(r.Instance.Namespace); err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to remove label from namespace[%v], error: %w", r.Instance.Namespace, err)
				}
			}

			if err := r.removeUnmanagedSourceNamespaceResources(r.Instance); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to remove resources from sourceNamespaces, error: %w", err)
			}

			if err := r.removeDeletionFinalizer(r.Instance); err != nil {
				return reconcile.Result{}, err
			}

			// remove namespace of deleted Argo CD instance from deprecationEventEmissionTracker (if exists) so that if another instance
			// is created in the same namespace in the future, that instance is appropriately tracked
			if _, ok := DeprecationEventEmissionTracker[r.Instance.Namespace]; ok {
				delete(DeprecationEventEmissionTracker, r.Instance.Namespace)
			}
		}
		return reconcile.Result{}, nil
	}

	if !r.Instance.IsDeletionFinalizerPresent() {
		if err := r.addDeletionFinalizer(r.Instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	if err = r.setResourceManagedNamespaces(); err != nil {
		return reconcile.Result{}, err
	}

	if err = r.setAppManagedNamespaces(); err != nil {
		return reconcile.Result{}, err
	}

	// if err := r.reconcileResources(r.Instance); err != nil {
	// 	// Error reconciling ArgoCD sub-resources - requeue the request.
	// 	return reconcile.Result{}, err
	// }

	r.InitializeControllerReconcilers()

	if err = r.reconcileControllers(); err != nil {
		return reconcile.Result{}, err
	}

	// Return and don't requeue
	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ArgoCDReconciler) SetupWithManager(mgr ctrl.Manager) error {
	bldr := ctrl.NewControllerManagedBy(mgr)
	r.setResourceWatches(bldr, r.clusterResourceMapper, r.tlsSecretMapper, r.namespaceResourceMapper, r.clusterSecretResourceMapper, r.applicationSetSCMTLSConfigMapMapper)
	return bldr.Complete(r)
}

func (r *ArgoCDReconciler) reconcileControllers() error {

	// core components, return reconciliation errors
	if err := r.SecretController.Reconcile(); err != nil {
		r.Logger.Error(err, "failed to reconcile secret controller")
		return err
	}

	if err := r.ConfigMapController.Reconcile(); err != nil {
		r.Logger.Error(err, "failed to reconcile configmap controller")
		return err
	}

	if err := r.AppController.Reconcile(); err != nil {
		r.Logger.Error(err, "failed to reconcile application controller")
		return err
	}

	if err := r.ServerController.Reconcile(); err != nil {
		r.Logger.Error(err, "failed to reconcile server")
		return err
	}

	if err := r.RedisController.Reconcile(); err != nil {
		r.Logger.Error(err, "failed to reconcile redis controller")
		return err
	}

	if err := r.ReposerverController.Reconcile(); err != nil {
		r.Logger.Error(err, "failed to reconcile reposerver controller")
		return err
	}

	// non-core components, don't return reconciliation errors
	if err := r.AppsetController.Reconcile(); err != nil {
		r.Logger.Error(err, "failed to reconcile applicationset controller")
	}

	if r.Instance.Spec.Notifications.Enabled {
		if err := r.NotificationsController.Reconcile(); err != nil {
			r.Logger.Error(err, "failed to reconcile notifications controller")
		}
	} else {
		if err := r.NotificationsController.DeleteResources(); err != nil {
			r.Logger.Error(err, "failed to delete notifications resources")
		}
	}

	if err := r.SSOController.Reconcile(); err != nil {
		r.Logger.Error(err, "failed to reconcile SSO controller")
	}

	return nil
}

func (r *ArgoCDReconciler) InitializeControllerReconcilers() {
	r.SecretController = &secret.SecretReconciler{
		Client:            &r.Client,
		Scheme:            r.Scheme,
		Instance:          r.Instance,
		ClusterScoped:     r.ClusterScoped,
		ManagedNamespaces: r.ResourceManagedNamespaces,
	}

	r.ConfigMapController = &configmap.ConfigMapReconciler{
		Client:   &r.Client,
		Scheme:   r.Scheme,
		Instance: r.Instance,
	}

	r.RedisController = &redis.RedisReconciler{
		Client:   &r.Client,
		Scheme:   r.Scheme,
		Instance: r.Instance,
	}

	r.ReposerverController = &reposerver.RepoServerReconciler{
		Client:   &r.Client,
		Scheme:   r.Scheme,
		Instance: r.Instance,
	}

	r.ServerController = &server.ServerReconciler{
		Client:            &r.Client,
		Scheme:            r.Scheme,
		Instance:          r.Instance,
		ClusterScoped:     r.ClusterScoped,
		ManagedNamespaces: r.ResourceManagedNamespaces,
		SourceNamespaces:  r.AppManagedNamespaces,
	}

	r.NotificationsController = &notifications.NotificationsReconciler{
		Client:   r.Client,
		Scheme:   r.Scheme,
		Instance: r.Instance,
	}

	r.AppController = &appcontroller.AppControllerReconciler{
		Client:            &r.Client,
		Scheme:            r.Scheme,
		Instance:          r.Instance,
		ClusterScoped:     r.ClusterScoped,
		ManagedNamespaces: r.ResourceManagedNamespaces,
		SourceNamespaces:  r.AppManagedNamespaces,
	}

	r.AppsetController = &applicationset.ApplicationSetReconciler{
		Client:   r.Client,
		Scheme:   r.Scheme,
		Instance: r.Instance,
	}

	r.SSOController = &sso.SSOReconciler{
		Client:   &r.Client,
		Scheme:   r.Scheme,
		Instance: r.Instance,
	}
}

// setResourceManagedNamespaces finds all namespaces being managed by a namespace-scoped Argo CD instance
func (r *ArgoCDReconciler) setResourceManagedNamespaces() error {
	r.ResourceManagedNamespaces = make(map[string]string)
	listOptions := []client.ListOption{
		client.MatchingLabels{
			common.ArgoCDArgoprojKeyManagedBy: r.Instance.Namespace,
		},
	}

	// get the list of namespaces managed by the Argo CD instance
	Managednamespaces, err := cluster.ListNamespaces(r.Client, listOptions)
	if err != nil {
		r.Logger.Error(err, "failed to retrieve list of managed namespaces")
		return err
	}

	r.Logger.Info("processing namespaces for resource management")

	for _, namespace := range Managednamespaces.Items {
		r.ResourceManagedNamespaces[namespace.Name] = ""
	}

	// get control plane namespace
	_, err = cluster.GetNamespace(r.Instance.Namespace, r.Client)
	if err != nil {
		r.Logger.Error(err, "failed to retrieve control plane namespace")
		return err
	}

	// append control-plane namespace to this map
	r.ResourceManagedNamespaces[r.Instance.Namespace] = ""
	return nil
}

// setAppManagedNamespaces sets a list of namespaces that a cluster-scoped Argo CD
// instance is allowed to source Applications from
func (r *ArgoCDReconciler) setAppManagedNamespaces() error {
	r.AppManagedNamespaces = make(map[string]string)
	allowedSourceNamespaces := make(map[string]string)

	if !r.ClusterScoped {
		r.Logger.V(1).Info("setSourceNamespaces: instance is not cluster scoped, skip processing namespaces for application management")
		return nil
	}

	r.Logger.Info("processing namespaces for application management")

	// Get list of existing namespaces currently carrying the ArgoCDAppsManagedBy label and convert to a map
	listOptions := []client.ListOption{
		client.MatchingLabels{
			common.ArgoCDArgoprojKeyManagedByClusterArgoCD: r.Instance.Namespace,
		},
	}

	existingManagedNamespaces, err := cluster.ListNamespaces(r.Client, listOptions)
	if err != nil {
		r.Logger.Error(err, "setSourceNamespaces: failed to list namespaces")
		return err
	}
	existingManagedNsMap := make(map[string]string)
	for _, ns := range existingManagedNamespaces.Items {
		existingManagedNsMap[ns.Name] = ""
	}

	// Get list of desired namespaces that should be carrying the ArgoCDAppsManagedBy label and convert to a map
	desiredManagedNsMap := make(map[string]string)
	for _, ns := range r.Instance.Spec.SourceNamespaces {
		desiredManagedNsMap[ns] = ""
	}

	// check if any of the desired namespaces are missing the label. If yes, add ArgoCDArgoprojKeyManagedByClusterArgoCD to it
	for _, desiredNs := range r.Instance.Spec.SourceNamespaces {
		if _, ok := existingManagedNsMap[desiredNs]; !ok {
			ns, err := cluster.GetNamespace(desiredNs, r.Client)
			if err != nil {
				r.Logger.Error(err, "setSourceNamespaces: failed to retrieve namespace", "name", ns.Name)
				continue
			}

			// sanity check
			if len(ns.Labels) == 0 {
				ns.Labels = make(map[string]string)
			}
			// check if desired namespace is already being managed by a different cluster scoped Argo CD instance. If yes, skip it
			// If not, add ArgoCDArgoprojKeyManagedByClusterArgoCD to it and add it to allowedSourceNamespaces
			if val, ok := ns.Labels[common.ArgoCDArgoprojKeyManagedByClusterArgoCD]; ok && val != r.Instance.Namespace {
				r.Logger.V(1).Info("setSourceNamespaces: skipping namespace as it is already managed by a different instance", "namespace", ns.Name, "managing-instance-namespace", val)
				continue
			} else {
				ns.Labels[common.ArgoCDArgoprojKeyManagedByClusterArgoCD] = r.Instance.Namespace
				allowedSourceNamespaces[desiredNs] = ""
			}
			err = cluster.UpdateNamespace(ns, r.Client)
			if err != nil {
				r.Logger.Error(err, "setSourceNamespaces: failed to update namespace", "namespace", ns.Name)
				continue
			}
			r.Logger.V(1).Info("setSourceNamespaces: labeled namespace", "namespace", ns.Name)
			continue
		}
		allowedSourceNamespaces[desiredNs] = ""
		continue
	}

	// check if any of the exisiting namespaces are carrying the label when they should not be. If yes, remove it
	for existingNs, _ := range existingManagedNsMap {
		if _, ok := desiredManagedNsMap[existingNs]; !ok {
			ns, err := cluster.GetNamespace(existingNs, r.Client)
			if err != nil {
				r.Logger.Error(err, "setSourceNamespaces: failed to retrieve namespace", "name", ns.Name)
				continue
			}
			delete(ns.Labels, common.ArgoCDArgoprojKeyManagedByClusterArgoCD)
			err = cluster.UpdateNamespace(ns, r.Client)
			if err != nil {
				r.Logger.Error(err, "setSourceNamespaces: failed to update namespace", "namespace", ns.Name)
				continue
			}
			r.Logger.V(1).Info("setSourceNamespaces: unlabeled namespace", "namespace", ns.Name)
			continue
		}
	}

	r.AppManagedNamespaces = allowedSourceNamespaces
	return nil
}
