package argocd

import (
	"context"
	"errors"
	"fmt"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	ssoLegalUnknown         string = "Unknown"
	ssoLegalSuccess         string = "Success"
	ssoLegalFailed          string = "Failed"
	illegalSSOConfiguration string = "illegal SSO configuration: "
)

var (
	ssoConfigLegalStatus string
)

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

// reconcileStatusSSOConfig will ensure that the SSOConfig status is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusSSO(cr *argoproj.ArgoCD) error {

	// set status to track ssoConfigLegalStatus so it is always up to date with latest sso situation
	status := ssoConfigLegalStatus

	// perform dex/keycloak status reconciliation only if sso configurations are legal
	if status == ssoLegalSuccess {
		if cr.Spec.SSO != nil && cr.Spec.SSO.Provider.ToLower() == argoproj.SSOProviderTypeDex {
			return r.reconcileStatusDex(cr)
		} else if cr.Spec.SSO != nil && cr.Spec.SSO.Provider.ToLower() == argoproj.SSOProviderTypeKeycloak {
			return r.reconcileStatusKeycloak(cr)
		}
	} else {
		// illegal/unknown sso configurations
		if cr.Status.SSO != status {
			cr.Status.SSO = status
			return r.Client.Status().Update(context.TODO(), cr)
		}
	}

	return nil
}
