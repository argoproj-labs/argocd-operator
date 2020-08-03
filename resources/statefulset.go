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
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewStatefulSet returns a new StatefulSet instance for the given ArgoCD instance.
func NewStatefulSet(meta metav1.ObjectMeta) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: meta.Namespace,
			Labels:    common.LabelsForCluster(meta),
		},
	}
}

// NewStatefulSetWithName returns a new StatefulSet instance for the given ArgoCD using the given name.
func NewStatefulSetWithName(meta metav1.ObjectMeta, name string, component string) *appsv1.StatefulSet {
	ss := NewStatefulSet(meta)
	ss.ObjectMeta.Name = name

	lbls := ss.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	lbls[common.ArgoCDKeyComponent] = component
	ss.ObjectMeta.Labels = lbls

	return ss
}

// NewStatefulSetWithSuffix returns a new StatefulSet instance for the given ArgoCD using the given suffix.
func NewStatefulSetWithSuffix(meta metav1.ObjectMeta, suffix string, component string) *appsv1.StatefulSet {
	return NewStatefulSetWithName(meta, fmt.Sprintf("%s-%s", meta.Name, suffix), component)
}
