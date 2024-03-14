package appcontroller

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/networking"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (acr *AppControllerReconciler) reconcileMetricsService() error {
	req := networking.ServiceRequest{
		ObjectMeta: argoutil.GetObjMeta(metricsResourceName, acr.Instance.Namespace, acr.Instance.Name, acr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				common.AppK8sKeyName: resourceName,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "metrics",
					Port:       common.AppControllerMetricsPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(common.AppControllerMetricsPort),
				},
			},
		},
		Instance:  acr.Instance,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Client:    acr.Client,
	}

	ignoreDrift := false
	updateFn := func(existing, desired *corev1.Service, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			{Existing: &existing.Spec.Selector, Desired: &desired.Spec.Selector, ExtraAction: nil},
			{Existing: &existing.Spec.Ports, Desired: &desired.Spec.Ports, ExtraAction: nil},
		}
		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}
	return acr.reconService(req, argocdcommon.UpdateFnSvc(updateFn), ignoreDrift)
}

func (acr *AppControllerReconciler) reconService(req networking.ServiceRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := networking.RequestService(req)
	if err != nil {
		acr.Logger.Debug("reconcileService: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconcileService: failed to request Service %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(acr.Instance, desired, acr.Scheme); err != nil {
		acr.Logger.Error(err, "reconcileService: failed to set owner reference for Service", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := networking.GetService(desired.Name, desired.Namespace, acr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileService: failed to retrieve Service %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = networking.CreateService(desired, acr.Client); err != nil {
			return errors.Wrapf(err, "reconcileService: failed to create Service %s in namespace %s", desired.Name, desired.Namespace)
		}
		acr.Logger.Info("service created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// Service found, no update required - nothing to do
	if ignoreDrift {
		return nil
	}

	changed := false

	// execute supplied update function
	if updateFn != nil {
		if fn, ok := updateFn.(argocdcommon.UpdateFnSvc); ok {
			if err := fn(existing, desired, &changed); err != nil {
				return errors.Wrapf(err, "reconcileService: failed to execute update function for %s in namespace %s", existing.Name, existing.Namespace)
			}
		}
	}

	if !changed {
		return nil
	}

	if err = networking.UpdateService(existing, acr.Client); err != nil {
		return errors.Wrapf(err, "reconcileService: failed to update service %s", existing.Name)
	}

	acr.Logger.Info("service updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

func (acr *AppControllerReconciler) deleteService(name, namespace string) error {
	if err := networking.DeleteService(name, namespace, acr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteService: failed to delete service %s", name)
	}
	acr.Logger.Info("service deleted", "name", name, "namespace", namespace)
	return nil
}
