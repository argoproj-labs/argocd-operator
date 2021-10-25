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
	template "github.com/openshift/api/template/v1"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

var (
	templateAPIFound = false
)

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
		// TemplateAPI is available, Install keycloak using openshift templates.
		if IsTemplateAPIAvailable() {
			err := r.reconcileKeycloakForOpenShift(cr)
			if err != nil {
				return err
			}
		} else {
			err := r.reconcileKeycloak(cr)
			if err != nil {
				return err
			}
		}
	} else if cr.Spec.SSO.Provider == argoprojv1a1.SSOProviderTypeDex {
		err := r.reconcileDexDeployment(cr)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteSSOConfiguration(cr *argoprojv1a1.ArgoCD) error {

	// If SSO is installed using OpenShift templates.
	if IsTemplateAPIAvailable() {
		err := deleteKeycloakConfigForOpenShift(cr)
		if err != nil {
			return err
		}
	} else {
		err := deleteKeycloakConfigForK8s(cr)
		if err != nil {
			return err
		}
	}

	return nil
}
