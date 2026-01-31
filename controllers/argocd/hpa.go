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
	"reflect"

	autoscaling "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

const (
	maxReplicas int32 = 3
	minReplicas int32 = 1
	tcup        int32 = 50
)

func newHorizontalPodAutoscaler(cr *argoproj.ArgoCD) *autoscaling.HorizontalPodAutoscaler {
	return &autoscaling.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
	}
}

func newHorizontalPodAutoscalerWithName(name string, cr *argoproj.ArgoCD) *autoscaling.HorizontalPodAutoscaler {
	hpa := newHorizontalPodAutoscaler(cr)
	hpa.Name = name

	lbls := hpa.Labels
	lbls[common.ArgoCDKeyName] = name
	hpa.Labels = lbls

	return hpa
}

func newHorizontalPodAutoscalerWithSuffix(suffix string, cr *argoproj.ArgoCD) *autoscaling.HorizontalPodAutoscaler {
	return newHorizontalPodAutoscalerWithName(nameWithSuffix(suffix, cr), cr)
}

// reconcileServerHPA will ensure that the HorizontalPodAutoscaler is present for the Argo CD Server component, and reconcile any detected changes.
func (r *ReconcileArgoCD) reconcileServerHPA(cr *argoproj.ArgoCD) error {

	defaultHPA := newHorizontalPodAutoscalerWithSuffix("server", cr)
	defaultHPA.Spec = autoscaling.HorizontalPodAutoscalerSpec{
		MaxReplicas:                    maxReplicas,
		MinReplicas:                    ptr.To(minReplicas),
		TargetCPUUtilizationPercentage: ptr.To(tcup),
		ScaleTargetRef: autoscaling.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       nameWithSuffix("server", cr),
		},
	}

	existingHPA := newHorizontalPodAutoscalerWithSuffix("server", cr)
	hpaExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, existingHPA.Name, existingHPA)
	if err != nil {
		return err
	}
	if hpaExists {
		if !cr.Spec.Server.Autoscale.Enabled {
			argoutil.LogResourceDeletion(log, existingHPA, "server autoscaling is disabled")
			return r.Delete(context.TODO(), existingHPA) // HorizontalPodAutoscaler found but globally disabled, delete it.
		}

		changed := false
		// HorizontalPodAutoscaler found, reconcile if necessary changes detected
		if cr.Spec.Server.Autoscale.HPA != nil {
			if !reflect.DeepEqual(existingHPA.Spec, cr.Spec.Server.Autoscale.HPA) {
				existingHPA.Spec = *cr.Spec.Server.Autoscale.HPA
				changed = true
			}
		}

		if changed {
			argoutil.LogResourceUpdate(log, existingHPA, "due to differences from ArgoCD CR")
			return r.Update(context.TODO(), existingHPA)
		}

		// HorizontalPodAutoscaler found, no changes detected
		return nil
	}

	if !cr.Spec.Server.Autoscale.Enabled {
		return nil // AutoScale not enabled, move along...
	}

	// AutoScale enabled, no existing HPA found, create
	if cr.Spec.Server.Autoscale.HPA != nil {
		defaultHPA.Spec = *cr.Spec.Server.Autoscale.HPA
	}

	argoutil.LogResourceCreation(log, defaultHPA)
	return r.Create(context.TODO(), defaultHPA)
}

// reconcileAutoscalers will ensure that all HorizontalPodAutoscalers are present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileAutoscalers(cr *argoproj.ArgoCD) error {
	if err := r.reconcileServerHPA(cr); err != nil {
		return err
	}
	return nil
}
