package sso

import argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"

// GetServerAddress will return the Redis service address for the given ArgoCD instance
func (sr *SSOReconciler) GetServerAddress() string {
	provider := sr.GetProvider(sr.Instance)

	switch provider {
	case argoproj.SSOProviderTypeDex:
		return sr.DexController.GetServerAddress()
	}
	return ""
}
