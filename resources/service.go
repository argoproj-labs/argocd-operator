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

// NewService returns a new Service for the given ArgoCD instance.
func NewService(meta metav1.ObjectMeta) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: meta.Namespace,
			Labels:    common.LabelsForCluster(meta),
		},
	}
}

// NewServiceWithName returns a new Service instance for the given ArgoCD using the given name.
func NewServiceWithName(meta metav1.ObjectMeta, name string, component string) *corev1.Service {
	svc := NewService(meta)
	svc.ObjectMeta.Name = name

	lbls := svc.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	lbls[common.ArgoCDKeyComponent] = component
	svc.ObjectMeta.Labels = lbls

	return svc
}

// NewServiceWithSuffix returns a new Service instance for the given ArgoCD using the given suffix.
func NewServiceWithSuffix(meta metav1.ObjectMeta, suffix string, component string) *corev1.Service {
	return NewServiceWithName(meta, fmt.Sprintf("%s-%s", meta.Name, suffix), component)
}
