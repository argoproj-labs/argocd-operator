package reposerver

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/monitoring"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (rsr *RepoServerReconciler) reconcileServiceMonitor() error {

	// return if prometheus API is not present on cluster
	if !monitoring.IsPrometheusAPIAvailable() {
		rsr.Logger.Debug("prometheus API unavailable, skip service monitor reconciliation")
		return nil
	}

	req := monitoring.ServiceMonitorRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceMetricsName, rsr.Instance.Namespace, rsr.Instance.Name, rsr.Instance.Namespace, component),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					common.AppK8sKeyName: resourceName,
				},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port: common.ArgoCDMetrics,
				},
			},
		},
	}

	req.ObjectMeta.Labels[common.PrometheusReleaseKey] = common.PrometheusOperator

	desired, err := monitoring.RequestServiceMonitor(req)
	if err != nil {
		return errors.Wrapf(err, "reconcileServiceMonitor: failed to request service monitor %s", desired.Name)
	}

	if err = controllerutil.SetControllerReference(rsr.Instance, desired, rsr.Scheme); err != nil {
		rsr.Logger.Error(err, "reconcileServiceMonitor: failed to set owner reference for service monitor", "name", desired.Name)
	}

	existing, err := monitoring.GetServiceMonitor(desired.Name, desired.Namespace, rsr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileServiceMonitor: failed to retrieve service monitor %s", desired.Name)
		}

		if err = monitoring.CreateServiceMonitor(desired, rsr.Client); err != nil {
			return errors.Wrapf(err, "reconcileServiceMonitor: failed to create service monitor %s", desired.Name)
		}
		rsr.Logger.Info("service monitor created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	changed := false

	fieldsToCompare := []argocdcommon.FieldToCompare{
		{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
		{Existing: &existing.Annotations, Desired: &desired.Annotations, ExtraAction: nil},
		{Existing: &existing.Spec, Desired: &desired.Spec, ExtraAction: nil},
	}

	argocdcommon.UpdateIfChanged(fieldsToCompare, &changed)

	if !changed {
		return nil
	}

	if err = monitoring.UpdateServiceMonitor(existing, rsr.Client); err != nil {
		return errors.Wrapf(err, "reconcileServiceMonitor: failed to update service monitor %s", existing.Name)
	}

	rsr.Logger.Info("service monitor updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

func (rsr *RepoServerReconciler) deleteServiceMonitor(name, namespace string) error {
	// return if prometheus API is not present on cluster
	if !monitoring.IsPrometheusAPIAvailable() {
		rsr.Logger.Debug("prometheus API unavailable, skip service monitor deletion")
		return nil
	}

	if err := monitoring.DeleteServiceMonitor(name, namespace, rsr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteServiceMonitor: failed to delete service monitor %s in namespace %s", name, namespace)
	}
	rsr.Logger.Info("service monitor deleted", "name", name, "namespace", namespace)
	return nil
}
