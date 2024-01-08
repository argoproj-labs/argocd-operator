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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj-labs/argocd-operator/common"
)

// TO DO: REFACTOR

// NewPVCResourceRequirements will provide a resource list of specified capacity.
func NewPVCResourceRequirements(capacity resource.Quantity) corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			"storage": capacity,
		},
	}
}

// NewPersistentVolumeClaim returns a new PersistentVolumeClaim instance for the ObjectMeta resource.
func NewPersistentVolumeClaim(meta metav1.ObjectMeta) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: meta.Namespace,
			Labels:    common.DefaultLabels(meta.Name),
		},
	}
}

// NewPersistentVolumeClaimWithName returns a new PersistentVolumeClaim instance with the given name.
func NewPersistentVolumeClaimWithName(name string, meta metav1.ObjectMeta) *corev1.PersistentVolumeClaim {
	pvc := NewPersistentVolumeClaim(meta)
	pvc.ObjectMeta.Name = name
	pvc.ObjectMeta.Labels[common.AppK8sKeyName] = name
	return pvc
}
