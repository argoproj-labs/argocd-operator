package appcontroller

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/monitoring"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

type AppControllerReconciler struct {
	Client            client.Client
	Scheme            *runtime.Scheme
	Instance          *argoproj.ArgoCD
	ClusterScoped     bool
	Logger            *util.Logger
	ManagedNamespaces map[string]string
	SourceNamespaces  map[string]string

	Redis      RedisController
	RepoServer RepoServerController
}

var (
	resourceName          string
	metricsResourceName   string
	managedNsResourceName string
	clusterResourceName   string
	component             string
)

func (acr *AppControllerReconciler) Reconcile() error {
	acr.varSetter()

	if err := acr.reconcileServiceAccount(); err != nil {
		acr.Logger.Error(err, "failed to reconcile service account")
		return err
	}

	if acr.ClusterScoped {
		if err := acr.reconcileClusterRole(); err != nil {
			acr.Logger.Error(err, "failed to reconcile clusterRole")
			return err
		}

		if err := acr.reconcileClusterRoleBinding(); err != nil {
			acr.Logger.Error(err, "failed to reconcile clusterRolebinding")
			return err
		}

	} else {
		// delete cluster RBAC
		if err := acr.deleteClusterRoleBinding(clusterResourceName); err != nil {
			acr.Logger.Error(err, "failed to delete cluster role")
		}

		if err := acr.deleteClusterRole(clusterResourceName); err != nil {
			acr.Logger.Error(err, "failed to delete cluster role")
		}

	}

	if err := acr.reconcileRoles(); err != nil {
		acr.Logger.Error(err, "failed to reconcile one or more roles")
	}

	if err := acr.reconcileRoleBindings(); err != nil {
		acr.Logger.Error(err, "failed to reconcile one or more rolebindings")
	}

	if err := acr.reconcileMetricsService(); err != nil {
		acr.Logger.Error(err, "failed to reconcile metrics service")
	}

	if monitoring.IsPrometheusAPIAvailable() {
		if acr.Instance.Spec.Prometheus.Enabled {
			if err := acr.reconcileMetricsServiceMonitor(); err != nil {
				acr.Logger.Error(err, "failed to reconcile metrics service monitor")
			}
		} else {
			if err := acr.deleteServiceMonitor(metricsResourceName, acr.Instance.Namespace); err != nil {
				acr.Logger.Error(err, "failed to delete serviceMonitor")
			}
		}
	} else {
		acr.Logger.Debug("prometheus API unavailable, skipping service monitor reconciliation")
	}

	if err := acr.reconcileStatefulSet(); err != nil {
		acr.Logger.Error(err, "failed to reconcile statefulset")
	}

	return nil
}

func (acr *AppControllerReconciler) DeleteResources() error {
	var deletionErr util.MultiError

	if err := acr.deleteStatefulSet(resourceName, acr.Instance.Namespace); err != nil {
		acr.Logger.Error(err, "failed to delete statefulset")
		deletionErr.Append(err)
	}

	if err := acr.deleteService(metricsResourceName, acr.Instance.Namespace); err != nil {
		acr.Logger.Error(err, "failed to delete service", "name", metricsResourceName)
		deletionErr.Append(err)
	}

	if err := acr.deleteClusterRole(clusterResourceName); err != nil {
		acr.Logger.Error(err, "failed to delete cluster role")
	}

	if err := acr.deleteClusterRoleBinding(clusterResourceName); err != nil {
		acr.Logger.Error(err, "failed to delete cluster role")
	}

	if err := acr.deleteRoleBinding(resourceName, acr.Instance.Namespace); err != nil {
		acr.Logger.Error(err, "failed to delete rolebinding")
	}

	if err := acr.deleteRole(resourceName, acr.Instance.Namespace); err != nil {
		acr.Logger.Error(err, "failed to delete role")
	}

	roles, rbs, err := acr.getManagedRBACToBeDeleted()
	if err != nil {
		acr.Logger.Error(err, "failed to retrieve one or more namespaced rbac resources to be deleted")
	} else {
		deletionErr.Append(acr.DeleteRoleBindings(rbs))
		deletionErr.Append(acr.DeleteRoles(roles))
		if !deletionErr.IsNil() {
			acr.Logger.Error(deletionErr, "failed to delete one or more managed namespaced rbac resources")
		}
	}

	if err := acr.deleteServiceAccount(resourceName, acr.Instance.Namespace); err != nil {
		acr.Logger.Error(err, "failed to delete serviceaccount")
	}

	return deletionErr.ErrOrNil()
}

func (acr *AppControllerReconciler) TriggerRollout(key string) error {
	return acr.TriggerStatefulSetRollout(resourceName, acr.Instance.Namespace, key)
}

func (acr *AppControllerReconciler) varSetter() {
	component = common.AppControllerComponent
	resourceName = argoutil.GenerateResourceName(acr.Instance.Name, common.AppControllerSuffix)
	clusterResourceName = argoutil.GenerateUniqueResourceName(acr.Instance.Name, acr.Instance.Namespace, common.AppControllerSuffix)

	metricsResourceName = argoutil.NameWithSuffix(resourceName, common.MetricsSuffix)
	managedNsResourceName = argoutil.NameWithSuffix(clusterResourceName, common.ResourceMgmtSuffix)
}
