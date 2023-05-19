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
	logr "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var log = logr.Log.WithName("validation_webhook_argocd")

func (r *ArgoCD) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-managed-gitops-redhat-com-v1alpha1-gitopsdeployment,mutating=true,failurePolicy=fail,sideEffects=None,groups=managed-gitops.redhat.com,resources=gitopsdeployments,verbs=create;update,versions=v1alpha1,name=mgitopsdeployment.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &ArgoCD{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (cr *ArgoCD) Default() {
	log.Info("default", "name", cr.Name)

}

//+kubebuilder:webhook:path=/validate-managed-gitops-redhat-com-v1alpha1-gitopsdeployment,mutating=false,failurePolicy=fail,sideEffects=None,groups=managed-gitops.redhat.com,resources=gitopsdeployments,verbs=create;update,versions=v1alpha1,name=vgitopsdeployment.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ArgoCD{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (cr *ArgoCD) ValidateCreate() error {
	log.Info("validate ArgoCD create", "name", cr.Name)

	if err := cr.ValidateArgocdCR(); err != nil {
		return err
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (cr *ArgoCD) ValidateUpdate(old runtime.Object) error {
	log.Info("validate update", "name", cr.Name)

	if err := cr.ValidateArgocdCR(); err != nil {
		return err
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (cr *ArgoCD) ValidateDelete() error {
	log.Info("validate delete", "name", cr.Name)

	return nil
}

func (cr *ArgoCD) ValidateArgocdCR() error {

	var minShards int32 = cr.Spec.Controller.Sharding.MinShards
	var maxShards int32 = cr.Spec.Controller.Sharding.MaxShards

	if cr.Spec.Controller.Sharding.DynamicScalingEnabled {
		if minShards < 1 {
			return fmt.Errorf("Minimum number of shards cannot be less than 1. Setting default value to 1")
		}

		if maxShards < minShards {
			return fmt.Errorf("Maximum number of shards cannot be less than minimum number of shards. Setting maximum shards same as minimum shards")
		}

		clustersPerShard := cr.Spec.Controller.Sharding.ClustersPerShard
		if clustersPerShard < 1 {
			return fmt.Errorf("ClustersPerShard cannot be less than 1. Defaulting to 1")
		}
	}

	return nil
}
