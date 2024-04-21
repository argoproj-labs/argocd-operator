package dex

import (
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DexReconciler struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Instance *argoproj.ArgoCD
	Logger   util.Logger

	Server ServerController
}

var (
	resourceName string
	component    string
)

// Reconcile consolidates all dex resources reconciliation calls. It serves as the single place to trigger both creation
// and deletion of dex resources based on the specified configuration of dex
func (dr *DexReconciler) Reconcile() error {
	dr.varSetter()

	if err := dr.reconcileRole(); err != nil {
		dr.Logger.Error(err, "failed to reconcile role")
		return err
	}

	if err := dr.reconcileRB(); err != nil {
		dr.Logger.Error(err, "failed to reconcile rolebinding")
		return err
	}

	if err := dr.reconcileServiceAccount(); err != nil {
		dr.Logger.Error(err, "failed to reconcile serviecaccount")
		return err
	}

	if err := dr.reconcileService(); err != nil {
		dr.Logger.Error(err, "failed to reconcile service")
		return err
	}

	if err := dr.reconcileDeployment(); err != nil {
		dr.Logger.Error(err, "failed to reconcile deployment")
		return err
	}

	return nil
}

func (dr *DexReconciler) DeleteResources() error {
	var deletionErr util.MultiError

	if err := dr.deleteDeployment(resourceName, dr.Instance.Namespace); err != nil {
		dr.Logger.Error(err, "failed to delete deployment")
		deletionErr.Append(err)
	}

	if err := dr.deleteService(resourceName, dr.Instance.Namespace); err != nil {
		dr.Logger.Error(err, "failed to delete service")
		deletionErr.Append(err)
	}

	if err := dr.deleteRoleBinding(resourceName, dr.Instance.Namespace); err != nil {
		dr.Logger.Error(err, "failed to delete roleBinding")
		deletionErr.Append(err)
	}

	if err := dr.deleteRole(resourceName, dr.Instance.Namespace); err != nil {
		dr.Logger.Error(err, "DeleteResources: failed to delete role")
		deletionErr.Append(err)
	}

	if err := dr.deleteServiceAccount(resourceName, dr.Instance.Namespace); err != nil {
		dr.Logger.Error(err, "failed to delete serviceaccount")
		deletionErr.Append(err)
	}
	return nil
}

func (dr *DexReconciler) varSetter() {
	component = common.DexServerComponent
	resourceName = argoutil.GenerateResourceName(dr.Instance.Name, common.DexSuffix)
}
