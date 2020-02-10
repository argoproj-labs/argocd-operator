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

package argocdexport

import (
	argoproj "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1b1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// reconcileArgoCDExportResources will reconcile all ArgoCDExport resources for the give CR.
func (r *ReconcileArgoCDExport) reconcileArgoCDExportResources(cr *argoprojv1a1.ArgoCDExport) error {
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

// watchArgoCDExportOwnedResource will register a Watch for a reource owned by an ArgoCDExport.
func watchArgoCDExportOwnedResource(c controller.Controller, obj runtime.Object) error {
	return c.Watch(&source.Kind{Type: obj}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &argoproj.ArgoCDExport{},
	})
}

// watchArgoCDExportResources will register Watches for each of the supported Resources.
func watchArgoCDExportResources(c controller.Controller) error {
	// Watch for changes to primary resource ArgoCDExport
	if err := c.Watch(&source.Kind{Type: &argoproj.ArgoCDExport{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return err
	}

	// Watch for changes to CronJob sub-resources owned by ArgoCDExport instances.
	if err := watchArgoCDExportOwnedResource(c, &batchv1b1.CronJob{}); err != nil {
		return err
	}

	// Watch for changes to Secret sub-resources owned by ArgoCD instances.
	if err := watchArgoCDExportOwnedResource(c, &batchv1.Job{}); err != nil {
		return err
	}

	// Watch for changes to Service sub-resources owned by ArgoCD instances.
	if err := watchArgoCDExportOwnedResource(c, &corev1.PersistentVolumeClaim{}); err != nil {
		return err
	}

	return nil
}
