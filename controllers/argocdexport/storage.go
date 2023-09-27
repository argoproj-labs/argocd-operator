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

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
)

// reconcileStorage will ensure that the storage options for the ArgoCDExport are present.
func (r *ArgoCDExportReconciler) reconcileStorage(cr *argoprojv1alpha1.ArgoCDExport) error {
	if cr.Spec.Storage == nil {
		cr.Spec.Storage = &argoprojv1alpha1.ArgoCDExportStorageSpec{
			Backend: common.ArgoCDExportStorageBackendLocal,
		}
		return r.Client.Update(context.TODO(), cr)
	}

	// Local storage
	if err := r.reconcileLocalStorage(cr); err != nil {
		return err
	}

	return nil
}
