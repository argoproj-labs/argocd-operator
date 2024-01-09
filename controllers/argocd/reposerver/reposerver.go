package reposerver

import (
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AppController interface {
	TriggerAppControllerStatefulSetRollout() error
}

type Server interface {
	TriggerServerDeploymentRollout() error
}

type RepoServerReconciler struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Instance *argoproj.ArgoCD
	Logger   logr.Logger

	AppController    AppController
	ServerController Server
}

var (
	resourceName   string
	resourceLabels map[string]string
	useTLSForRedis bool
)

func (rsr *RepoServerReconciler) Reconcile() error {
	rsr.Logger = ctrl.Log.WithName(common.RepoServerControllerComponent).WithValues("instance", rsr.Instance.Name, "instance-namespace", rsr.Instance.Namespace)
	resourceName = argoutil.GenerateResourceName(rsr.Instance.Name, common.RepoServerControllerComponent)
	resourceLabels = common.DefaultResourceLabels(resourceName, rsr.Instance.Name, common.RepoServerControllerComponent)
	useTLSForRedis = rsr.Instance.Spec.Repo.WantsAutoTLS()

	if err := rsr.reconcileService(); err != nil {
		rsr.Logger.Info("reconciling repo server service")
		return err
	}

	if rsr.Instance.Spec.Prometheus.Enabled {
		if err := rsr.reconcileServiceMonitor(); err != nil {
			rsr.Logger.Info("reconciling repo server serviceMonitor")
			return err
		}
	} else {
		if err := rsr.deleteServiceMonitor(resourceName, rsr.Instance.Namespace); err != nil {
			rsr.Logger.Error(err, "DeleteResources: failed to delete serviceMonitor")
			return err
		}
	}

	if err := rsr.reconcileTLSSecret(); err != nil {
		rsr.Logger.Info("reconciling repo server tls secret")
		return err
	}

	if err := rsr.reconcileDeployment(); err != nil {
		rsr.Logger.Info("reconciling repo server deployment")
		return err
	}

	return nil
}

func (rsr *RepoServerReconciler) DeleteResources() error {

	var deletionError error = nil

	if err := rsr.deleteDeployment(resourceName, rsr.Instance.Namespace); err != nil {
		rsr.Logger.Error(err, "DeleteResources: failed to delete deployment")
		deletionError = err
	}

	if err := rsr.deleteTLSSecret(rsr.Instance.Namespace); err != nil {
		rsr.Logger.Error(err, "DeleteResources: failed to delete secret")
		deletionError = err
	}

	if err := rsr.deleteServiceMonitor(resourceName, rsr.Instance.Namespace); err != nil {
		rsr.Logger.Error(err, "DeleteResources: failed to delete serviceMonitor")
		deletionError = err
	}

	if err := rsr.deleteService(resourceName, rsr.Instance.Namespace); err != nil {
		rsr.Logger.Error(err, "DeleteResources: failed to delete service")
		deletionError = err
	}

	return deletionError
}
