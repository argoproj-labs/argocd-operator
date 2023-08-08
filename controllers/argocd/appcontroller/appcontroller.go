package appcontroller

import (
	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AppControllerReconciler struct {
	Client            *client.Client
	Scheme            *runtime.Scheme
	Instance          *v1alpha1.ArgoCD
	ClusterScoped     bool
	Logger            logr.Logger
	ManagedNamespaces map[string]string
	SourceNamespaces  map[string]string
}

func (acr *AppControllerReconciler) Reconcile() error {

	// controller logic goes here
	return nil
}
