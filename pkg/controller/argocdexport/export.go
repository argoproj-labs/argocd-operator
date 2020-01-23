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
	"context"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
)

// reconcileExport will ensure that the resources for the export process are present for the ArgoCDExport.
func (r *ReconcileArgoCDExport) reconcileExport(cr *argoprojv1a1.ArgoCDExport) error {
	if cr.Spec.Schedule != nil && len(*cr.Spec.Schedule) > 0 {
		log.Info("reconciling export cronjob")
		if err := r.reconcileCronJob(cr); err != nil {
			return err
		}
	} else {
		log.Info("reconciling export job")
		if err := r.reconcileJob(cr); err != nil {
			return err
		}
	}
	return nil
}

// validateExport will ensure that the given ArgoCDExport is valid.
func (r *ReconcileArgoCDExport) validateExport(cr *argoprojv1alpha1.ArgoCDExport) error {
	if len(cr.Status.Phase) <= 0 {
		cr.Status.Phase = "Pending"
		return r.client.Status().Update(context.TODO(), cr)
	}
	return nil
}
