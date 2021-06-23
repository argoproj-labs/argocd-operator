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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewStatefulSetWithSuffix returns a new StatefulSet instance for the given ArgoCD using the given suffix.
func NewStatefulSetWithSuffix(suffix string, component string, cr *argoprojv1a1.ArgoCD) *appsv1.StatefulSet {
	return newStatefulSetWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), component, cr)
}

// newStatefulSetWithName returns a new StatefulSet instance for the given ArgoCD using the given name.
func newStatefulSetWithName(name string, component string, cr *argoprojv1a1.ArgoCD) *appsv1.StatefulSet {
	ss := newStatefulSet(cr)
	ss.ObjectMeta.Name = name

	lbls := ss.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	lbls[common.ArgoCDKeyComponent] = component
	ss.ObjectMeta.Labels = lbls

	ss.Spec = appsv1.StatefulSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				common.ArgoCDKeyName: name,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					common.ArgoCDKeyName: name,
				},
			},
		},
	}

	ss.Spec.ServiceName = name
	return ss
}

// newStatefulSet returns a new StatefulSet instance for the given ArgoCD instance.
func newStatefulSet(cr *argoprojv1a1.ArgoCD) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}
