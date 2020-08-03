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
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewIngress returns a new Ingress instance for the given ObjectMeta.
func NewIngress(meta metav1.ObjectMeta) *extv1beta1.Ingress {
	return &extv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: meta.Namespace,
			Labels:    common.LabelsForCluster(meta),
		},
	}
}

// NewIngressWithName returns a new Ingress with the given name and ObjectMeta.
func NewIngressWithName(meta metav1.ObjectMeta, name string) *extv1beta1.Ingress {
	ingress := NewIngress(meta)
	ingress.ObjectMeta.Name = name

	lbls := ingress.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	ingress.ObjectMeta.Labels = lbls

	return ingress
}

// NewIngressWithSuffix returns a new Ingress with the given name suffix for the ObjectMeta.
func NewIngressWithSuffix(meta metav1.ObjectMeta, suffix string) *extv1beta1.Ingress {
	return NewIngressWithName(meta, fmt.Sprintf("%s-%s", meta.Name, suffix))
}
