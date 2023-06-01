/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var log = logf.Log.WithName("validation_webhook_argocd")

func (r *ArgoCD) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-argoproj-io-v1alpha1-argocd,mutating=false,failurePolicy=fail,sideEffects=None,groups=argoproj.io,resources=argocds,verbs=create;update,versions=v1alpha1,name=vargocd.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &ArgoCD{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ArgoCD) ValidateCreate() error {
	log.Info("validate ArgoCD create", "name", r.Name)

	if err := r.ValidateArgocdCR(); err != nil {
		return err
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ArgoCD) ValidateUpdate(old runtime.Object) error {
	log.Info("validate update", "name", r.Name)

	if err := r.ValidateArgocdCR(); err != nil {
		return err
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ArgoCD) ValidateDelete() error {
	log.Info("validate delete", "name", r.Name)

	return nil
}

func (cr *ArgoCD) ValidateArgocdCR() error {

	var minShards int32 = cr.Spec.Controller.Sharding.MinShards
	var maxShards int32 = cr.Spec.Controller.Sharding.MaxShards

	if cr.Spec.Controller.Sharding.DynamicScalingEnabled {
		if minShards < 1 {
			return fmt.Errorf("spec.controller.sharding.minShards cannot be less than 1")
		}

		if maxShards < minShards {
			return fmt.Errorf("spec.controller.sharding.maxShards cannot be less than spec.controller.sharding.minShards")
		}

		clustersPerShard := cr.Spec.Controller.Sharding.ClustersPerShard
		if clustersPerShard < 1 {
			return fmt.Errorf("spec.controller.sharding.clustersPerShard cannot be less than 1")
		}
	}

	return nil
}
