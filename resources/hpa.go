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
	autoscaling "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewHorizontalPodAutoscaler returns a new HorizontalPodAutoscaler instance for the given ObjectMeta.
func NewHorizontalPodAutoscaler(meta metav1.ObjectMeta) *autoscaling.HorizontalPodAutoscaler {
	return &autoscaling.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: meta.Namespace,
			Labels:    common.LabelsForCluster(meta),
		},
	}
}

// NewHorizontalPodAutoscalerWithName creates a new HorizontalPodAutoscaler with the given name for the given ObjectMeta.
func NewHorizontalPodAutoscalerWithName(meta metav1.ObjectMeta, name string) *autoscaling.HorizontalPodAutoscaler {
	hpa := NewHorizontalPodAutoscaler(meta)
	hpa.ObjectMeta.Name = name

	lbls := hpa.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	hpa.ObjectMeta.Labels = lbls

	return hpa
}

// NewHorizontalPodAutoscalerWithSuffix creates a new HorizontalPodAutoscaler with the given suffix appended to the name.
// The name for the HorizontalPodAutoscaler is based on the name of the given ObjectMeta.
func NewHorizontalPodAutoscalerWithSuffix(meta metav1.ObjectMeta, suffix string) *autoscaling.HorizontalPodAutoscaler {
	return NewHorizontalPodAutoscalerWithName(meta, common.NameWithSuffix(meta, suffix))
}
