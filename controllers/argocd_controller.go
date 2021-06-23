/*
Copyright 2021.

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

package controllers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cri-api/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
)

const (
	applicationController = "argocd-application-controller"
	server                = "argocd-server"
	redisHa               = "argocd-redis-ha"
	dexServer             = "argocd-dex-server"
)

// blank assignment to verify that ReconcileArgoCD implements reconcile.Reconciler
var _ reconcile.Reconciler = &ArgoCDReconciler{}

// ArgoCDReconciler reconciles a ArgoCD object
type ArgoCDReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var logr = log.Log.WithName("controller_argocd")

//+kubebuilder:rbac:groups=argoproj.io,resources=argocds,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=argoproj.io,resources=argocds/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=argoproj.io,resources=argocds/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ArgoCD object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *ArgoCDReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx, "namespace", req.Namespace, "name", req.Name)
	reqLogger.Info("Reconciling ArgoCD")

	argocd := &argoproj.ArgoCD{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, argocd)
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

	if argocd.GetDeletionTimestamp() != nil {
		if argocd.IsDeletionFinalizerPresent() {
			if err := r.deleteClusterResources(argocd); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to delete ClusterResources: %w", err)
			}

			if err := r.removeDeletionFinalizer(argocd); err != nil {
				return reconcile.Result{}, err
			}
		}
		return reconcile.Result{}, nil
	}

	if !argocd.IsDeletionFinalizerPresent() {
		if err := r.addDeletionFinalizer(argocd); err != nil {
			return reconcile.Result{}, err
		}
	}

	// get the latest version of argocd instance before reconciling
	if err = r.Client.Get(context.TODO(), req.NamespacedName, argocd); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.reconcileResources(argocd); err != nil {
		// Error reconciling ArgoCD sub-resources - requeue the request.
		return reconcile.Result{}, err
	}

	// Return and don't requeue
	return reconcile.Result{}, nil
	//return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ArgoCDReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&argoproj.ArgoCD{}).
		Complete(r)
}
