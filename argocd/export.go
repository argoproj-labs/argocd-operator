// Copyright 2019 Argo CD Operator Developers
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
	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/resources"
)

func (r *ArgoClusterReconciler) getArgoCDExport(cr *v1alpha1.ArgoCD) *v1alpha1.ArgoCDExport {
	if cr.Spec.Import == nil {
		return nil
	}

	namespace := cr.ObjectMeta.Namespace
	if cr.Spec.Import.Namespace != nil && len(*cr.Spec.Import.Namespace) > 0 {
		namespace = *cr.Spec.Import.Namespace
	}

	export := &v1alpha1.ArgoCDExport{}
	if resources.IsObjectFound(r.Client, namespace, cr.Spec.Import.Name, export) {
		return export
	}
	return nil
}

func getArgoExportSecretName(export *v1alpha1.ArgoCDExport) string {
	name := common.NameWithSuffix(export.ObjectMeta, "export")
	if export.Spec.Storage != nil && len(export.Spec.Storage.SecretName) > 0 {
		name = export.Spec.Storage.SecretName
	}
	return name
}

// getStorageSecretName will return the name of the Secret to use for the export process.
func getStorageSecretName(export *v1alpha1.ArgoCDExport) string {
	name := common.NameWithSuffix(export.ObjectMeta, "export")
	if export.Spec.Storage != nil && len(export.Spec.Storage.SecretName) > 0 {
		name = export.Spec.Storage.SecretName
	}
	return name
}
