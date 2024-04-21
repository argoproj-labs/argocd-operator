package notifications

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/monitoring"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

type NotificationsReconciler struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Instance *argoproj.ArgoCD
	Logger   *util.Logger
}

var (
	resourceName        string
	metricsResourceName string
	component           string
)

func (nr *NotificationsReconciler) Reconcile() error {
	nr.varSetter()

	if err := nr.reconcileConfigurationCR(); err != nil {
		nr.Logger.Error(err, "failed to reconcile configuration instance")
		return err
	}

	if err := nr.reconcileServiceAccount(); err != nil {
		nr.Logger.Error(err, "failed to reconcile service account")
		return err
	}

	if err := nr.reconcileRole(); err != nil {
		nr.Logger.Error(err, "failed to reconcile role")
		return err
	}

	if err := nr.reconcileRoleBinding(); err != nil {
		nr.Logger.Error(err, "failed to reconcile rolebinding")
		return err
	}

	if err := nr.reconcileSecret(); err != nil {
		nr.Logger.Error(err, "failed to reconcile secret")
		return err
	}

	if err := nr.reconcileDeployment(); err != nil {
		nr.Logger.Error(err, "failed to reconcile deployment")
		return err
	}

	if err := nr.reconcileMetricsService(); err != nil {
		nr.Logger.Error(err, "failed to reconcile metrics service")
	}

	if monitoring.IsPrometheusAPIAvailable() {
		if nr.Instance.Spec.Prometheus.Enabled {
			if err := nr.reconcileMetricsServiceMonitor(); err != nil {
				nr.Logger.Error(err, "failed to reconcile metrics service monitor")
			}
		} else {
			if err := nr.deleteServiceMonitor(metricsResourceName, nr.Instance.Namespace); err != nil {
				nr.Logger.Error(err, "failed to delete serviceMonitor")
			}
		}
	} else {
		nr.Logger.Debug("prometheus API unavailable, skipping service monitor reconciliation")
	}

	return nil
}

func (nr *NotificationsReconciler) DeleteResources() error {
	var deletionError util.MultiError

	if err := nr.deleteConfigurationCR(); err != nil {
		nr.Logger.Error(err, "DeleteResources: failed to delete configuration CR")
		deletionError.Append(err)
	}

	if err := nr.deleteDeployment(resourceName, nr.Instance.Namespace); err != nil {
		nr.Logger.Error(err, "DeleteResources: failed to delete deployment")
		deletionError.Append(err)
	}

	if err := nr.deleteSecret(nr.Instance.Namespace); err != nil {
		nr.Logger.Error(err, "DeleteResources: failed to delete secret")
		deletionError.Append(err)
	}

	if err := nr.deleteRoleBinding(resourceName, nr.Instance.Namespace); err != nil {
		nr.Logger.Error(err, "DeleteResources: failed to delete roleBinding")
		deletionError.Append(err)
	}

	if err := nr.deleteRole(resourceName, nr.Instance.Namespace); err != nil {
		nr.Logger.Error(err, "DeleteResources: failed to delete role")
		deletionError.Append(err)
	}

	if err := nr.deleteServiceAccount(resourceName, nr.Instance.Namespace); err != nil {
		nr.Logger.Error(err, "DeleteResources: failed to delete serviceaccount")
		deletionError.Append(err)
	}

	return deletionError.ErrOrNil()
}

func (nr *NotificationsReconciler) varSetter() {
	component = common.NotificationsControllerComponent
	resourceName = argoutil.GenerateResourceName(nr.Instance.Name, common.NotificationsControllerSuffix)

	metricsResourceName = argoutil.NameWithSuffix(resourceName, common.MetricsSuffix)
}
