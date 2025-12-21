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

package v1beta1

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type ArgoCDValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ admission.CustomValidator = &ArgoCDValidator{}

func SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&ArgoCD{}).WithValidator(&ArgoCDValidator{}).
		Complete()
}

//+kubebuilder:webhook:path=/validate-argoproj-io-v1beta1-argocd,mutating=false,failurePolicy=fail,sideEffects=None,groups=argoproj.io,resources=argocds,verbs=create;update,versions=v1beta1,name=vargocd.kb.io,admissionReviewVersions=v1

// ValidateCreate implements admission.CustomValidator so a webhook will be registered for the type
func (v *ArgoCDValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	argocd, ok := obj.(*ArgoCD)
	if !ok {
		return nil, errors.New("expected ArgoCD object")
	}
	if argocd.Spec.SSO != nil && argocd.Spec.OIDCConfig != "" {
		return nil, errors.New("SSO and OIDCConfig cannot be used together")
	}
	return nil, nil
}

// ValidateUpdate implements admission.CustomValidator so a webhook will be registered for the type
func (v *ArgoCDValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	argocd, ok := newObj.(*ArgoCD)
	if !ok {
		return nil, errors.New("expected ArgoCD object")
	}
	if argocd.Spec.SSO != nil && argocd.Spec.OIDCConfig != "" {
		return nil, errors.New("SSO and OIDCConfig cannot be used together")
	}
	return nil, nil
}

// ValidateDelete implements admission.CustomValidator so a webhook will be registered for the type
func (v *ArgoCDValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// Add any deletion validation logic here if needed
	return nil, nil
}
