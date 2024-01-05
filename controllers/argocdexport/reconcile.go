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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
)

// reconcileArgoCDExportResources will reconcile all ArgoCDExport resources for the give CR.
func (r *ArgoCDExportReconciler) reconcileArgoCDExportResources(cr *argoprojv1alpha1.ArgoCDExport) error {
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

// setResourceWatches will register Watches for each of the supported Resources.
func setResourceWatches(bld *builder.Builder) *builder.Builder {
	// Watch for changes to primary resource ArgoCDExport
	bld.For(&argoprojv1alpha1.ArgoCDExport{})

	// Watch for changes to CronJob sub-resources owned by ArgoCDExport instances.
	bld.Owns(&batchv1.CronJob{})

	// Watch for changes to Job sub-resources owned by ArgoCD instances.
	bld.Owns(&batchv1.Job{})

	// Watch for changes to PersistentVolumeClaim sub-resources owned by ArgoCD instances.
	bld.Owns(&corev1.PersistentVolumeClaim{})

	// Watch for changes to Secret sub-resources owned by ArgoCD instances.
	bld.Owns(&corev1.Secret{})

	return bld
}
