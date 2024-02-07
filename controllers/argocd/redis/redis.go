package redis

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

type RedisReconciler struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Instance *argoproj.ArgoCD
	Logger   *util.Logger
}

func (rr *RedisReconciler) Reconcile() error {

	// controller logic goes here
	return nil
}
