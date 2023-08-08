package secret

import (
	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SecretReconciler struct {
	Client            *client.Client
	Scheme            *runtime.Scheme
	Instance          *v1alpha1.ArgoCD
	ClusterScoped     bool
	Logger            logr.Logger
	ManagedNamespaces map[string]string
}

func (sr *SecretReconciler) Reconcile() error {

	// controller logic goes here
	return nil
}
