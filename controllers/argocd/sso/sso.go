package sso

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

type SSOReconciler struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Instance *argoproj.ArgoCD
	Logger   logr.Logger
}

func (sr *SSOReconciler) Reconcile() error {

	// controller logic goes here
	return nil
}
