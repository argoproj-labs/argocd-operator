// Copyright 2021 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package argorollouts

import (
	"context"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logr "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// The following are the rollouts controller resources reconciled by the operator.
// kind: ServiceAccount # name: argo-rollouts
// kind: ClusterRole # name: argo-rollouts
// kind: ClusterRole # name: argo-rollouts-aggregate-to-admin
// kind: ClusterRole # name: argo-rollouts-aggregate-to-edit
// kind: ClusterRole # name: argo-rollouts-aggregate-to-view
// kind: ClusterRoleBinding # name: argo-rollouts
// kind: Secret # name: argo-rollouts-notification-secret
// kind: Service # name: argo-rollouts-metrics
// kind: Deployment # name: argo-rollouts

var log = logr.Log.WithName("controller_argorollouts")

// blank assignment to verify that ArgoRolloutsReconciler implements reconcile.Reconciler
var _ reconcile.Reconciler = &ArgoRolloutsReconciler{}

// ArgoRolloutsReconciler reconciles ArgoRollouts object
type ArgoRolloutsReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=argoproj.io,resources=rollouts;rollouts/finalizers;rollouts/status,verbs=get;list;watch;create;delete;patch;update
//+kubebuilder:rbac:groups=argoproj.io,resources=experiments;experiments/finalizers;experiments/status,verbs=get;list;watch;create;delete;patch;update
//+kubebuilder:rbac:groups=argoproj.io,resources=clusteranalysistemplates;clusteranalysistemplates/finalizers;clusteranalysistemplates/status,verbs=get;list;watch;create;delete;patch;update
//+kubebuilder:rbac:groups=argoproj.io,resources=analysistemplates;analysistemplates/finalizers;analysistemplates/status,verbs=get;list;watch;create;delete;patch;update
//+kubebuilder:rbac:groups=argoproj.io,resources=analysisruns;analysisruns/finalizers;analysisruns/status,verbs=get;list;watch;create;delete;patch;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *ArgoRolloutsReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	reqLogger := logr.FromContext(ctx, "Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ArgoRollouts")

	// Fetch the ArgoRollouts instance
	rollouts := &argoproj.ArgoRollouts{}
	err := r.Client.Get(ctx, request.NamespacedName, rollouts)
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

	if err := r.reconcileRolloutsController(rollouts); err != nil {
		// Error reconciling ArgoCDExport sub-resources - requeue the request.
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ArgoRolloutsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	bld := ctrl.NewControllerManagedBy(mgr)
	setResourceWatches(bld)
	return bld.Complete(r)
}
