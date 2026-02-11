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

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

const (
	ssoLegalUnknown         string = "Unknown"
	ssoLegalSuccess         string = "Success"
	ssoLegalFailed          string = "Failed"
	illegalSSOConfiguration string = "illegal SSO configuration: "
)

// The purpose of reconcileSSO is to try and catch as many illegal configuration edge cases at the highest level (that can lead to conflicts)
// as possible, that may arise from the operator supporting multiple SSO providers.
// The operator must support `.spec.sso.dex` fields for dex.
// The operator must identify edge cases involving partial configurations of specs, spec mismatch with
// active provider, contradicting configuration etc, and throw the appropriate errors.
func (r *ReconcileArgoCD) reconcileSSO(cr *argoproj.ArgoCD, argocdStatus *argoproj.ArgoCDStatus, oAuthEnabled bool) error {

	// case 1
	if cr.Spec.SSO == nil {
		// no SSO configured, nothing to do here
		return nil
	}

	// After case 1, cr.Spec.SSO is necessarily non-nil

	// case 2
	if cr.Spec.SSO.Provider.ToLower() == argoproj.SSOProviderTypeDex {
		// Relevant SSO settings at play are `.spec.sso.dex` fields

		errMsg := ""
		isError := false

		if cr.Spec.SSO.Dex == nil || (cr.Spec.SSO.Dex != nil && !cr.Spec.SSO.Dex.OpenShiftOAuth && cr.Spec.SSO.Dex.Config == "") {
			// sso provider specified as dex but no dexconfig supplied. This will cause health probe to fail as per
			// https://github.com/argoproj-labs/argocd-operator/pull/615 ==> conflict
			errMsg = "must supply valid dex configuration when requested SSO provider is dex"
			isError = true
		} else if cr.Spec.SSO.Keycloak != nil {
			errMsg = "keycloak configuration is specified even though Dex is enabled. Keycloak support has been deprecated and is no longer available."
			isError = true
		}

		if isError {
			err := errors.New(illegalSSOConfiguration + errMsg)
			log.Error(err, fmt.Sprintf("Illegal expression of SSO configuration detected for Argo CD %s in namespace %s. %s", cr.Name, cr.Namespace, errMsg))
			argocdStatus.SSO = "Failed"
			return err
		}
	}

	// case 3
	if cr.Spec.SSO.Provider.ToLower() == argoproj.SSOProviderTypeKeycloak {
		log.Info("Keycloak SSO provider is no longer supported. RBAC scopes configuration is ignored.")
		argocdStatus.SSO = "Failed"
		return fmt.Errorf("keycloak is set as SSO provider, but keycloak support has been deprecated and is no longer available")
	}

	// case 4
	if cr.Spec.SSO.Provider.ToLower() == "" {

		if cr.Spec.SSO.Dex != nil {
			// `.spec.sso.dex` expressed without specifying SSO provider ==> conflict
			errMsg := "Cannot specify SSO provider spec without specifying SSO provider type"
			err := errors.New(illegalSSOConfiguration + errMsg)
			log.Error(err, fmt.Sprintf("Cannot specify SSO provider spec without specifying SSO provider type for Argo CD %s in namespace %s.", cr.Name, cr.Namespace))
			argocdStatus.SSO = "Failed"
			return err
		}
	}

	// case 5
	if cr.Spec.SSO.Provider.ToLower() != argoproj.SSOProviderTypeDex {
		// `.spec.sso.provider` contains unsupported value

		errMsg := fmt.Sprintf("Unsupported SSO provider type. Supported provider is %s", argoproj.SSOProviderTypeDex)
		err := errors.New(illegalSSOConfiguration + errMsg)
		log.Error(err, fmt.Sprintf("Unsupported SSO provider type for Argo CD %s in namespace %s.", cr.Name, cr.Namespace))
		argocdStatus.SSO = "Failed"
		return err
	}

	// control reaching this point means that none of the illegal config combinations were detected. SSO is configured legally
	// set global indicator that SSO config has been successful

	// reconcile resources based on enabled provider
	// keycloak
	if cr.Spec.SSO != nil && cr.Spec.SSO.Provider.ToLower() == argoproj.SSOProviderTypeKeycloak {
		log.Info("Keycloak SSO provider is no longer supported. RBAC scopes configuration is ignored.")
		argocdStatus.SSO = "Unknown"
		return nil
		// Keycloak functionality has been removed, skipping reconciliation
	} else if UseDex(cr) {
		// dex
		if err := r.reconcileDexResources(cr, oAuthEnabled); err != nil {
			argocdStatus.SSO = "Failed"
			return err
		}
	}

	return nil
}
