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
	"github.com/argoproj-labs/argocd-operator/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewServiceAccount returns a new ServiceAccount instance.
func NewServiceAccount(meta metav1.ObjectMeta) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: meta.Namespace,
			Labels:    common.LabelsForCluster(meta),
		},
	}
}

// NewServiceAccountWithName creates a new ServiceAccount with the given name for the given ArgCD.
func NewServiceAccountWithName(meta metav1.ObjectMeta, name string) *corev1.ServiceAccount {
	sa := NewServiceAccount(meta)
	sa.ObjectMeta.Name = name

	lbls := sa.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	sa.ObjectMeta.Labels = lbls

	return sa
}
