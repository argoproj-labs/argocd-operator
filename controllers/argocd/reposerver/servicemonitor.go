package reposerver

import (
	"github.com/argoproj-labs/argocd-operator/common"
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
		rsr.Logger.Debug("prometheus API unavailable, skip reconciling service monitor")
		return nil
	}

	smReq := monitoring.ServiceMonitorRequest{
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

	smReq.ObjectMeta.Labels[common.PrometheusReleaseKey] = common.PrometheusOperator

	desiredSM, err := monitoring.RequestServiceMonitor(smReq)
	if err != nil {
		return errors.Wrapf(err, "reconcileServiceMonitor: failed to request service monitor %s", desiredSM.Name)
	}

	if err = controllerutil.SetControllerReference(rsr.Instance, desiredSM, rsr.Scheme); err != nil {
		rsr.Logger.Error(err, "reconcileServiceMonitor: failed to set owner reference for service monitor", "name", desiredSM.Name)
	}

	_, err = monitoring.GetServiceMonitor(desiredSM.Name, desiredSM.Namespace, rsr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileServiceMonitor: failed to retrieve service %s", desiredSM.Name)
		}

		if err = monitoring.CreateServiceMonitor(desiredSM, rsr.Client); err != nil {
			return errors.Wrapf(err, "reconcileServiceMonitor: failed to create service monitor %s", desiredSM.Name)
		}
		rsr.Logger.Info("service monitor created", "name", desiredSM.Name, "namespace", desiredSM.Namespace)
		return nil
	}

	return nil
}

func (rsr *RepoServerReconciler) deleteServiceMonitor(name, namespace string) error {
	// return if prometheus API is not present on cluster
	if !monitoring.IsPrometheusAPIAvailable() {
		rsr.Logger.Debug("prometheus API unavailable, skip deleting service monitor")
		return nil
	}

	if err := monitoring.DeleteServiceMonitor(name, namespace, rsr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		rsr.Logger.Error(err, "DeleteServiceMonitor: failed to delete servicemonitor", "name", name, "namespace", namespace)
		return err
	}
	rsr.Logger.Info("service monitor deleted", "name", name, "namespace", namespace)
	return nil
}
