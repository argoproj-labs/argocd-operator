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
	"strings"
	"time"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/appcontroller"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/applicationset"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/notifications"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/redis"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/reposerver"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/server"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/sso"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/sso/dex"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/monitoring"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/argoproj/argo-cd/v2/util/glob"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ArgoCDController = "argocd-controller"
)

const (
	grafanaDeprecationWarning    = "warning: grafana is deprecated from ArgoCD; field will be ignored"
	prometheusDeprecationWarning = "warning: prometheus is deprecated from ArgoCD; field will be ignored"
)

var (
	caResourceName string
)

func (r *ArgoCDReconciler) varSetter() {
	caResourceName = argoutil.GenerateResourceName(r.Instance.Name, common.ArgoCDCASuffix)
}

// blank assignment to verify that ArgoCDReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &ArgoCDReconciler{}

// ArgoCDReconciler reconciles a ArgoCD object
// TODO(upgrade): rename to ArgoCDRecoonciler
type ReconcileArgoCD struct {
	client.Client
	Scheme            *runtime.Scheme
	ManagedNamespaces *corev1.NamespaceList
	// Stores a list of SourceNamespaces as values
	ManagedSourceNamespaces map[string]string
	// Stores label selector used to reconcile a subset of ArgoCD
	LabelSelector string
}

// ArgoCDReconciler reconciles a ArgoCD object
type ArgoCDReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Instance      *argoproj.ArgoCD
	ClusterScoped bool
	Logger        *util.Logger

	ResourceManagedNamespaces map[string]string
	AppSourceNamespaces       map[string]string
	AppsetSourceNamespaces    map[string]string

	// Stores label selector used to reconcile a subset of ArgoCD
	LabelSelector string

	RedisController         *redis.RedisReconciler
	ReposerverController    *reposerver.RepoServerReconciler
	ServerController        *server.ServerReconciler
	NotificationsController *notifications.NotificationsReconciler
	AppController           *appcontroller.AppControllerReconciler
	AppsetController        *applicationset.ApplicationSetReconciler
	SSOController           *sso.SSOReconciler
}

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
//+kubebuilder:rbac:groups=argoproj.io,resources=notificationsconfigurations;notificationsconfigurations/finalizers,verbs=*

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the ArgoCD object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile

