package sso

import (
	"context"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/pkg/errors"
	"k8s.io/client-go/util/retry"
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
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := sr.Client.Status().Update(context.TODO(), sr.Instance); err != nil {
			return errors.Wrap(err, "UpdateInstanceStatus: failed to update instance status")
		}
		return nil
	})

	if err != nil {
		// May be conflict if max retries were hit, or may be something unrelated
		// like permissions or a network error
		return err
	}
	return nil
}
