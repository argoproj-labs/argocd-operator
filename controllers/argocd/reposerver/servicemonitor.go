package reposerver

import (
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/monitoring"
	"github.com/argoproj-labs/argocd-operator/pkg/util"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (rsr *RepoServerReconciler) reconcileServiceMonitor() error {

	rsr.Logger.Info("reconciling serviceMonitor")

	serviceMonitorRequest := monitoring.ServiceMonitorRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        util.GenerateResourceName(rsr.Instance.Name, RepoServerMetrics),
			Namespace:   rsr.Instance.Namespace,
			Labels:      resourceLabels,
			Annotations: rsr.Instance.Annotations,
		},
	}

	desiredServiceMonitor, err := monitoring.RequestServiceMonitor(serviceMonitorRequest)
	if err != nil {
		rsr.Logger.Error(err, "reconcileServiceMonitor: failed to request serviceMonitor", "name", desiredServiceMonitor.Name, "namespace", desiredServiceMonitor.Namespace)
		return err
	}

	namespace, err := cluster.GetNamespace(rsr.Instance.Namespace, rsr.Client)
	if err != nil {
		rsr.Logger.Error(err, "reconcileServiceMonitor: failed to retrieve namespace", "name", rsr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := rsr.deleteServiceMonitor(desiredServiceMonitor.Name, desiredServiceMonitor.Namespace); err != nil {
			rsr.Logger.Error(err, "reconcileServiceMonitor: failed to delete serviceMonitor", "name", desiredServiceMonitor.Name, "namespace", desiredServiceMonitor.Namespace)
		}
		return err
	}

	_, err = monitoring.GetServiceMonitor(desiredServiceMonitor.Name, desiredServiceMonitor.Namespace, rsr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			rsr.Logger.Error(err, "reconcileServiceMonitor: failed to retrieve serviceMonitor", "name", desiredServiceMonitor.Name, "namespace", desiredServiceMonitor.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(rsr.Instance, desiredServiceMonitor, rsr.Scheme); err != nil {
			rsr.Logger.Error(err, "reconcileServiceMonitor: failed to set owner reference for serviceMonitor", "name", desiredServiceMonitor.Name, "namespace", desiredServiceMonitor.Namespace)
		}

		if err = monitoring.CreateServiceMonitor(desiredServiceMonitor, rsr.Client); err != nil {
			rsr.Logger.Error(err, "reconcileServiceMonitor: failed to create serviceMonitor", "name", desiredServiceMonitor.Name, "namespace", desiredServiceMonitor.Namespace)
			return err
		}
		rsr.Logger.V(0).Info("reconcileServiceMonitor: serviceMonitor created", "name", desiredServiceMonitor.Name, "namespace", desiredServiceMonitor.Namespace)
		return nil
	}

	return nil
}

func (rsr *RepoServerReconciler) deleteServiceMonitor(name, namespace string) error {
	if err := monitoring.DeleteServiceMonitor(name, namespace, rsr.Client); err != nil {
		rsr.Logger.Error(err, "DeleteServiceMonitor: failed to delete serviceMonitor", "name", name, "namespace", namespace)
		return err
	}
	rsr.Logger.V(0).Info("DeleteServiceMonitor: serviceMonitor deleted", "name", name, "namespace", namespace)
	return nil
}
