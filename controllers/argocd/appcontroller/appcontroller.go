package appcontroller

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

type AppControllerReconciler struct {
	Client            client.Client
	Scheme            *runtime.Scheme
	Instance          *argoproj.ArgoCD
	ClusterScoped     bool
	Logger            logr.Logger
	ManagedNamespaces map[string]string
	SourceNamespaces  map[string]string
}

func (acr *AppControllerReconciler) Reconcile() error {

	// controller logic goes here
	return nil
}

// TO DO: finish this
func (acr *AppControllerReconciler) TriggerRollout(key string) error {
	return acr.TriggerStatefulSetRollout("", "", key)
}
