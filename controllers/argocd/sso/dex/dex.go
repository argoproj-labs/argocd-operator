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

func (dr *DexReconciler) Reconcile() error {
	dr.varSetter()

	return nil
}

func (dr *DexReconciler) varSetter() {
	component = common.DexServerComponent
	resourceName = argoutil.GenerateResourceName(dr.Instance.Name, common.DexSuffix)
}
