package sso

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/sso/dex"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

type SSOReconciler struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Instance *argoproj.ArgoCD
	Logger   util.Logger

	DexController *dex.DexReconciler
}

func (sr *SSOReconciler) Reconcile() error {

	// controller logic goes here
	return nil
}

func (sr *SSOReconciler) GetProvider() argoproj.SSOProviderType {
	if sr.Instance.Spec.SSO == nil {
		return ""
	}
	return sr.Instance.Spec.SSO.Provider
}
