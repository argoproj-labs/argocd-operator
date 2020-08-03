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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewDeployment returns a new Deployment instance for the given ObjectMeta.
func NewDeployment(meta metav1.ObjectMeta) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: meta.Namespace,
			Labels:    common.LabelsForCluster(meta),
		},
	}
}

// NewDeploymentWithName returns a new Deployment instance for the given ObjectMeta using the given name.
func NewDeploymentWithName(meta metav1.ObjectMeta, name string, component string) *appsv1.Deployment {
	deploy := NewDeployment(meta)
	deploy.ObjectMeta.Name = name

	lbls := deploy.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	lbls[common.ArgoCDKeyComponent] = component
	deploy.ObjectMeta.Labels = lbls

	deploy.Spec = appsv1.DeploymentSpec{
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

	return deploy
}

// NewDeploymentWithSuffix returns a new Deployment instance for the given ObjectMeta using the given suffix.
func NewDeploymentWithSuffix(meta metav1.ObjectMeta, suffix string, component string) *appsv1.Deployment {
	return NewDeploymentWithName(meta, fmt.Sprintf("%s-%s", meta.Name, suffix), component)
}
