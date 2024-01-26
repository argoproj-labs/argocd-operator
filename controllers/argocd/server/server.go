package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/openshift"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

type ServerReconciler struct {
	Client            client.Client
	Scheme            *runtime.Scheme
	Instance          *argoproj.ArgoCD
	Logger            logr.Logger
	ClusterScoped     bool
	ManagedNamespaces map[string]string
	SourceNamespaces  map[string]string
}

var (
	resourceName   				string
	uniqueResourceName 			string
	component      				string
	appcontrollerResourceName 	string
)

func (sr *ServerReconciler) Reconcile() error {

	sr.Logger = ctrl.Log.WithName(common.ArgoCDServerController).WithValues("instance", sr.Instance.Name, "instance-namespace", sr.Instance.Namespace)

	component = common.ArgoCDServerComponent
	resourceName = argoutil.GenerateResourceName(sr.Instance.Name, component)
	uniqueResourceName = argoutil.GenerateUniqueResourceName(sr.Instance.Name, sr.Instance.Namespace, component)
	appcontrollerResourceName = argoutil.GenerateResourceName(sr.Instance.Name, common.ArgoCDApplicationControllerComponent)

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

	if err := sr.reconcileRoles(); err != nil {
		return err
	}

	if err := sr.reconcileRoleBindings(); err != nil {
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

// TO DO: fix this
func (acr *ServerReconciler) TriggerRollout(key string) error {
	return acr.TriggerDeploymentRollout("", "", key)
}

func (sr *ServerReconciler) DeleteResources() error {

	if openshift.IsRouteAPIAvailable() {
		if err := sr.deleteRoute(resourceName, sr.Instance.Namespace); err != nil {
			return err
		}
	}

	if err := sr.deleteIngresses(resourceName,  sr.Instance.Namespace); err != nil {
		return err
	}

	if err := sr.deleteHorizontalPodAutoscaler(resourceName,  sr.Instance.Namespace); err != nil {
		return err
	}

	if err := sr.deleteService(resourceName,  sr.Instance.Namespace); err != nil {
		return err
	}

	if err := sr.deleteDeployment(resourceName,  sr.Instance.Namespace); err != nil {
		return err
	}

	if err := sr.deleteRoleBindings(resourceName, uniqueResourceName); err != nil {
		return err
	}

	if err := sr.deleteRoles(resourceName, uniqueResourceName); err != nil {
		return err
	}

	if err := sr.deleteClusterRoleBinding(uniqueResourceName); err != nil {
		return err
	}

	if err := sr.deleteClusterRole(uniqueResourceName); err != nil {
		return err
	}

	if err := sr.deleteServiceAccount(resourceName, sr.Instance.Namespace); err != nil {
		return err
	}

	return nil
}
