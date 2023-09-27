package applicationset

import (
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ApplicationSetReconciler struct {
	Client   *client.Client
	Scheme   *runtime.Scheme
	Instance *argoproj.ArgoCD
	Logger   logr.Logger
}

func (asr *ApplicationSetReconciler) Reconcile() error {

	// controller logic goes here
	return nil
}
