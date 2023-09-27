package redis

import (
	"github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RedisReconciler struct {
	Client   *client.Client
	Scheme   *runtime.Scheme
	Instance *v1beta1.ArgoCD
	Logger   logr.Logger
}

func (rr *RedisReconciler) Reconcile() error {

	// controller logic goes here
	return nil
}
