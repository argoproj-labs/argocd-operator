package sso

import (
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
)

// reconcileStatus will ensure that the sso status is updated for the given ArgoCD instance
func (sr *SSOReconciler) ReconcileStatus() error {

	// set status to track ssoConfigLegalStatus so it is always up to date with latest sso situation
	status := ssoConfigLegalStatus

	// perform dex/keycloak status reconciliation only if sso configurations are legal
	if status == SSOLegalSucces {
		provider := sr.GetProvider(sr.Instance)
		switch provider {
		case argoproj.SSOProviderTypeKeycloak:
			// TO DO: get status from keycloak
		case argoproj.SSOProviderTypeDex:
			status = sr.DexController.ReconcileStatus()
		}
	}

	if sr.Instance.Status.SSO != status {
		sr.Instance.Status.SSO = status
	}

	return sr.updateInstanceStatus()
}

func (sr *SSOReconciler) updateInstanceStatus() error {
	return resource.UpdateStatusSubResource(sr.Instance, sr.Client)
}
