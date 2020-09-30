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
	"strings"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AppendStringMap will append the map `add` to the given map `src` and return the result.
func AppendStringMap(src map[string]string, add map[string]string) map[string]string {
	res := src
	if len(src) <= 0 {
		res = make(map[string]string, len(add))
	}
	for key, val := range add {
		res[key] = val
	}
	return res
}

// CombineImageTag will return the combined image and tag in the proper format for tags and digests.
func CombineImageTag(img string, tag string) string {
	if strings.Contains(tag, ":") {
		return fmt.Sprintf("%s@%s", img, tag) // Digest
	} else if len(tag) > 0 {
		return fmt.Sprintf("%s:%s", img, tag) // Tag
	}
	return img // No tag, use default
}

// CreateEvent will create a new Kubernetes Event with the given action, message, reason and involved uid.
func CreateEvent(client client.Client, action string, message string, reason string, meta metav1.ObjectMeta) error {
	event := newEvent(meta)
	event.Action = action
	event.InvolvedObject = corev1.ObjectReference{
		Name:      meta.Name,
		Namespace: meta.Namespace,
		UID:       meta.UID,
	}
	event.Message = message
	event.Reason = reason
	return client.Create(context.TODO(), event)
}

// DefaultLabels returns the default set of labels for controllers.
func DefaultLabels(name string) map[string]string {
	return map[string]string{
		common.ArgoCDKeyName:   name,
		common.ArgoCDKeyPartOf: common.ArgoCDAppName,
	}
}

// FetchObject will retrieve the object with the given namespace and name using the Kubernetes API.
// The result will be stored in the given object.
func FetchObject(client client.Client, namespace string, name string, obj runtime.Object) error {
	return client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, obj)
}

// FetchStorageSecretName will return the name of the Secret to use for the export process.
func FetchStorageSecretName(export *argoprojv1a1.ArgoCDExport) string {
	name := NameWithSuffix(export.ObjectMeta, "export")
	if export.Spec.Storage != nil && len(export.Spec.Storage.SecretName) > 0 {
		name = export.Spec.Storage.SecretName
	}
	return name
}

// IsObjectFound will perform a basic check that the given object exists via the Kubernetes API.
// If an error occurs as part of the check, the function will return false.
func IsObjectFound(client client.Client, namespace string, name string, obj runtime.Object) bool {
	return !apierrors.IsNotFound(FetchObject(client, namespace, name, obj))
}

// NameWithSuffix will return a string using the Name from the given ObjectMeta with the provded suffix appended.
// Example: If ObjectMeta.Name is "test" and suffix is "object", the value of "test-object" will be returned.
func NameWithSuffix(meta metav1.ObjectMeta, suffix string) string {
	return fmt.Sprintf("%s-%s", meta.Name, suffix)
}

func newEvent(meta metav1.ObjectMeta) *corev1.Event {
	event := &corev1.Event{}
	event.ObjectMeta.GenerateName = fmt.Sprintf("%s-", meta.Name)
	event.ObjectMeta.Labels = meta.Labels
	event.ObjectMeta.Namespace = meta.Namespace
	return event
}
