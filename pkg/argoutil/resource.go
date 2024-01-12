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
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
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
func GenerateResourceName(instanceName, suffix string) string {
	return NameWithSuffix(instanceName, suffix)
}

// GenerateUniqueResourceName generates unique names for cluster scoped resources
func GenerateUniqueResourceName(instanceName, instanceNamespace, suffix string) string {
	return fmt.Sprintf("%s-%s-%s", instanceName, instanceNamespace, suffix)
}

func GetObjMeta(resName, resNs, instanceName, instanceNs, component string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        resName,
		Namespace:   resNs,
		Labels:      common.DefaultResourceLabels(resName, instanceName, component),
		Annotations: common.DefaultResourceAnnotations(instanceName, instanceNs),
	}
}

// FetchObject will retrieve the object with the given namespace and name using the Kubernetes API.
// The result will be stored in the given object.
func FetchObject(client client.Client, namespace string, name string, obj client.Object) error {
	return client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, obj)
}

// FetchStorageSecretName will return the name of the Secret to use for the export process.
func FetchStorageSecretName(export *argoprojv1alpha1.ArgoCDExport) string {
	name := NameWithSuffix(export.ObjectMeta.Name, "export")
	if export.Spec.Storage != nil && len(export.Spec.Storage.SecretName) > 0 {
		name = export.Spec.Storage.SecretName
	}
	return name
}

// IsObjectFound will perform a basic check that the given object exists via the Kubernetes API.
// If an error occurs as part of the check, the function will return false.
func IsObjectFound(client client.Client, namespace string, name string, obj client.Object) bool {
	return !apierrors.IsNotFound(FetchObject(client, namespace, name, obj))
}

func FilterObjectsBySelector(c client.Client, objectList client.ObjectList, selector labels.Selector) error {
	return c.List(context.TODO(), objectList, client.MatchingLabelsSelector{Selector: selector})
}
