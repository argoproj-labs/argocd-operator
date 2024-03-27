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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

// FQDNwithPort will return the FQDN referencing a specific service name, as set up by the operator, with the given port.
func FQDNwithPort(name, namespace string, port int) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local:%d", name, namespace, port)
}

// FQDN will return the FQDN referencing a specific service name, as set up by the operator
func FQDN(name, namespace string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", name, namespace)
}

// NameWithSuffix will return a string using the Name from the given ObjectMeta with the provded suffixes appended.
func NameWithSuffix(name string, suffixes ...string) string {
	return fmt.Sprintf("%s-%s", name, strings.Join(suffixes, "-"))
}

// GenerateResourceName generates names for namespace scoped resources
func GenerateResourceName(instanceName string, suffixes ...string) string {
	return NameWithSuffix(instanceName, suffixes...)
}

// GenerateUniqueResourceName generates unique names for cluster scoped resources
func GenerateUniqueResourceName(instanceName, instanceNamespace string, suffixes ...string) string {
	return NameWithSuffix(NameWithSuffix(instanceName, instanceNamespace), suffixes...)
}

func GetObjMeta(resName, resNs, instanceName, instanceNs, component string, labels map[string]string, antns map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        resName,
		Namespace:   resNs,
		Labels:      util.MergeMaps(common.DefaultResourceLabels(resName, instanceNs, component), labels),
		Annotations: util.MergeMaps(common.DefaultResourceAnnotations(instanceName, instanceNs), antns),
	}
}

// FetchStorageSecretName will return the name of the Secret to use for the export process.
func FetchStorageSecretName(export *argoprojv1alpha1.ArgoCDExport) string {
	name := NameWithSuffix(export.ObjectMeta.Name, "export")
	if export.Spec.Storage != nil && len(export.Spec.Storage.SecretName) > 0 {
		name = export.Spec.Storage.SecretName
	}
	return name
}
