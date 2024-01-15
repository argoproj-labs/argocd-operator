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

package argoutil

import (
	"fmt"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
)

// FqdnServiceRef will return the FQDN referencing a specific service name, as set up by the operator, with the
// given port.
func FqdnServiceRef(serviceName, namespace string, port int) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local:%d", serviceName, namespace, port)
}

// NameWithSuffix will return a string using the Name from the given ObjectMeta with the provded suffix appended.
func NameWithSuffix(name, suffix string) string {
	return fmt.Sprintf("%s-%s", name, suffix)
}

// GenerateResourceName generates names for namespace scoped resources
func GenerateResourceName(instanceName, component string) string {
	return NameWithSuffix(instanceName, component)
}

// GenerateUniqueResourceName generates unique names for cluster scoped resources
func GenerateUniqueResourceName(instanceName, instanceNamespace, component string) string {
	return fmt.Sprintf("%s-%s-%s", instanceName, instanceNamespace, component)
}

// FetchStorageSecretName will return the name of the Secret to use for the export process.
func FetchStorageSecretName(export *argoprojv1alpha1.ArgoCDExport) string {
	name := NameWithSuffix(export.ObjectMeta.Name, "export")
	if export.Spec.Storage != nil && len(export.Spec.Storage.SecretName) > 0 {
		name = export.Spec.Storage.SecretName
	}
	return name
}
