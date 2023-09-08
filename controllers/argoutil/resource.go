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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
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
func CreateEvent(client client.Client, eventType, action, message, reason string, objectMeta metav1.ObjectMeta, typeMeta metav1.TypeMeta) error {
	event := newEvent(objectMeta)
	event.Action = action
	event.Type = eventType
	event.InvolvedObject = corev1.ObjectReference{
		Name:            objectMeta.Name,
		Namespace:       objectMeta.Namespace,
		UID:             objectMeta.UID,
		ResourceVersion: objectMeta.ResourceVersion,
		Kind:            typeMeta.Kind,
		APIVersion:      typeMeta.APIVersion,
	}
	event.Message = message
	event.Reason = reason
	event.CreationTimestamp = metav1.Now()
	event.FirstTimestamp = event.CreationTimestamp
	event.LastTimestamp = event.CreationTimestamp
	return client.Create(context.TODO(), event)
}

// FetchObject will retrieve the object with the given namespace and name using the Kubernetes API.
// The result will be stored in the given object.
func FetchObject(client client.Client, namespace string, name string, obj client.Object) error {
	return client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, obj)
}

// FetchStorageSecretName will return the name of the Secret to use for the export process.
func FetchStorageSecretName(export *argoprojv1alpha1.ArgoCDExport) string {
	name := NameWithSuffix(export.ObjectMeta, "export")
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

// LabelsForCluster returns the labels for all cluster resources.
func LabelsForCluster(cr *argoproj.ArgoCD) map[string]string {
	labels := common.DefaultLabels(cr.Name)
	return labels
}

// annotationsForCluster returns the annotations for all cluster resources.
func AnnotationsForCluster(cr *argoproj.ArgoCD) map[string]string {
	annotations := common.DefaultAnnotations(cr.Name, cr.Namespace)
	for key, val := range cr.ObjectMeta.Annotations {
		annotations[key] = val
	}
	return annotations
}
