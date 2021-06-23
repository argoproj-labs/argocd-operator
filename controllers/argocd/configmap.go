// Copyright 2021 ArgoCD Operator Developers
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
	"fmt"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewConfigMapWithName creates a new ConfigMap with the given name for the given ArgCD.
func NewConfigMapWithName(name string, cr *argoprojv1a1.ArgoCD) *corev1.ConfigMap {
	cm := newConfigMap(cr)
	cm.ObjectMeta.Name = name

	lbls := cm.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	cm.ObjectMeta.Labels = lbls

	return cm
}

// newConfigMap returns a new ConfigMap instance for the given ArgoCD.
func newConfigMap(cr *argoprojv1a1.ArgoCD) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}

// GetCAConfigMapName will return the CA ConfigMap name for the given ArgoCD.
func GetCAConfigMapName(cr *argoprojv1a1.ArgoCD) string {
	if len(cr.Spec.TLS.CA.ConfigMapName) > 0 {
		return cr.Spec.TLS.CA.ConfigMapName
	}
	return nameWithSuffix(common.ArgoCDCASuffix, cr)
}

// nameWithSuffix will return a name based on the given ArgoCD. The given suffix is appended to the generated name.
// Example: Given an ArgoCD with the name "example-argocd", providing the suffix "foo" would result in the value of
// "example-argocd-foo" being returned.
func nameWithSuffix(suffix string, cr *argoprojv1a1.ArgoCD) string {
	return fmt.Sprintf("%s-%s", cr.Name, suffix)
}

// NewConfigMapWithSuffix creates a new ConfigMap with the given suffix appended to the name.
// The name for the CongifMap is based on the name of the given ArgCD.
func NewConfigMapWithSuffix(suffix string, cr *argoprojv1a1.ArgoCD) *corev1.ConfigMap {
	return NewConfigMapWithName(fmt.Sprintf("%s-%s", cr.ObjectMeta.Name, suffix), cr)
}