func (r *ArgoCDReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {

	reconcileStartTS := time.Now()
	defer func() {
		ReconcileTime.WithLabelValues(request.Namespace).Observe(time.Since(reconcileStartTS).Seconds())
	}()

	argocd := &argoproj.ArgoCD{}
	err := r.Client.Get(ctx, request.NamespacedName, argocd)
	if err != nil {
		if apierrors.IsNotFound(err) {
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
	r.Logger = util.NewLogger(ArgoCDController, "instance", r.Instance.Name, "instance-namespace", r.Instance.Namespace)

	// Fetch labelSelector from r.LabelSelector (command-line option)
	labelSelector, err := labels.Parse(r.LabelSelector)
	if err != nil {
		r.Logger.Info(fmt.Sprintf("error parsing the labelSelector '%s'.", labelSelector))
		return reconcile.Result{}, err
	}
	// Match the value of labelSelector from ReconcileArgoCD to labels from the argocd instance
	if !labelSelector.Matches(labels.Set(r.Instance.Labels)) {
		r.Logger.Info(fmt.Sprintf("the ArgoCD instance '%s' does not match the label selector '%s' and skipping for reconciliation", request.NamespacedName, r.LabelSelector))
		return reconcile.Result{}, fmt.Errorf("error: failed to reconcile ArgoCD instance: '%s'", request.NamespacedName)
	}

	if r.Instance.GetDeletionTimestamp() != nil {

		// Argo CD instance marked for deletion; remove entry from activeInstances map and decrement active instance count by phase as well as total
		delete(ActiveInstanceMap, r.Instance.Namespace)
		ActiveInstancesByPhase.WithLabelValues(newPhase).Dec()
		ActiveInstancesTotal.Dec()
		ActiveInstanceReconciliationCount.DeleteLabelValues(r.Instance.Namespace)
		ReconcileTime.DeletePartialMatch(prometheus.Labels{"namespace": r.Instance.Namespace})

		if r.Instance.IsDeletionFinalizerPresent() {
			r.deleteClusterResources()

			r.removeManagedNamespaceLabels()

			if err := r.removeDeletionFinalizer(); err != nil {
				return reconcile.Result{}, err
			}
		}
		return reconcile.Result{}, nil
	}

	if !r.Instance.IsDeletionFinalizerPresent() {
		if err := r.addDeletionFinalizer(); err != nil {
			return reconcile.Result{}, err
		}
	}

	if err = r.setResourceManagedNamespaces(); err != nil {
		return reconcile.Result{}, err
	}
	if err = r.setAppSourceNamespaces(); err != nil {
		return reconcile.Result{}, err
	}

	if err = r.setAppsetSourceNamespaces(); err != nil {
		return reconcile.Result{}, err
	}

	r.InitializeControllerReconcilers()

	r.varSetter()

	if err = r.reconcileControllers(); err != nil {
		return reconcile.Result{}, err
	}

	// Return and don't requeue
	return reconcile.Result{}, nil
}

// setResourceManagedNamespaces finds all namespaces carrying the managed-by label, adds the control plane namespace and stores that list in ArgoCDReconciler to be accessed later
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

// getAllowedAppSourceNsList calculates the list of allowed app source namespaces for the given instance
// It detects conflicts if a newly specified namespace is already being managed by a different cluster scoped
// or namespace scoped instance
func (r *ArgoCDReconciler) getAllowedAppSourceNsList() ([]string, error) {
	sourceNamespaces := []string{}

	// retrieve all cluster namespaces
	nsList, err := cluster.ListNamespaces(r.Client, []client.ListOption{})
	if err != nil {
		return []string{}, errors.Wrap(err, "getAllowedAppSourceNsList: failed to retrieve cluster namespaces")
	}

	for _, namespace := range nsList.Items {
		if glob.MatchStringInList(r.Instance.Spec.SourceNamespaces, namespace.Name, false) {
			// namespace matches request source ns criteria
			// reject ns if already managed by a different instance
			if val, ok := namespace.Labels[common.ArgoCDArgoprojKeyManagedBy]; ok && val != r.Instance.Namespace {
				r.Logger.Debug("getAllowedAppSourceNsList: skipping namespace %s as it is already managed by a different Argo CD instance")
				continue
			}

			if val, ok := namespace.Labels[common.ArgoCDArgoprojKeyAppsManagedBy]; ok && val != r.Instance.Namespace {
				r.Logger.Debug("getAllowedAppSourceNsList: skipping namespace %s as it is already managed by a different Argo CD instance")
				continue
			}

			// TO DO: future update - add exclusion list check here

			sourceNamespaces = append(sourceNamespaces, namespace.Name)
		}
	}
	return sourceNamespaces, nil
}

// setAppSourceNamespaces sets and updates the list of namespaces that a cluster-scoped Argo CD instance is allowed
// to source Applications from. It is responsible for keeping cluster namespace labels in
// sync with the list provided in the Argo CD CR.
func (r *ArgoCDReconciler) setAppSourceNamespaces() error {
	r.AppSourceNamespaces = make(map[string]string)

	if !r.ClusterScoped {
		r.Logger.Debug("setAppSourceNamespaces: instance is not cluster scoped, skip processing namespaces for application management")
		return nil
	}

	// Get list of existing namespaces currently carrying the ArgoCDAppsManagedBy label and convert to a map
	listOptions := []client.ListOption{
		client.MatchingLabels{
			common.ArgoCDArgoprojKeyAppsManagedBy: r.Instance.Namespace,
		},
	}
	existingAppSourceNamespaces, err := cluster.ListNamespaces(r.Client, listOptions)
	if err != nil {
		r.Logger.Error(err, "setAppSourceNamespaces: failed to list namespaces")
		return err
	}
	existingAppSrcNsMap := make(map[string]string)
	for _, ns := range existingAppSourceNamespaces.Items {
		existingAppSrcNsMap[ns.Name] = ""
	}

	// Get list of allowed namespaces that should be carrying the ArgoCDAppsManagedBy label and convert to a map
	allowedAppSrcNamespaces, err := r.getAllowedAppSourceNsList()
	if err != nil {
		return errors.Wrap(err, "setAppSourceNamespaces: failed to list requested source namespaces")
	}
	allowedAppSrcNsMap := util.StringSliceToMap(allowedAppSrcNamespaces)

	// calculate new source namespaces by performing allowed - existing
	newSourceNsMap := util.SetDiff(allowedAppSrcNsMap, existingAppSrcNsMap)

	// calculate obsolete source namespaces by performing existing - allowed
	obsoleteSourceNsMap := util.SetDiff(existingAppSrcNsMap, allowedAppSrcNsMap)

	// label the new namespaces
	for newNs := range newSourceNsMap {
		ns, err := cluster.GetNamespace(newNs, r.Client)
		if err != nil {
			r.Logger.Error(err, "setAppSourceNamespaces: failed to retrieve namespace", "name", ns.Name)
			continue
		}
		// sanity check
		if len(ns.Labels) == 0 {
			ns.Labels = make(map[string]string)
		}
		ns.Labels[common.ArgoCDArgoprojKeyAppsManagedBy] = r.Instance.Namespace
		if err := cluster.UpdateNamespace(ns, r.Client); err != nil {
			r.Logger.Error(err, "setAppSourceNamespaces: failed to update namespace", "namespace", ns.Name)
			continue
		}
		r.Logger.Debug("setAppSourceNamespaces: labeled namespace", "namespace", ns.Name)
		continue
	}

	// unlabel obsolete namespaces
	for oldNs := range obsoleteSourceNsMap {
		ns, err := cluster.GetNamespace(oldNs, r.Client)
		if err != nil {
			r.Logger.Error(err, "setSourceNamespaces: failed to retrieve namespace", "name", ns.Name)
			continue
		}
		delete(ns.Labels, common.ArgoCDArgoprojKeyAppsManagedBy)
		if err := cluster.UpdateNamespace(ns, r.Client); err != nil {
			r.Logger.Error(err, "setSourceNamespaces: failed to update namespace", "namespace", ns.Name)
			continue
		}
		r.Logger.Debug("setSourceNamespaces: unlabeled namespace", "namespace", ns.Name)
		continue
	}

	r.AppSourceNamespaces = allowedAppSrcNsMap
	return nil
}

func (r *ArgoCDReconciler) getAllowedAppsetSourceNsList() ([]string, error) {
	sourceNamespaces := []string{}

	// Get list of existing app source namespaces. Set of Appset source namespaces
	// should be a subset of this set
	listOptions := []client.ListOption{
		client.MatchingLabels{
			common.ArgoCDArgoprojKeyAppsManagedBy: r.Instance.Namespace,
		},
	}

	existingAppSourceNamespaces, err := cluster.ListNamespaces(r.Client, listOptions)
	if err != nil {
		r.Logger.Error(err, "getDesiredAppsetSourceNsList: failed to list app source namespaces")
		return nil, err
	}

	for _, namespace := range existingAppSourceNamespaces.Items {
		if glob.MatchStringInList(r.Instance.Spec.ApplicationSet.SourceNamespaces, namespace.Name, false) {
			// TO DO: future update - add exclusion list check here
			sourceNamespaces = append(sourceNamespaces, namespace.Name)
		}
	}
	return sourceNamespaces, nil
}

// setAppsetSourceNamespaces sets and updates the list of namespaces that a cluster-scoped Argo CD instance is allowed
// to source Applicationsets from. It is responsible for keeping cluster namespace labels in
// sync with the list provided in the Argo CD CR.
func (r *ArgoCDReconciler) setAppsetSourceNamespaces() error {
	r.AppsetSourceNamespaces = make(map[string]string)

	if !r.ClusterScoped {
		r.Logger.Debug("setAppsetSourceNamespaces: instance is not cluster scoped, skip processing namespaces for applicationset management")
		return nil
	}

	// Get list of existing namespaces currently carrying the ArgoCDAppSetsManagedBy label and convert to a map
	listOptions := []client.ListOption{
		client.MatchingLabels{
			common.ArgoCDArgoprojKeyAppSetsManagedBy: r.Instance.Namespace,
		},
	}
	existingAppsetSourceNamespaces, err := cluster.ListNamespaces(r.Client, listOptions)
	if err != nil {
		r.Logger.Error(err, "setAppsetSourceNamespaces: failed to list namespaces")
		return err
	}
	existingAppsetSrcNsMap := make(map[string]string)
	for _, ns := range existingAppsetSourceNamespaces.Items {
		existingAppsetSrcNsMap[ns.Name] = ""
	}

	// Get list of allowed namespaces that should be carrying the ArgoCDAppSetsManagedBy label and convert to a map
	allowedAppsetSrcNamespaces, err := r.getAllowedAppsetSourceNsList()
	if err != nil {
		return errors.Wrap(err, "setAppsetSourceNamespaces: failed to list requested appset source namespaces")
	}
	allowedAppsetSrcNsMap := util.StringSliceToMap(allowedAppsetSrcNamespaces)

	// calculate new source namespaces by performing allowed - existing
	newSourceNsMap := util.SetDiff(allowedAppsetSrcNsMap, existingAppsetSrcNsMap)
	// calculate obsolete source namespaces by performing existing - allowed
	obsoleteSourceNsMap := util.SetDiff(existingAppsetSrcNsMap, allowedAppsetSrcNsMap)

	// label the new namespaces
	for newNs := range newSourceNsMap {
		ns, err := cluster.GetNamespace(newNs, r.Client)
		if err != nil {
			r.Logger.Error(err, "setAppsetSourceNamespaces: failed to retrieve namespace", "name", ns.Name)
			continue
		}
		// sanity check
		if len(ns.Labels) == 0 {
			ns.Labels = make(map[string]string)
		}
		ns.Labels[common.ArgoCDArgoprojKeyAppSetsManagedBy] = r.Instance.Namespace
		if err := cluster.UpdateNamespace(ns, r.Client); err != nil {
			r.Logger.Error(err, "setAppsetSourceNamespaces: failed to update namespace", "namespace", ns.Name)
			continue
		}
		r.Logger.Debug("setAppsetSourceNamespaces: labeled namespace", "namespace", ns.Name)
		continue
	}

	// unlabel obsolete namespaces
	for oldNs := range obsoleteSourceNsMap {
		ns, err := cluster.GetNamespace(oldNs, r.Client)
		if err != nil {
			r.Logger.Error(err, "setAppsetSourceNamespaces: failed to retrieve namespace", "name", ns.Name)
			continue
		}
		delete(ns.Labels, common.ArgoCDArgoprojKeyAppSetsManagedBy)
		if err := cluster.UpdateNamespace(ns, r.Client); err != nil {
			r.Logger.Error(err, "setAppsetSourceNamespaces: failed to update namespace", "namespace", ns.Name)
			continue
		}
		r.Logger.Debug("setAppsetSourceNamespaces: unlabeled namespace", "namespace", ns.Name)
		continue
	}

	r.AppsetSourceNamespaces = allowedAppsetSrcNsMap
	return nil
}

func (r *ArgoCDReconciler) reconcileControllers() error {

	if err := r.reconcileSecrets(); err != nil {
		r.Logger.Error(err, "failed to reconcile required secrets")
		return err
	}

	if err := r.reconcileConfigMaps(); err != nil {
		r.Logger.Error(err, "failed to reconcile required config maps")
		return err
	}

	if r.Instance.Spec.Controller.IsEnabled() {
		if err := r.AppController.Reconcile(); err != nil {
			r.Logger.Error(err, "failed to reconcile application controller")
			return err
		}
	} else {
		if err := r.AppController.DeleteResources(); err != nil {
			r.Logger.Error(err, "failed to delete app controller resources")
		}
	}

	if r.Instance.Spec.Server.IsEnabled() {
		if err := r.ServerController.Reconcile(); err != nil {
			r.Logger.Error(err, "failed to reconcile server controller")
			return err
		}
	} else {
		if err := r.ServerController.DeleteResources(); err != nil {
			r.Logger.Error(err, "failed to delete server resources")
		}
	}

	if r.Instance.Spec.Redis.IsEnabled() {
		if err := r.RedisController.Reconcile(); err != nil {
			r.Logger.Error(err, "failed to reconcile redis controller")
			return err
		}
	} else {
		if err := r.RedisController.DeleteResources(); err != nil {
			r.Logger.Error(err, "failed to delete redis resources")
			return err
		}
	}

	if r.Instance.Spec.Repo.IsEnabled() {
		if err := r.ReposerverController.Reconcile(); err != nil {
			r.Logger.Error(err, "failed to reconcile repo-server controller")
			return err
		}
	} else {
		if err := r.ReposerverController.DeleteResources(); err != nil {
			r.Logger.Error(err, "failed to delete repo-server resources")
		}
	}

	if r.Instance.Spec.ApplicationSet != nil && r.Instance.Spec.ApplicationSet.IsEnabled() {
		if err := r.AppsetController.Reconcile(); err != nil {
			r.Logger.Error(err, "failed to reconcile applicationset controller")
		}
	} else {
		if err := r.AppsetController.DeleteResources(); err != nil {
			r.Logger.Error(err, "failed to delete applicationset resources")
		}
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

	if r.Instance.Spec.Grafana.Enabled {
		r.Logger.Info(grafanaDeprecationWarning)
	}

	if r.Instance.Spec.Prometheus.Enabled {
		r.Logger.Info(prometheusDeprecationWarning)
	}

	if monitoring.IsPrometheusAPIAvailable() {
		if r.Instance.Spec.Monitoring.Enabled {
			if err := r.reoncilePrometheusRule(); err != nil {
				r.Logger.Error(err, "failed to reconcile prometheusRule")
			}
		} else {
			if err := r.deletePrometheusRule(common.ArogCDComponentStatusAlertRuleName, r.Instance.Namespace); err != nil {
				r.Logger.Error(err, "failed to delete prometheusRule")
			}
		}
	}

	if r.Instance.Spec.SSO != nil {
		if err := r.SSOController.Reconcile(); err != nil {
			r.Logger.Error(err, "failed to reconcile SSO")
		}
	}

	if err := r.reconcileStatus(); err != nil {
		return err
	}

	return nil
}

func (r *ArgoCDReconciler) InitializeControllerReconcilers() {

	redisController := &redis.RedisReconciler{
		Client:   r.Client,
		Scheme:   r.Scheme,
		Instance: r.Instance,
		Logger:   util.NewLogger(common.RedisController, "instance", r.Instance.Name, "instance-namespace", r.Instance.Namespace),
	}

	reposerverController := &reposerver.RepoServerReconciler{
		Client:   r.Client,
		Scheme:   r.Scheme,
		Instance: r.Instance,
		Logger:   util.NewLogger(common.RepoServerController, "instance", r.Instance.Name, "instance-namespace", r.Instance.Namespace),
	}

	serverController := &server.ServerReconciler{
		Client:                 r.Client,
		Scheme:                 r.Scheme,
		Instance:               r.Instance,
		Logger:                 util.NewLogger(common.ServerController, "instance", r.Instance.Name, "instance-namespace", r.Instance.Namespace),
		ClusterScoped:          r.ClusterScoped,
		ManagedNamespaces:      r.ResourceManagedNamespaces,
		SourceNamespaces:       r.AppSourceNamespaces,
		AppsetSourceNamespaces: r.AppSourceNamespaces,
	}

	notificationsController := &notifications.NotificationsReconciler{
		Client:   r.Client,
		Scheme:   r.Scheme,
		Instance: r.Instance,
		Logger:   util.NewLogger(common.NotificationsController, "instance", r.Instance.Name, "instance-namespace", r.Instance.Namespace),
	}

	appController := &appcontroller.AppControllerReconciler{
		Client:            r.Client,
		Scheme:            r.Scheme,
		Instance:          r.Instance,
		ClusterScoped:     r.ClusterScoped,
		ManagedNamespaces: r.ResourceManagedNamespaces,
		SourceNamespaces:  r.AppSourceNamespaces,
		Logger:            util.NewLogger(common.AppControllerComponent, "instance", r.Instance.Name, "instance-namespace", r.Instance.Namespace),
	}

	appsetController := &applicationset.ApplicationSetReconciler{
		Client:                 r.Client,
		Scheme:                 r.Scheme,
		Instance:               r.Instance,
		ClusterScoped:          r.ClusterScoped,
		AppsetSourceNamespaces: r.AppsetSourceNamespaces,
		Logger:                 util.NewLogger(common.AppSetControllerComponent, "instance", r.Instance.Name, "instance-namespace", r.Instance.Namespace),
	}

	ssoController := &sso.SSOReconciler{
		Client:   r.Client,
		Scheme:   r.Scheme,
		Instance: r.Instance,
		Logger:   util.NewLogger(common.SSOController, "instance", r.Instance.Name, "instance-namespace", r.Instance.Namespace),
	}

	dexController := &dex.DexReconciler{
		Client:   r.Client,
		Scheme:   r.Scheme,
		Instance: r.Instance,
		Logger:   *ssoController.Logger,
		Server:   serverController,
	}

	r.SSOController = ssoController
	r.SSOController.DexController = dexController

	r.AppController = appController

	r.ServerController = serverController
	r.ServerController.Redis = redisController
	r.ServerController.RepoServer = reposerverController
	// TODO: use sso abstraction
	r.ServerController.SSO = ssoController

	r.ReposerverController = reposerverController
	r.ReposerverController.Appcontroller = appController
	r.ReposerverController.Server = serverController
	r.ReposerverController.Redis = redisController

	r.AppsetController = appsetController
	r.AppsetController.RepoServer = reposerverController

	r.RedisController = redisController
	r.RedisController.Appcontroller = appController
	r.RedisController.Server = serverController
	r.RedisController.RepoServer = reposerverController

	r.AppController.Redis = redisController
	r.AppController.RepoServer = reposerverController

	r.NotificationsController = notificationsController

}

// SetupWithManager sets up the controller with the Manager.
func (r *ArgoCDReconciler) SetupWithManager(mgr ctrl.Manager) error {
	bldr := ctrl.NewControllerManagedBy(mgr)
	r.setResourceWatches(bldr, r.namespaceMapper, r.clusterResourceMapper, r.tlsSecretMapper, r.clusterSecretMapper, r.applicationSetSCMTLSConfigMapMapper)
	return bldr.Complete(r)
}

func (r *ArgoCDReconciler) addDeletionFinalizer() error {
	r.Instance.Finalizers = append(r.Instance.Finalizers, common.ArgoCDDeletionFinalizer)
	if err := resource.UpdateObject(r.Instance, r.Client); err != nil {
		return errors.Wrapf(err, "addDeletionFinalizer: failed to add deletion finalizer for %s", r.Instance.Name)
	}
	return nil
}

func (r *ArgoCDReconciler) removeDeletionFinalizer() error {
	r.Instance.Finalizers = util.RemoveString(r.Instance.GetFinalizers(), common.ArgoCDDeletionFinalizer)
	if err := resource.UpdateObject(r.Instance, r.Client); err != nil {
		return errors.Wrapf(err, "removeDeletionFinalizer: failed to remove deletion finalizer for %s", r.Instance.Name)
	}
	return nil
}

func RemoveManagedByLabelOnDeletion() bool {
	v := util.GetEnv(common.ArgoCDRemoveManagedByLabelOnDeletionEnvVar)
	return strings.ToLower(v) == "true"
}

func (r *ArgoCDReconciler) removeManagedNamespaceLabels() {

	// only remove managed by label from namespaces if explicity asked for. This will change once
	// namespace management has been made to be self service
	if RemoveManagedByLabelOnDeletion() {
		for managedNs := range r.ResourceManagedNamespaces {
			ns, err := cluster.GetNamespace(managedNs, r.Client)
			if err != nil {
				r.Logger.Error(err, "failed to retrieve namespace for deletion", "name", managedNs)
				continue
			}
			if ns.Labels == nil {
				continue
			}
			delete(ns.Labels, common.ArgoCDArgoprojKeyManagedBy)
			if err := cluster.UpdateNamespace(ns, r.Client); err != nil {
				r.Logger.Error(err, "failed to unlabel namespace", "name", managedNs)
			}
		}
	}

	for appSourceNs := range r.AppSourceNamespaces {
		ns, err := cluster.GetNamespace(appSourceNs, r.Client)
		if err != nil {
			r.Logger.Error(err, "failed to retrieve namespace for deletion", "name", appSourceNs)
			continue
		}
		if ns.Labels == nil {
			continue
		}
		delete(ns.Labels, common.ArgoCDArgoprojKeyAppsManagedBy)
		if err := cluster.UpdateNamespace(ns, r.Client); err != nil {
			r.Logger.Error(err, "failed to unlabel namespace", "name", appSourceNs)
		}

	}

	for appsetSourceNs := range r.AppsetSourceNamespaces {
		ns, err := cluster.GetNamespace(appsetSourceNs, r.Client)
		if err != nil {
			r.Logger.Error(err, "failed to retrieve namespace for deletion", "name", appsetSourceNs)
			continue
		}
		if ns.Labels == nil {
			continue
		}
		delete(ns.Labels, common.ArgoCDArgoprojKeyAppSetsManagedBy)
		if err := cluster.UpdateNamespace(ns, r.Client); err != nil {
			r.Logger.Error(err, "failed to unlabel namespace", "name", appsetSourceNs)
		}
	}
}

func (r *ArgoCDReconciler) deleteClusterResources() {
	ClusterRoles := []types.NamespacedName{}
	ClusterRolebindings := []types.NamespacedName{}

	req, err := argocdcommon.GetInstanceLabelRequirement(r.Instance.Namespace)
	if err != nil {
		r.Logger.Error(err, "deleteClusterResources")
	}

	instanceLS := argocdcommon.GetLabelSelector(*req)

	clusterRolBindingList, err := permissions.ListClusterRoles(r.Client, []client.ListOption{
		&client.ListOptions{
			LabelSelector: instanceLS,
		},
	})
	if err != nil {
		r.Logger.Error(err, "deleteClusterResources")
		return
	}
	for _, crb := range clusterRolBindingList.Items {
		ClusterRolebindings = append(ClusterRolebindings, types.NamespacedName{Name: crb.Name})
	}

	for _, crb := range ClusterRolebindings {
		if err := permissions.DeleteClusterRoleBinding(crb.Name, r.Client); err != nil {
			r.Logger.Error(err, "failed to delete clusterrolebinding", "name", crb.Name)
		}
	}

	clusterRoleList, err := permissions.ListClusterRoles(r.Client, []client.ListOption{
		&client.ListOptions{
			LabelSelector: instanceLS,
		},
	})
	if err != nil {
		r.Logger.Error(err, "deleteClusterResources")
		return
	}
	for _, r := range clusterRoleList.Items {
		ClusterRoles = append(ClusterRoles, types.NamespacedName{Name: r.Name, Namespace: r.Namespace})
	}

	for _, cr := range ClusterRoles {
		if err := permissions.DeleteClusterRole(cr.Name, r.Client); err != nil {
			r.Logger.Error(err, "failed to delete clusterrole", "name", cr.Name)
		}
	}
}
