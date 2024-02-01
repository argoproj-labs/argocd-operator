package reposerver

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

type RepoServerReconciler struct {
	Client     client.Client
	Scheme     *runtime.Scheme
	Instance   *argoproj.ArgoCD
	Logger     *util.Logger
	TLSEnabled bool

	Appcontroller AppController
	Server        ServerController
	Redis         RedisController
}

var (
	resourceName        string
	resourceMetricsName string
	component           string
)

func (rsr *RepoServerReconciler) Reconcile() error {
	rsr.varSetter()

	// check if TLS is enabled
	rsr.UseTLS()

	if err := rsr.reconcileServiceAccount(); err != nil {
		rsr.Logger.Error(err, "failed to reconcile serviceaccount")
		return err
	}

	if err := rsr.reconcileService(); err != nil {
		rsr.Logger.Error(err, "failed to reconcile service")
		return err
	}

	if rsr.Instance.Spec.Prometheus.Enabled {
		if err := rsr.reconcileServiceMonitor(); err != nil {
			rsr.Logger.Error(err, "failed to reconcile service monitor")
			return err
		}
	} else {
		if err := rsr.deleteServiceMonitor(resourceMetricsName, rsr.Instance.Namespace); err != nil {
			rsr.Logger.Error(err, "DeleteResources: failed to delete serviceMonitor")
			return err
		}
	}

	if err := rsr.reconcileTLSSecret(); err != nil {
		rsr.Logger.Error(err, "failed to reconcile TLS secret")
		return err
	}

	if err := rsr.reconcileDeployment(); err != nil {
		rsr.Logger.Error(err, "failed to reconcile deployment")
		return err
	}

	return nil
}

// DeleteResources triggers deletion of all repo-server resources
func (rsr *RepoServerReconciler) DeleteResources() error {
	var deletionErr util.MultiError

	// delete deployment
	err := rsr.deleteDeployment(resourceName, rsr.Instance.Namespace)
	if err != nil {
		rsr.Logger.Error(err, "DeleteResources")
		deletionErr.Append(err)
	}

	// delete service monitor
	err = rsr.deleteServiceMonitor(resourceName, rsr.Instance.Namespace)
	if err != nil {
		rsr.Logger.Error(err, "DeleteResources")
		deletionErr.Append(err)
	}

	// delete service
	err = rsr.deleteService(resourceName, rsr.Instance.Namespace)
	if err != nil {
		rsr.Logger.Error(err, "DeleteResources")
		deletionErr.Append(err)
	}

	// delete serviceaccount
	err = rsr.deleteServiceAccount(resourceName, rsr.Instance.Namespace)
	if err != nil {
		rsr.Logger.Error(err, "DeleteResources")
		deletionErr.Append(err)
	}

	// delete TLS secret
	err = rsr.deleteSecret(common.ArgoCDRepoServerTLSSecretName, rsr.Instance.Namespace)
	if err != nil {
		rsr.Logger.Error(err, "DeleteResources")
		deletionErr.Append(err)
	}

	return deletionErr.ErrOrNil()
}

func (rsr *RepoServerReconciler) TriggerRollout(key string) error {
	if err := rsr.TriggerDeploymentRollout(resourceName, rsr.Instance.Namespace, key); err != nil {
		rsr.Logger.Error(err, "TriggerRollout: failed to rollout repo-server deployment")
		return err
	}
	return nil
}

func (rsr *RepoServerReconciler) varSetter() {
	component = common.RepoServerComponent
	resourceName = argoutil.GenerateResourceName(rsr.Instance.Name, common.RepoServerSuffix)
	resourceMetricsName = argoutil.GenerateResourceName(rsr.Instance.Name, common.RepoServerSuffix, common.MetricsSuffix)
}
