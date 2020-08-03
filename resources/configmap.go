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

package resources

import (
	"fmt"

	"github.com/argoproj-labs/argocd-operator/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewConfigMap returns a new ConfigMap instance for the given ObjectMeta.
func NewConfigMap(meta metav1.ObjectMeta) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: meta.Namespace,
			Labels:    common.LabelsForCluster(meta),
		},
	}
}

// NewConfigMapWithName creates a new ConfigMap with the given name for the given ObjectMeta.
func NewConfigMapWithName(meta metav1.ObjectMeta, name string) *corev1.ConfigMap {
	cm := NewConfigMap(meta)
	cm.ObjectMeta.Name = name

	lbls := cm.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	cm.ObjectMeta.Labels = lbls

	return cm
}

// NewConfigMapWithSuffix creates a new ConfigMap with the given suffix appended to the name.
// The name for the CongifMap is based on the name of the given ObjectMeta.
func NewConfigMapWithSuffix(meta metav1.ObjectMeta, suffix string) *corev1.ConfigMap {
	return NewConfigMapWithName(meta, fmt.Sprintf("%s-%s", meta.Name, suffix))
}
