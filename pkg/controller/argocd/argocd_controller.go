// Copyright 2019 ArgoCD Operator Developers
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

package argocd

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	argoproj "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
)

// blank assignment to verify that ReconcileArgoCD implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileArgoCD{}

// ReconcileArgoCD reconciles a ArgoCD object
type ReconcileArgoCD struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

var log = logf.Log.WithName("controller_argocd")

// Add creates a new ArgoCD Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("argocd-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Register watches for all controller resources
	if err := watchResources(c); err != nil {
		return err
	}

	return nil
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileArgoCD{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// Reconcile reads that state of the cluster for a ArgoCD object and makes changes based on the state read
// and what is in the ArgoCD.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileArgoCD) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("namespace", request.Namespace, "name", request.Name)
	reqLogger.Info("Reconciling ArgoCD")

	argocd := &argoproj.ArgoCD{}
	err := r.client.Get(context.TODO(), request.NamespacedName, argocd)
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

	if argocd.ObjectMeta.DeletionTimestamp.IsZero() {
		if err := r.reconcileResources(argocd); err != nil {
			// Error reconciling ArgoCD sub-resources - requeue the request.
			return reconcile.Result{}, err
		}
	}

	// Return and don't requeue
	return reconcile.Result{}, nil
}
