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
	"context"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
	template "github.com/openshift/api/template/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

var templateAPIFound = false

// IsTemplateAPIAvailable returns true if the template API is present.
func IsTemplateAPIAvailable() bool {
	return templateAPIFound
}

// verifyTemplateAPI will verify that the template API is present.
func verifyTemplateAPI() error {
	found, err := argoutil.VerifyAPI(template.SchemeGroupVersion.Group, template.SchemeGroupVersion.Version)
	if err != nil {
		return err
	}
	templateAPIFound = found
	return nil
}

func (r *ReconcileArgoCD) reconcileSSO(cr *argoprojv1a1.ArgoCD) error {
	if cr.Spec.SSO.Provider == argoprojv1a1.SSOProviderTypeKeycloak {
		// TemplateAPI is available, Install keycloack using openshift templates.
		if IsTemplateAPIAvailable() {
			templateInstanceRef, err := newKeycloakTemplateInstance(cr.Namespace)
			if err != nil {
				return err
			}
			existingTemplateInstance := template.TemplateInstance{}
			err = r.client.Get(context.TODO(), types.NamespacedName{Name: templateInstanceRef.Name, Namespace: templateInstanceRef.Namespace}, &existingTemplateInstance)
			if err != nil && errors.IsNotFound(err) {
				log.Info("Creating a new keycloak template instance")
				err = r.client.Create(context.TODO(), templateInstanceRef)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
