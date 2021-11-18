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
	"errors"
	"fmt"
	"reflect"

	apiErrors "k8s.io/apimachinery/pkg/api/errors"

	template "github.com/openshift/api/template/v1"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
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
		// Ensure SSO provider type and provided configuration are compatible
		if !reflect.DeepEqual(cr.Spec.SSO.Dex, argoprojv1a1.ArgoCDDexSpec{}) {
			err := errors.New("incorrect SSO configuration")
			log.Error(err, fmt.Sprintf("provided SSO configuration is incompatible with provider type specified: %s", cr.Spec.SSO.Provider))
			return err
		}

		// Trigger reconciliation of Dex resources so they get deleted
		if err := r.reconcileDexResources(cr); err != nil {
			log.Error(err, "Unable to reconcile necessary resources for uninstallation of Dex")
			return err
		}

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

		// Ensure SSO provider type and provided configuration are compatible
		if !reflect.DeepEqual(cr.Spec.SSO.Keycloak, argoprojv1a1.ArgoCDKeycloakSpec{}) {
			err := errors.New("incorrect SSO configuration")
			log.Error(err, fmt.Sprintf("provided SSO configuration is incompatible with provider type specified: %s", cr.Spec.SSO.Provider))
			return err
		}

		// Ensure Dex spec is supplied
		if reflect.DeepEqual(cr.Spec.SSO.Dex, argoprojv1a1.ArgoCDDexSpec{}) {
			err := errors.New("incorrect SSO configuration")
			log.Error(err, fmt.Sprintf("Must supply configuration specifications with provider type specified: %s", cr.Spec.SSO.Provider))
			return err
		}

		// Delete any lingering keycloak artifacts before Dex is configured as this is not handled by the reconcilliation loop
		if err := deleteKeycloakConfiguration(cr); err != nil {
			if !apiErrors.IsNotFound(err) {
				log.Error(err, "Unable to delete existing SSO configuration before configuring Dex")
				return err
			}
		}

		// Trigger reconciliation of Dex resources so they get created
		if err := r.reconcileDexResources(cr); err != nil {
			log.Error(err, "Unable to reconcile necessary resources for installation of Dex")
			return err
		}

	}
	return nil
}

// reconcileResources will trigger reconciliation of necessary resources after changes to Dex SSO configuration
func (r *ReconcileArgoCD) reconcileDexResources(cr *argoprojv1a1.ArgoCD) error {

	if _, err := r.reconcileRole(dexServer, policyRuleForDexServer(), cr); err != nil {
		return err
	}

	if err := r.reconcileRoleBinding(dexServer, policyRuleForDexServer(), cr); err != nil {
		return fmt.Errorf("error reconciling roleBinding for %q: %w", dexServer, err)
	}

	if err := r.reconcileServiceAccountPermissions(common.ArgoCDDexServerComponent, policyRuleForDexServer(), cr); err != nil {
		return err
	}

	// specialized handling for dex
	if err := r.reconcileDexServiceAccount(cr); err != nil {
		return err
	}

	// Reconcile dex config in argocd-cm
	if err := r.reconcileArgoConfigMap(cr); err != nil {
		return err
	}

	if err := r.reconcileDexService(cr); err != nil {
		return err
	}

	if err := r.reconcileDexDeployment(cr); err != nil {
		return err
	}

	if err := r.reconcileStatusDex(cr); err != nil {
		return err
	}

	return nil
}
