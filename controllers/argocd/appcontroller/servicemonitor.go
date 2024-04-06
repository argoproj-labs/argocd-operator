package appcontroller

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

func (acr *AppControllerReconciler) reconcileMetricsServiceMonitor() error {
	req := monitoring.ServiceMonitorRequest{
		ObjectMeta: argoutil.GetObjMeta(metricsResourceName, acr.Instance.Namespace, acr.Instance.Name, acr.Instance.Namespace, component, argocdcommon.GetSvcMonitorLabel(), util.EmptyMap()),
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
		Client:    acr.Client,
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
	return acr.reconServiceMonitor(req, argocdcommon.UpdateFnSM(updateFn), ignoreDrift)
}

func (acr *AppControllerReconciler) reconServiceMonitor(req monitoring.ServiceMonitorRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := monitoring.RequestServiceMonitor(req)
	if err != nil {
		acr.Logger.Debug("reconcileServiceMonitor: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconcileServiceMonitor: failed to request ServiceMonitor %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(acr.Instance, desired, acr.Scheme); err != nil {
		acr.Logger.Error(err, "reconcileServiceMonitor: failed to set owner reference for ServiceMonitor", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := monitoring.GetServiceMonitor(desired.Name, desired.Namespace, acr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileServiceMonitor: failed to retrieve ServiceMonitor %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = monitoring.CreateServiceMonitor(desired, acr.Client); err != nil {
			return errors.Wrapf(err, "reconcileServiceMonitor: failed to create ServiceMonitor %s in namespace %s", desired.Name, desired.Namespace)
		}
		acr.Logger.Info("ServiceMonitor created", "name", desired.Name, "namespace", desired.Namespace)
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

	if err = monitoring.UpdateServiceMonitor(existing, acr.Client); err != nil {
		return errors.Wrapf(err, "reconcileServiceMonitor: failed to update ServiceMonitor %s", existing.Name)
	}

	acr.Logger.Info("ServiceMonitor updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

func (acr *AppControllerReconciler) deleteServiceMonitor(name, namespace string) error {
	// return if prometheus API is not present on cluster
	if !monitoring.IsPrometheusAPIAvailable() {
		acr.Logger.Debug("prometheus API unavailable, skip service monitor deletion")
		return nil
	}

	if err := monitoring.DeleteServiceMonitor(name, namespace, acr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteServiceMonitor: failed to delete service monitor %s in namespace %s", name, namespace)
	}
	acr.Logger.Info("service monitor deleted", "name", name, "namespace", namespace)
	return nil
}
