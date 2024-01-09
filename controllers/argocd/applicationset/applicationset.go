package applicationset

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

type ApplicationSetReconciler struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Instance *argoproj.ArgoCD
	Logger   logr.Logger
}

var (
	resourceName   string
	resourceLabels map[string]string
)

func (asr *ApplicationSetReconciler) Reconcile() error {

	asr.Logger = ctrl.Log.WithName(common.AppSetControllerComponent).WithValues("instance", asr.Instance.Name, "instance-namespace", asr.Instance.Namespace)

	resourceName = argoutil.GenerateUniqueResourceName(asr.Instance.Name, asr.Instance.Namespace, common.AppSetControllerComponent)
	resourceLabels = common.DefaultResourceLabels(resourceName, asr.Instance.Name, common.AppSetControllerComponent)

	if err := asr.reconcileServiceAccount(); err != nil {
		asr.Logger.Info("reconciling applicationSet serviceaccount")
		return err
	}

	if err := asr.reconcileRole(); err != nil {
		asr.Logger.Info("reconciling applicationSet role")
		return err
	}

	if err := asr.reconcileRoleBinding(); err != nil {
		asr.Logger.Info("reconciling applicationSet roleBinding")
		return err
	}

	if asr.Instance.Spec.ApplicationSet.WebhookServer.Route.Enabled {
		if err := asr.reconcileWebhookRoute(); err != nil {
			asr.Logger.Info("reconciling applicationSet webhook route")
			return err
		}
	} else {
		if err := asr.deleteWebhookRoute(common.AppSetWebhookRouteName, asr.Instance.Namespace); err != nil {
			asr.Logger.Error(err, "deleting applicationSet webhook route: failed to delete webhook route")
			return err
		}
	}

	if err := asr.reconcileService(); err != nil {
		asr.Logger.Info("reconciling applicationSet service")
		return err
	}

	if err := asr.reconcileDeployment(); err != nil {
		asr.Logger.Info("reconciling applicationSet deployment")
		return err
	}

	return nil
}

func (asr *ApplicationSetReconciler) DeleteResources() error {

	var deletionError error = nil

	if err := asr.deleteDeployment(resourceName, asr.Instance.Namespace); err != nil {
		asr.Logger.Error(err, "DeleteResources: failed to delete deployment")
		deletionError = err
	}

	if err := asr.deleteService(resourceName, asr.Instance.Namespace); err != nil {
		asr.Logger.Error(err, "DeleteResources: failed to delete service")
		deletionError = err
	}

	if err := asr.deleteWebhookRoute(resourceName, asr.Instance.Namespace); err != nil {
		asr.Logger.Error(err, "DeleteResources: failed to delete webhook service")
		deletionError = err
	}

	if err := asr.deleteRoleBinding(resourceName, asr.Instance.Namespace); err != nil {
		asr.Logger.Error(err, "DeleteResources: failed to delete roleBinding")
		deletionError = err
	}

	if err := asr.deleteRole(resourceName, asr.Instance.Namespace); err != nil {
		asr.Logger.Error(err, "DeleteResources: failed to delete role")
		deletionError = err
	}

	if err := asr.deleteServiceAccount(resourceName, asr.Instance.Namespace); err != nil {
		asr.Logger.Error(err, "DeleteResources: failed to delete serviceaccount")
		deletionError = err
	}

	return deletionError
}
