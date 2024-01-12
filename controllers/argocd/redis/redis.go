package redis

import (
	"fmt"

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

	Appcontroller  AppController
	Server         Server
	RepoServer     RepoServer
	TLSEnabled     bool
	IsOpenShiftEnv bool
}

var (
	resourceName   string
	component      string
	resourceLabels map[string]string
)

func (rr *RedisReconciler) Reconcile() error {

	// controller logic goes here

	rr.Logger = cntrlr.Log.WithName(common.RedisController).WithValues("instance", rr.Instance.Name, "instance-namespace", rr.Instance.Namespace)
	component = common.RedisComponent
	resourceName = argoutil.GenerateResourceName(rr.Instance.Name, component)
	resourceLabels = common.DefaultResourceLabels(resourceName, rr.Instance.Name, component)

	// check if TLS needs to be used
	rr.TLSEnabled = rr.UseTLS()

	if rr.Instance.Spec.HA.Enabled {

	} else {

	}

	return nil
}

func (rr *RedisReconciler) TriggerRollout(key string) error {

	if rr.Instance.Spec.HA.Enabled {
		errs := rr.TriggerHARollout(key)
		if len(errs) > 0 {
			return fmt.Errorf("TriggerRollout: failed to trigger HA rollout")
		}
	} else {
		err := rr.TriggerDeploymentRollout(resourceName, rr.Instance.Namespace, key)
		if err != nil {
			return fmt.Errorf("TriggerRollout: failed to trigger HA rollout: %w", err)
		}
	}
	return nil
}

func (rr *RedisReconciler) DeleteResources() error {}
