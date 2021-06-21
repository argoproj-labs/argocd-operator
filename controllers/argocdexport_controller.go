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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cri-api/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
)

// blank assignment to verify that ReconcileArgoCDExport implements reconcile.Reconciler
var _ reconcile.Reconciler = &ArgoCDExportReconciler{}

var exportLogger = log.Log.WithName("controller_argocdexport")

// ArgoCDExportReconciler reconciles a ArgoCDExport object
type ArgoCDExportReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=argoproj.io,resources=argocdexports,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=argoproj.io,resources=argocdexports/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=argoproj.io,resources=argocdexports/finalizers,verbs=update

// Reconcile reads that state of the cluster for a ArgoCDExport object and makes changes based on the state read
// and what is in the ArgoCDExport.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ArgoCDExportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx, "Request.Namespac", req.Namespace, "Request.Name", req.Name)
	reqLogger.Info("Reconciling ArgoCDExport")

	// Fetch the ArgoCDExport instance
	export := &argoproj.ArgoCDExport{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, export)
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

	if err := r.reconcileArgoCDExportResources(export); err != nil {
		// Error reconciling ArgoCDExport sub-resources - requeue the request.
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// reconcileArgoCDExportResources will reconcile all ArgoCDExport resources for the give CR.
func (r *ArgoCDExportReconciler) reconcileArgoCDExportResources(cr *argoproj.ArgoCDExport) error {
	if err := r.validateExport(cr); err != nil {
		return err
	}

	if err := r.reconcileStorage(cr); err != nil {
		return err
	}

	if err := r.reconcileExport(cr); err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ArgoCDExportReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&argoproj.ArgoCDExport{}).
		Complete(r)
}
