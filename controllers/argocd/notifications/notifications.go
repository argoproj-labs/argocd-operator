package notifications

import (
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NotificationsReconciler struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Instance *argoproj.ArgoCD
	Logger   logr.Logger
}

var (
	resourceName   string
	resourceLabels map[string]string
)

func (nr *NotificationsReconciler) Reconcile() error {

	nr.Logger = ctrl.Log.WithName(common.NotificationsControllerComponent).WithValues("instance", nr.Instance.Name, "instance-namespace", nr.Instance.Namespace)

	resourceName = util.GenerateUniqueResourceName(nr.Instance.Name, nr.Instance.Namespace, common.NotificationsControllerComponent)
	resourceLabels = common.DefaultLabels(resourceName, nr.Instance.Name, common.NotificationsControllerComponent)

	if err := nr.reconcileServiceAccount(); err != nil {
		nr.Logger.Info("reconciling notifications serviceaccount")
		return err
	}

	if err := nr.reconcileRole(); err != nil {
		nr.Logger.Info("reconciling notifications role")
		return err
	}

	if err := nr.reconcileRoleBinding(); err != nil {
		nr.Logger.Info("reconciling notifications roleBinding")
		return err
	}

	if err := nr.reconcileConfigMap(); err != nil {
		nr.Logger.Info("reconciling notifications configmap")
		return err
	}

	if err := nr.reconcileSecret(); err != nil {
		nr.Logger.Info("reconciling notifications secret")
		return err
	}

	if err := nr.reconcileDeployment(); err != nil {
		nr.Logger.Info("reconciling notifications deployment")
		return err
	}

	return nil
}

func (nr *NotificationsReconciler) DeleteResources() error {

	var deletionError error = nil

	if err := nr.deleteDeployment(resourceName, nr.Instance.Namespace); err != nil {
		nr.Logger.Error(err, "DeleteResources: failed to delete deployment")
		deletionError = err
	}

	if err := nr.deleteSecret(nr.Instance.Namespace); err != nil {
		nr.Logger.Error(err, "DeleteResources: failed to delete secret")
		deletionError = err
	}

	if err := nr.deleteConfigMap(nr.Instance.Namespace); err != nil {
		nr.Logger.Error(err, "DeleteResources: failed to delete configmap")
		deletionError = err
	}

	if err := nr.deleteRoleBinding(resourceName, nr.Instance.Namespace); err != nil {
		nr.Logger.Error(err, "DeleteResources: failed to delete roleBinding")
		deletionError = err
	}

	if err := nr.deleteRole(resourceName, nr.Instance.Namespace); err != nil {
		nr.Logger.Error(err, "DeleteResources: failed to delete role")
		deletionError = err
	}

	if err := nr.deleteServiceAccount(resourceName, nr.Instance.Namespace); err != nil {
		nr.Logger.Error(err, "DeleteResources: failed to delete serviceaccount")
		deletionError = err
	}

	return deletionError
}
