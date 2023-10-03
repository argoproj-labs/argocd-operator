package redis

import (
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RedisReconciler struct {
	Client   *client.Client
	Scheme   *runtime.Scheme
	Instance *argoproj.ArgoCD
	Logger   logr.Logger
}

func (rr *RedisReconciler) Reconcile() error {
	rr.Logger = ctrl.Log.WithName(RedisControllerComponent).WithValues("instance", rr.Instance.Name, "instance-namespace", rr.Instance.Namespace)

	// controller logic goes here
	return nil
}
