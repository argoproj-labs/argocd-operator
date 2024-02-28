package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/openshift"
	"github.com/argoproj-labs/argocd-operator/pkg/util"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

type ServerReconciler struct {
	Client            client.Client
	Scheme            *runtime.Scheme
	Instance          *argoproj.ArgoCD
	Logger            *util.Logger
	ClusterScoped     bool
	ManagedNamespaces map[string]string
	SourceNamespaces  map[string]string

	RepoServer RepoServerController
	Redis      RedisController
	Dex        DexController
}

var (
	resourceName        string
	clusterResourceName string
	component           string
)

func (sr *ServerReconciler) Reconcile() error {
	sr.varSetter()

	// perform resource reconciliation
	if err := sr.reconcileServiceAccount(); err != nil {
		return err
	}

	if err := sr.reconcileClusterRole(); err != nil {
		return err
	}

	if err := sr.reconcileClusterRoleBinding(); err != nil {
		return err
	}

	if err := sr.reconcileRole(); err != nil {
		return err
	}

	if err := sr.reconcileRoleBinding(); err != nil {
		return err
	}

	if err := sr.reconcileDeployment(); err != nil {
		return err
	}

	if err := sr.reconcileService(); err != nil {
		return err
	}

	if err := sr.reconcileHorizontalPodAutoscaler(); err != nil {
		return err
	}

	if err := sr.reconcileIngresses(); err != nil {
		return err
	}

	if openshift.IsRouteAPIAvailable() {
		if err := sr.reconcileRoute(); err != nil {
			return err
		}
	}

	return nil
}

func (sr *ServerReconciler) DeleteResources() error {

	if openshift.IsRouteAPIAvailable() {
		if err := sr.deleteRoute(resourceName, sr.Instance.Namespace); err != nil {
			return err
		}
	}

	if err := sr.deleteIngresses(resourceName, sr.Instance.Namespace); err != nil {
		return err
	}

	if err := sr.deleteHorizontalPodAutoscaler(resourceName, sr.Instance.Namespace); err != nil {
		return err
	}

	if err := sr.deleteService(resourceName, sr.Instance.Namespace); err != nil {
		return err
	}

	if err := sr.deleteDeployment(resourceName, sr.Instance.Namespace); err != nil {
		return err
	}

	if err := sr.deleteRoleBinding(resourceName, sr.Instance.Namespace); err != nil {
		return err
	}

	if err := sr.deleteRole(resourceName, sr.Instance.Namespace); err != nil {
		return err
	}

	if err := sr.deleteClusterRoleBinding(clusterResourceName); err != nil {
		return err
	}

	if err := sr.deleteClusterRole(clusterResourceName); err != nil {
		return err
	}

	if err := sr.deleteServiceAccount(resourceName, sr.Instance.Namespace); err != nil {
		return err
	}

	return nil
}

func (sr *ServerReconciler) varSetter() {
	component = common.ServerComponent
	resourceName = argoutil.GenerateResourceName(sr.Instance.Name, common.ServerSuffix)
	clusterResourceName = argoutil.GenerateUniqueResourceName(sr.Instance.Name, sr.Instance.Namespace, common.ServerSuffix)
}

// TO DO: fix this
func (acr *ServerReconciler) TriggerRollout(key string) error {
	return acr.TriggerDeploymentRollout("", "", key)
}
