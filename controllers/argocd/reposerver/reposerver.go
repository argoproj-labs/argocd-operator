package reposerver

import (
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RepoServerReconciler struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Instance *argoproj.ArgoCD
	Logger   logr.Logger
}

var (
	resourceName   string
	resourceLabels map[string]string
)

func (rsr *RepoServerReconciler) Reconcile() error {
	rsr.Logger = ctrl.Log.WithName(ArgoCDRepoServerControllerComponent).WithValues("instance", rsr.Instance.Name, "instance-namespace", rsr.Instance.Namespace)
	resourceName = util.GenerateResourceName(rsr.Instance.Name, ArgoCDRepoServerControllerComponent)
	resourceLabels = common.DefaultLabels(resourceName, rsr.Instance.Name, ArgoCDRepoServerControllerComponent)

	if err := rsr.reconcileService(); err != nil {
		rsr.Logger.Info("reconciling repo server service")
		return err
	}

	if err := rsr.reconcileTLSSecret(); err != nil {
		rsr.Logger.Info("reconciling repo server secret")
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

	if err := rsr.DeleteDeployment(resourceName, rsr.Instance.Namespace); err != nil {
		rsr.Logger.Error(err, "DeleteResources: failed to delete deployment")
		deletionError = err
	}

	if err := rsr.DeleteTLSSecret(rsr.Instance.Namespace); err != nil {
		rsr.Logger.Error(err, "DeleteResources: failed to delete secret")
		deletionError = err
	}

	if err := rsr.DeleteService(resourceName, rsr.Instance.Namespace); err != nil {
		rsr.Logger.Error(err, "DeleteResources: failed to delete service")
		deletionError = err
	}

	return deletionError
}
