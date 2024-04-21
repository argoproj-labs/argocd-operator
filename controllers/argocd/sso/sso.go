package sso

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/sso/dex"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/pkg/errors"
)

type SSOReconciler struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Instance *argoproj.ArgoCD
	Logger   *util.Logger

	DexController *dex.DexReconciler
}

const (
	SSOLegalUnknown         string = "Unknown"
	SSOLegalSucces          string = "Success"
	SSOLegalFailed          string = "Failed"
	illegalSSOConfiguration string = "illegal SSO configuration: "
)

var (
	ssoConfigLegalStatus = SSOLegalUnknown
)

func (sr *SSOReconciler) Reconcile() error {

	// controller logic goes here
	// reset ssoConfigLegalStatus at the beginning of each SSO reconciliation round
	ssoConfigLegalStatus = SSOLegalUnknown

	provider := sr.GetProvider(sr.Instance)

	// case 1
	if provider == "" {
		// no SSO configured, nothing to do here
		return nil
	}

	errMsg := ""
	var err error
	isError := false

	// case 2
	if provider == argoproj.SSOProviderTypeDex {
		if sr.Instance.Spec.SSO.Dex == nil || (sr.Instance.Spec.SSO.Dex != nil && !sr.Instance.Spec.SSO.Dex.OpenShiftOAuth && sr.Instance.Spec.SSO.Dex.Config == "") {
			// sso provider specified as dex but no dexconfig supplied. This will cause health probe to fail as per
			// https://github.com/argoproj-labs/argocd-operator/pull/615 ==> conflict
			errMsg = "must supply valid dex configuration when requested SSO provider is dex"
			isError = true
		} else if sr.Instance.Spec.SSO.Keycloak != nil {
			// new keycloak spec fields are expressed when `.spec.sso.provider` is set to dex ==> conflict
			errMsg = "cannot supply keycloak configuration in .spec.sso.keycloak when requested SSO provider is dex"
			isError = true
		}

		if isError {
			err = errors.New(errMsg)
			sr.Logger.Error(err, illegalSSOConfiguration)
			ssoConfigLegalStatus = SSOLegalFailed // set global indicator that SSO config has gone wrong
			return err
		}
	}

	// case 3
	if provider == argoproj.SSOProviderTypeKeycloak {
		if sr.Instance.Spec.SSO.Dex != nil {
			// new dex spec fields are expressed when `.spec.sso.provider` is set to keycloak ==> conflict
			errMsg = "cannot supply dex configuration when requested SSO provider is keycloak"
			isError = true
		}

		if isError {
			err = errors.New(errMsg)
			sr.Logger.Error(err, illegalSSOConfiguration)
			ssoConfigLegalStatus = SSOLegalFailed // set global indicator that SSO config has gone wrong
			return err
		}
	}

	// case 4
	if sr.Instance.Spec.SSO.Provider.ToLower() == "" {
		if sr.Instance.Spec.SSO.Dex != nil ||
			// `.spec.sso.dex` expressed without specifying SSO provider ==> conflict
			sr.Instance.Spec.SSO.Keycloak != nil {
			// `.spec.sso.keycloak` expressed without specifying SSO provider ==> conflict

			errMsg = "Cannot specify SSO provider spec without specifying SSO provider type"
			err = errors.New(errMsg)
			sr.Logger.Error(err, illegalSSOConfiguration)
			ssoConfigLegalStatus = SSOLegalFailed // set global indicator that SSO config has gone wrong
			return err
		}
	}

	// case 5
	if sr.Instance.Spec.SSO.Provider.ToLower() != argoproj.SSOProviderTypeDex && sr.Instance.Spec.SSO.Provider.ToLower() != argoproj.SSOProviderTypeKeycloak {
		// `.spec.sso.provider` contains unsupported value

		errMsg = fmt.Sprintf("Unsupported SSO provider type. Supported providers are %s and %s", argoproj.SSOProviderTypeDex, argoproj.SSOProviderTypeKeycloak)
		err = errors.New(errMsg)
		sr.Logger.Error(err, illegalSSOConfiguration)
		ssoConfigLegalStatus = SSOLegalFailed // set global indicator that SSO config has gone wrong
		return err
	}

	// control reaching this point means that none of the illegal config combinations were detected. SSO is configured legally
	// set global indicator that SSO config has been successful
	ssoConfigLegalStatus = SSOLegalSucces

	switch provider {
	case argoproj.SSOProviderTypeDex:
		// TO DO: Delete keycloak resources

		if err := sr.DexController.Reconcile(); err != nil {
			return err
		}
	case argoproj.SSOProviderTypeKeycloak:
		// Delete dex resources
		// errors are already logged earlier, no need to handle here
		_ = sr.DexController.DeleteResources()

		// TO DO: handle keycloak reconciliation
	}

	return nil
}

func (sr *SSOReconciler) DeleteResources(newCr *argoproj.ArgoCD, oldCr *argoproj.ArgoCD) error {
	provider := sr.GetProvider(oldCr)

	switch provider {
	case argoproj.SSOProviderTypeKeycloak:
		// TO DO: Delete keycloak resources

	case argoproj.SSOProviderTypeDex:
		// Delete dex resources
		if err := sr.DexController.DeleteResources(); err != nil {
			return err
		}
	}

	return nil
}

func (sr *SSOReconciler) GetProvider(cr *argoproj.ArgoCD) argoproj.SSOProviderType {
	if cr.Spec.SSO == nil {
		return ""
	}
	return cr.Spec.SSO.Provider.ToLower()
}

func (sr *SSOReconciler) GetStatus() string {
	return ssoConfigLegalStatus
}
