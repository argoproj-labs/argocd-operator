package redis

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	cntrlr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
)

type RedisReconciler struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Instance *argoproj.ArgoCD
	Logger   logr.Logger

	Appcontroller AppController
	TLSEnabled    bool
}

var (
	resourceName        string
	HAProxyResourceName string
	HAResourceName      string
	resourceLabels      map[string]string
)

func (rr *RedisReconciler) Reconcile() error {

	// controller logic goes here

	rr.Logger = cntrlr.Log.WithName(common.RedisComponent).WithValues("instance", rr.Instance.Name, "instance-namespace", rr.Instance.Namespace)

	resourceName = argoutil.GenerateResourceName(rr.Instance.Name, common.RedisComponent)
	HAResourceName = argoutil.GenerateResourceName(rr.Instance.Name, common.RedisHASuffix)
	HAProxyResourceName = argoutil.GenerateResourceName(rr.Instance.Name, common.RedisHAProxySuffix)
	resourceLabels = common.DefaultResourceLabels(resourceName, rr.Instance.Name, common.RedisComponent)

	// check if TLS needs to be used
	rr.TLSEnabled = rr.UseTLS()

	if rr.Instance.Spec.HA.Enabled {

	} else {

	}

	return nil
}

func (rr *RedisReconciler) TriggerRollout() error {
	return nil
}

func (rr *RedisReconciler) UpdateInstanceStatus() error {
	return rr.Client.Status().Update(context.TODO(), rr.Instance)
}
