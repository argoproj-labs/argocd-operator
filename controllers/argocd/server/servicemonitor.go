package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/monitoring"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (sr *ServerReconciler) reconcileMetricsServiceMonitor() error {
	req := monitoring.ServiceMonitorRequest{
		ObjectMeta: argoutil.GetObjMeta(metricsResourceName, sr.Instance.Namespace, sr.Instance.Name, sr.Instance.Namespace, component, argocdcommon.GetSvcMonitorLabel(), util.EmptyMap()),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					common.AppK8sKeyName: metricsResourceName,
				},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port: common.ArgoCDMetrics,
				},
			},
		},
		Client:    sr.Client,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	ignoreDrift := false
	updateFn := func(existing, desired *monitoringv1.ServiceMonitor, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			{Existing: &existing.Spec, Desired: &desired.Spec, ExtraAction: nil},
		}
		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}
	return sr.reconServiceMonitor(req, argocdcommon.UpdateFnSM(updateFn), ignoreDrift)
}

func (sr *ServerReconciler) reconServiceMonitor(req monitoring.ServiceMonitorRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := monitoring.RequestServiceMonitor(req)
	if err != nil {
		sr.Logger.Debug("reconcileServiceMonitor: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconcileServiceMonitor: failed to request ServiceMonitor %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(sr.Instance, desired, sr.Scheme); err != nil {
		sr.Logger.Error(err, "reconcileServiceMonitor: failed to set owner reference for ServiceMonitor", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := monitoring.GetServiceMonitor(desired.Name, desired.Namespace, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileServiceMonitor: failed to retrieve ServiceMonitor %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = monitoring.CreateServiceMonitor(desired, sr.Client); err != nil {
			return errors.Wrapf(err, "reconcileServiceMonitor: failed to create ServiceMonitor %s in namespace %s", desired.Name, desired.Namespace)
		}
		sr.Logger.Info("ServiceMonitor created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// ServiceMonitor found, no update required - nothing to do
	if ignoreDrift {
		return nil
	}

	changed := false

	// execute supplied update function
	if updateFn != nil {
		if fn, ok := updateFn.(argocdcommon.UpdateFnSM); ok {
			if err := fn(existing, desired, &changed); err != nil {
				return errors.Wrapf(err, "reconcileServiceMonitor: failed to execute update function for %s in namespace %s", existing.Name, existing.Namespace)
			}
		}
	}

	if !changed {
		return nil
	}

	if err = monitoring.UpdateServiceMonitor(existing, sr.Client); err != nil {
		return errors.Wrapf(err, "reconcileServiceMonitor: failed to update ServiceMonitor %s", existing.Name)
	}

	sr.Logger.Info("ServiceMonitor updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

func (sr *ServerReconciler) deleteServiceMonitor(name, namespace string) error {
	// return if prometheus API is not present on cluster
	if !monitoring.IsPrometheusAPIAvailable() {
		sr.Logger.Debug("prometheus API unavailable, skip service monitor deletion")
		return nil
	}

	if err := monitoring.DeleteServiceMonitor(name, namespace, sr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteServiceMonitor: failed to delete service monitor %s in namespace %s", name, namespace)
	}
	sr.Logger.Info("service monitor deleted", "name", name, "namespace", namespace)
	return nil
}
