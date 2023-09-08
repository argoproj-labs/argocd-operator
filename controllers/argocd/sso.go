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

	template "github.com/openshift/api/template/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

const (
	ssoLegalUnknown         string = "Unknown"
	ssoLegalSuccess         string = "Success"
	ssoLegalFailed          string = "Failed"
	illegalSSOConfiguration string = "illegal SSO configuration: "
)

var (
	templateAPIFound     = false
	ssoConfigLegalStatus string
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

// The purpose of reconcileSSO is to try and catch as many illegal configuration edge cases at the highest level (that can lead to conflicts)
// as possible, that may arise from the operator supporting multiple SSO providers.
// The operator must support `.spec.sso.dex` fields for dex, and `.spec.sso.keycloak` fields for keycloak.
// The operator must identify edge cases involving partial configurations of specs, spec mismatch with
// active provider, contradicting configuration etc, and throw the appropriate errors.
func (r *ReconcileArgoCD) reconcileSSO(cr *argoproj.ArgoCD) error {

	// reset ssoConfigLegalStatus at the beginning of each SSO reconciliation round
	ssoConfigLegalStatus = ssoLegalUnknown

	// case 1
	if cr.Spec.SSO == nil {
		// no SSO configured, nothing to do here
		return nil
	}

	if cr.Spec.SSO != nil {

		errMsg := ""
		var err error
		isError := false

		// case 2
		if cr.Spec.SSO.Provider.ToLower() == argoproj.SSOProviderTypeDex {
			// Relevant SSO settings at play are `.spec.sso.dex` fields, `.spec.sso.keycloak`

			if cr.Spec.SSO.Dex == nil || (cr.Spec.SSO.Dex != nil && !cr.Spec.SSO.Dex.OpenShiftOAuth && cr.Spec.SSO.Dex.Config == "") {
				// sso provider specified as dex but no dexconfig supplied. This will cause health probe to fail as per
				// https://github.com/argoproj-labs/argocd-operator/pull/615 ==> conflict
				errMsg = "must supply valid dex configuration when requested SSO provider is dex"
				isError = true
			} else if cr.Spec.SSO.Keycloak != nil {
				// new keycloak spec fields are expressed when `.spec.sso.provider` is set to dex ==> conflict
				errMsg = "cannot supply keycloak configuration in .spec.sso.keycloak when requested SSO provider is dex"
				isError = true
			}

			if isError {
				err = errors.New(illegalSSOConfiguration + errMsg)
				log.Error(err, fmt.Sprintf("Illegal expression of SSO configuration detected for Argo CD %s in namespace %s. %s", cr.Name, cr.Namespace, errMsg))
				ssoConfigLegalStatus = ssoLegalFailed // set global indicator that SSO config has gone wrong
				_ = r.reconcileStatusSSO(cr)
				return err
			}
		}

		// case 3
		if cr.Spec.SSO.Provider.ToLower() == argoproj.SSOProviderTypeKeycloak {
			// Relevant SSO settings at play are `.spec.sso.keycloak` fields, `.spec.sso.dex`

			if cr.Spec.SSO.Dex != nil {
				// new dex spec fields are expressed when `.spec.sso.provider` is set to keycloak ==> conflict
				errMsg = "cannot supply dex configuration when requested SSO provider is keycloak"
				err = errors.New(illegalSSOConfiguration + errMsg)
				isError = true
			}

			if isError {
				log.Error(err, fmt.Sprintf("Illegal expression of SSO configuration detected for Argo CD %s in namespace %s. %s", cr.Name, cr.Namespace, errMsg))
				ssoConfigLegalStatus = ssoLegalFailed // set global indicator that SSO config has gone wrong
				_ = r.reconcileStatusSSO(cr)
				return err
			}
		}

		// case 4
		if cr.Spec.SSO.Provider.ToLower() == "" {

			if cr.Spec.SSO.Dex != nil ||
				// `.spec.sso.dex` expressed without specifying SSO provider ==> conflict
				cr.Spec.SSO.Keycloak != nil {
				// `.spec.sso.keycloak` expressed without specifying SSO provider ==> conflict

				errMsg = "Cannot specify SSO provider spec without specifying SSO provider type"
				err = errors.New(illegalSSOConfiguration + errMsg)
				log.Error(err, fmt.Sprintf("Cannot specify SSO provider spec without specifying SSO provider type for Argo CD %s in namespace %s.", cr.Name, cr.Namespace))
				ssoConfigLegalStatus = ssoLegalFailed // set global indicator that SSO config has gone wrong
				_ = r.reconcileStatusSSO(cr)
				return err
			}
		}

		// case 5
		if cr.Spec.SSO.Provider.ToLower() != argoproj.SSOProviderTypeDex && cr.Spec.SSO.Provider.ToLower() != argoproj.SSOProviderTypeKeycloak {
			// `.spec.sso.provider` contains unsupported value

			errMsg = fmt.Sprintf("Unsupported SSO provider type. Supported providers are %s and %s", argoproj.SSOProviderTypeDex, argoproj.SSOProviderTypeKeycloak)
			err = errors.New(illegalSSOConfiguration + errMsg)
			log.Error(err, fmt.Sprintf("Unsupported SSO provider type for Argo CD %s in namespace %s.", cr.Name, cr.Namespace))
			ssoConfigLegalStatus = ssoLegalFailed // set global indicator that SSO config has gone wrong
			_ = r.reconcileStatusSSO(cr)
			return err
		}
	}

	// control reaching this point means that none of the illegal config combinations were detected. SSO is configured legally
	// set global indicator that SSO config has been successful
	ssoConfigLegalStatus = ssoLegalSuccess

	// reconcile resources based on enabled provider
	// keycloak
	if cr.Spec.SSO != nil && cr.Spec.SSO.Provider.ToLower() == argoproj.SSOProviderTypeKeycloak {

		// Trigger reconciliation of any Dex resources so they get deleted
		if err := r.reconcileDexResources(cr); err != nil && !apiErrors.IsNotFound(err) {
			log.Error(err, "Unable to delete existing dex resources before configuring keycloak")
			return err
		}

		if err := r.reconcileKeycloakConfiguration(cr); err != nil {
			return err
		}
	} else if UseDex(cr) {
		// dex
		// Delete any lingering keycloak artifacts before Dex is configured as this is not handled by the reconcilliation loop
		if err := deleteKeycloakConfiguration(cr); err != nil && !apiErrors.IsNotFound(err) {
			log.Error(err, "Unable to delete existing SSO configuration before configuring Dex")
			return err
		}

		if err := r.reconcileDexResources(cr); err != nil {
			return err
		}
	}

	_ = r.reconcileStatusSSO(cr)

	return nil
}

func (r *ReconcileArgoCD) deleteSSOConfiguration(newCr *argoproj.ArgoCD, oldCr *argoproj.ArgoCD) error {

	log.Info("uninstalling existing SSO configuration")

	if oldCr.Spec.SSO.Provider.ToLower() == argoproj.SSOProviderTypeKeycloak {
		if err := deleteKeycloakConfiguration(newCr); err != nil {
			log.Error(err, "Unable to delete existing keycloak configuration")
			return err
		}
	} else if oldCr.Spec.SSO.Provider.ToLower() == argoproj.SSOProviderTypeDex {
		// Trigger reconciliation of Dex resources so they get deleted
		if err := r.deleteDexResources(newCr); err != nil {
			log.Error(err, "Unable to reconcile necessary resources for uninstallation of Dex")
			return err
		}
	}

	_ = r.reconcileStatusSSO(newCr)
	return nil
}
