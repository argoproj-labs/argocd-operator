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
	"os"
	"reflect"

	template "github.com/openshift/api/template/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

const (
	ssoLegalUnknown          string = "Unknown"
	ssoLegalSuccess          string = "Success"
	ssoLegalFailed           string = "Failed"
	illegalSSOConfiguration  string = "illegal SSO configuration: "
	multipleSSOConfiguration string = "multiple SSO configuration: "
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
// as possible, that may arise from the operator supporting multiple SSO providers in a backwards-compatible way.
// The operator must support both `.spec.dex` and `.spec.sso.dex` for dex, and `.spec.sso` fields and `.spec.sso.keycloak`
// fields for keycloak. The operator must identify edge cases involving partial configurations of specs, spec mismatch with
// active provider, contradicting configuration etc, and throw the appropriate errors.
func (r *ReconcileArgoCD) reconcileSSO(cr *argoprojv1a1.ArgoCD) error {

	// reset ssoConfigLegalStatus at the beginning of each SSO reconciliation round
	ssoConfigLegalStatus = ssoLegalUnknown

	// Emit events warning users about deprecation notice for soon-to-be-removed fields in the CR if being used

	if env := os.Getenv("DISABLE_DEX"); env != "" {
		// Emit event for each instance providing users with deprecation notice for `DISABLE_DEX` if not emitted already
		if currentInstanceEventEmissionStatus, ok := DeprecationEventEmissionTracker[cr.Namespace]; !ok || !currentInstanceEventEmissionStatus.DisableDexDeprecationWarningEmitted {
			err := argoutil.CreateEvent(r.Client, "Warning", "Deprecated", "`DISABLE_DEX` is deprecated, and support will be removed in Argo CD Operator v0.8.0/OpenShift GitOps v1.10.0. Dex can be enabled/disabled through `.spec.sso`", "DeprecationNotice", cr.ObjectMeta, cr.TypeMeta)
			if err != nil {
				return err
			}

			if !ok {
				currentInstanceEventEmissionStatus = DeprecationEventEmissionStatus{DisableDexDeprecationWarningEmitted: true}
			} else {
				currentInstanceEventEmissionStatus.DisableDexDeprecationWarningEmitted = true
			}
			DeprecationEventEmissionTracker[cr.Namespace] = currentInstanceEventEmissionStatus
		}

	}

	if cr.Spec.Dex != nil && !reflect.DeepEqual(cr.Spec.Dex, &v1alpha1.ArgoCDDexSpec{}) {

		// Emit event for each instance providing users with deprecation notice for `.spec.dex` if not emitted already
		if currentInstanceEventEmissionStatus, ok := DeprecationEventEmissionTracker[cr.Namespace]; !ok || !currentInstanceEventEmissionStatus.DexSpecDeprecationWarningEmitted {
			err := argoutil.CreateEvent(r.Client, "Warning", "Deprecated", "`.spec.dex` is deprecated, and support will be removed in Argo CD Operator v0.8.0/OpenShift GitOps v1.10.0. Dex configuration can be managed through `.spec.sso.dex`", "DeprecationNotice", cr.ObjectMeta, cr.TypeMeta)
			if err != nil {
				return err
			}

			if !ok {
				currentInstanceEventEmissionStatus = DeprecationEventEmissionStatus{DexSpecDeprecationWarningEmitted: true}
			} else {
				currentInstanceEventEmissionStatus.DexSpecDeprecationWarningEmitted = true
			}
			DeprecationEventEmissionTracker[cr.Namespace] = currentInstanceEventEmissionStatus
		}

	}

	if cr.Spec.SSO != nil && (cr.Spec.SSO.Image != "" || cr.Spec.SSO.Version != "" ||
		cr.Spec.SSO.VerifyTLS != nil || cr.Spec.SSO.Resources != nil) {

		// Emit event for each instance providing users with deprecation notice for `.spec.SSO` subfields if not emitted already
		if currentInstanceEventEmissionStatus, ok := DeprecationEventEmissionTracker[cr.Namespace]; !ok || !currentInstanceEventEmissionStatus.SSOSpecDeprecationWarningEmitted {
			err := argoutil.CreateEvent(r.Client, "Warning", "Deprecated", "`.spec.SSO.Image`, `.spec.SSO.Version`, `.spec.SSO.Resources` and `.spec.SSO.VerifyTLS` are deprecated, and support will be removed in Argo CD Operator v0.8.0/OpenShift GitOps v1.10.0. Keycloak configuration can be managed through `.spec.sso.keycloak`", "DeprecationNotice", cr.ObjectMeta, cr.TypeMeta)
			if err != nil {
				return err
			}

			if !ok {
				currentInstanceEventEmissionStatus = DeprecationEventEmissionStatus{SSOSpecDeprecationWarningEmitted: true}
			} else {
				currentInstanceEventEmissionStatus.SSOSpecDeprecationWarningEmitted = true
			}
			DeprecationEventEmissionTracker[cr.Namespace] = currentInstanceEventEmissionStatus
		}
	}

	// case 1
	if cr.Spec.SSO == nil {

		errMsg := ""
		var err error

		// no SSO configured, nothing to do here
		if !UseDex(cr) {
			return nil
		}

		if (!isDexDisabled() && isDisableDexSet) && cr.Spec.Dex != nil && !reflect.DeepEqual(cr.Spec.Dex, &v1alpha1.ArgoCDDexSpec{}) && !cr.Spec.Dex.OpenShiftOAuth && cr.Spec.Dex.Config == "" {
			// dex is enabled but no dexconfig supplied. This will cause health probe to fail as per
			// https://github.com/argoproj-labs/argocd-operator/pull/615 ==> conflict
			errMsg = "must suppy valid dex configuration when dex is enabled"
			err = errors.New(illegalSSOConfiguration + errMsg)
			log.Error(err, fmt.Sprintf("Illegal expression of SSO configuration detetected for Argo CD %s in namespace %s. %s", cr.Name, cr.Namespace, errMsg))
			ssoConfigLegalStatus = ssoLegalFailed // set global indicator that SSO config has gone wrong
			_ = r.reconcileStatusSSOConfig(cr)
			return err
		}
	}

	if cr.Spec.SSO != nil {

		errMsg := ""
		var err error
		isError := false

		// case 2
		if cr.Spec.SSO.Provider == v1alpha1.SSOProviderTypeDex {
			// Relevant SSO settings at play are `DISABLE_DEX`, `.spec.dex`, `.spec.sso` fields, `.spec.sso.keycloak`

			if isDexDisabled() && isDisableDexSet {
				// DISABLE_DEX is true when `.spec.sso.provider` is set to dex ==> conflict
				errMsg = "cannot set DISABLE_DEX to true when dex is configured through .spec.sso"
				isError = true
			} else if cr.Spec.SSO.Dex == nil || (cr.Spec.SSO.Dex != nil && !cr.Spec.SSO.Dex.OpenShiftOAuth && cr.Spec.SSO.Dex.Config == "") {
				// sso provider specified as dex but no dexconfig supplied. This will cause health probe to fail as per
				// https://github.com/argoproj-labs/argocd-operator/pull/615 ==> conflict
				errMsg = "must suppy valid dex configuration when requested SSO provider is dex"
				isError = true
			} else if cr.Spec.SSO.Keycloak != nil {
				// new keycloak spec fields are expressed when `.spec.sso.provider` is set to dex ==> conflict
				errMsg = "cannot supply keycloak configuration in .spec.sso.keycloak when requested SSO provider is dex"
				isError = true
			} else if cr.Spec.Dex != nil && (cr.Spec.Dex.Image != "" || cr.Spec.Dex.Config != "" || cr.Spec.Dex.Resources != nil || len(cr.Spec.Dex.Groups) != 0 ||
				cr.Spec.Dex.Version != "" || cr.Spec.Dex.OpenShiftOAuth != cr.Spec.SSO.Dex.OpenShiftOAuth) {
				// old dex spec fields are expressed when `.spec.sso.provider` is set to dex instead of using new `.spec.sso.dex` ==> conflict
				errMsg = "cannot specify .spec.Dex fields when dex is configured through .spec.sso.dex"
				isError = true
			} else if cr.Spec.SSO.Image != "" || cr.Spec.SSO.Version != "" || cr.Spec.SSO.VerifyTLS != nil || cr.Spec.SSO.Resources != nil {
				// old keycloak spec fields expressed when `.spec.sso.provider` is set to dex ==> conflict
				errMsg = "cannot supply keycloak configuration in spec.sso when requested SSO provider is dex"
				isError = true
			}

			if isError {
				err = errors.New(illegalSSOConfiguration + errMsg)
				log.Error(err, fmt.Sprintf("Illegal expression of SSO configuration detetected for Argo CD %s in namespace %s. %s", cr.Name, cr.Namespace, errMsg))
				ssoConfigLegalStatus = ssoLegalFailed // set global indicator that SSO config has gone wrong
				_ = r.reconcileStatusSSOConfig(cr)
				return err
			}
		}

		// case 3
		if cr.Spec.SSO.Provider == v1alpha1.SSOProviderTypeKeycloak {
			// Relevant SSO settings at play are `DISABLE_DEX`, `.spec.dex`, `.spec.sso` fields, `.spec.sso.keycloak`, `.spec.sso.dex`

			if (cr.Spec.SSO.Keycloak != nil) && (cr.Spec.SSO.Image != "" || cr.Spec.SSO.Version != "" ||
				cr.Spec.SSO.Resources != nil || cr.Spec.SSO.VerifyTLS != nil) {
				// Keycloak specs expressed both in old `.spec.sso` fields as well as in `.spec.sso.keycloak` simultaneously and they don't match
				// ==> conflict
				errMsg = "cannot specify keycloak fields in .spec.sso when keycloak is configured through .spec.sso.keycloak"
				err = errors.New(illegalSSOConfiguration + errMsg)
				isError = true
			} else if cr.Spec.SSO.Dex != nil {
				// new dex spec fields are expressed when `.spec.sso.provider` is set to keycloak ==> conflict
				errMsg = "cannot supply dex configuration when requested SSO provider is keycloak"
				err = errors.New(illegalSSOConfiguration + errMsg)
				isError = true
			} else if (cr.Spec.Dex != nil && !reflect.DeepEqual(cr.Spec.Dex, &v1alpha1.ArgoCDDexSpec{}) && (cr.Spec.Dex.OpenShiftOAuth || cr.Spec.Dex.Config != "")) {
				// Keycloak configured as SSO provider, but dex config also present in argocd-cm. May cause both SSO providers to get
				// configured if Dex pods happen to be running due to `DEX_DISABLED` being set to false ==> conflict
				errMsg = "multiple SSO providers configured simultaneously"
				err = errors.New(multipleSSOConfiguration + errMsg)
				isError = true
			}
			// (cannot check against presence of DISABLE_DEX as erroring out here would break current behavior)

			if isError {
				log.Error(err, fmt.Sprintf("Illegal expression of SSO configuration deletected for Argo CD %s in namespace %s. %s", cr.Name, cr.Namespace, errMsg))
				ssoConfigLegalStatus = ssoLegalFailed // set global indicator that SSO config has gone wrong
				_ = r.reconcileStatusSSOConfig(cr)
				return err
			}
		}

		// case 4
		if cr.Spec.SSO.Provider == "" {

			if cr.Spec.SSO.Dex != nil ||
				// `.spec.sso.dex` expressed without specifying SSO provider ==> conflict
				cr.Spec.SSO.Keycloak != nil {
				// `.spec.sso.keycloak` expressed without specifying SSO provider ==> conflict

				errMsg = "Cannot specify SSO provider spec without specifying SSO provider type"
				err = errors.New(illegalSSOConfiguration + errMsg)
				log.Error(err, fmt.Sprintf("Cannot specify SSO provider spec without specifying SSO provider type for Argo CD %s in namespace %s.", cr.Name, cr.Namespace))
				ssoConfigLegalStatus = ssoLegalFailed // set global indicator that SSO config has gone wrong
				_ = r.reconcileStatusSSOConfig(cr)
				return err
			}
		}
	}

	// control reaching this point means that none of the illegal config combinations were detected. SSO is configured legally
	// set global indicator that SSO config has been successful
	ssoConfigLegalStatus = ssoLegalSuccess

	// reconcile resources based on enabled provider
	// keycloak
	if cr.Spec.SSO != nil && cr.Spec.SSO.Provider == argoprojv1a1.SSOProviderTypeKeycloak {

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

	_ = r.reconcileStatusSSOConfig(cr)

	return nil
}

func (r *ReconcileArgoCD) deleteSSOConfiguration(newCr *argoprojv1a1.ArgoCD, oldCr *argoprojv1a1.ArgoCD) error {

	log.Info("uninstalling existing SSO configuration")

	if oldCr.Spec.SSO.Provider == argoprojv1a1.SSOProviderTypeKeycloak {
		if err := deleteKeycloakConfiguration(newCr); err != nil {
			log.Error(err, "Unable to delete existing keycloak configuration")
			return err
		}
	} else if oldCr.Spec.SSO.Provider == argoprojv1a1.SSOProviderTypeDex {
		// Trigger reconciliation of Dex resources so they get deleted
		if err := r.deleteDexResources(newCr); err != nil {
			log.Error(err, "Unable to reconcile necessary resources for uninstallation of Dex")
			return err
		}
	}

	_ = r.reconcileStatusSSOConfig(newCr)
	return nil
}
