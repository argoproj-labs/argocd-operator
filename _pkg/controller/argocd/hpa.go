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

package argocd

import (
	"context"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
	autoscaling "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newHorizontalPodAutoscaler(cr *argoprojv1a1.ArgoCD) *autoscaling.HorizontalPodAutoscaler {
	return &autoscaling.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}

func newHorizontalPodAutoscalerWithName(name string, cr *argoprojv1a1.ArgoCD) *autoscaling.HorizontalPodAutoscaler {
	hpa := newHorizontalPodAutoscaler(cr)
	hpa.ObjectMeta.Name = name

	lbls := hpa.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	hpa.ObjectMeta.Labels = lbls

	return hpa
}

func newHorizontalPodAutoscalerWithSuffix(suffix string, cr *argoprojv1a1.ArgoCD) *autoscaling.HorizontalPodAutoscaler {
	return newHorizontalPodAutoscalerWithName(nameWithSuffix(suffix, cr), cr)
}

// reconcileServerHPA will ensure that the HorizontalPodAutoscaler is present for the Argo CD Server component.
func (r *ReconcileArgoCD) reconcileServerHPA(cr *argoprojv1a1.ArgoCD) error {
	hpa := newHorizontalPodAutoscalerWithSuffix("server", cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, hpa.Name, hpa) {
		if !cr.Spec.Server.Autoscale.Enabled {
			return r.client.Delete(context.TODO(), hpa) // HorizontalPodAutoscaler found but globally disabled, delete it.
		}
		return nil // HorizontalPodAutoscaler found and configured, nothing do to, move along...
	}

	if !cr.Spec.Server.Autoscale.Enabled {
		return nil // AutoScale not enabled, move along...
	}

	if cr.Spec.Server.Autoscale.HPA != nil {
		hpa.Spec = *cr.Spec.Server.Autoscale.HPA
	} else {
		hpa.Spec.MaxReplicas = 3

		var minrReplicas int32 = 1
		hpa.Spec.MinReplicas = &minrReplicas

		var tcup int32 = 50
		hpa.Spec.TargetCPUUtilizationPercentage = &tcup

		hpa.Spec.ScaleTargetRef = autoscaling.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       nameWithSuffix("server", cr),
		}
	}

	return r.client.Create(context.TODO(), hpa)
}

// reconcileAutoscalers will ensure that all HorizontalPodAutoscalers are present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileAutoscalers(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileServerHPA(cr); err != nil {
		return err
	}
	return nil
}
