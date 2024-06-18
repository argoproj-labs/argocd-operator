package notifications

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

func (nr *NotificationsReconciler) reconcileMetricsService() error {
	req := networking.ServiceRequest{
		ObjectMeta: argoutil.GetObjMeta(metricsResourceName, nr.Instance.Namespace, nr.Instance.Name, nr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "metrics",
					Port:       common.NotificationsControllerMetricsPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(common.NotificationsControllerMetricsPort),
				},
			},
			Selector: map[string]string{
				common.AppK8sKeyName: resourceName,
			},
		},
		Instance:  nr.Instance,
		Client:    nr.Client,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
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
	return nr.reconService(req, argocdcommon.UpdateFnSvc(updateFn), ignoreDrift)
}

func (nr *NotificationsReconciler) reconService(req networking.ServiceRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := networking.RequestService(req)
	if err != nil {
		nr.Logger.Debug("reconcileService: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconcileService: failed to request Service %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(nr.Instance, desired, nr.Scheme); err != nil {
		nr.Logger.Error(err, "reconcileService: failed to set owner reference for Service", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := networking.GetService(desired.Name, desired.Namespace, nr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileService: failed to retrieve Service %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = networking.CreateService(desired, nr.Client); err != nil {
			return errors.Wrapf(err, "reconcileService: failed to create Service %s in namespace %s", desired.Name, desired.Namespace)
		}
		nr.Logger.Info("service created", "name", desired.Name, "namespace", desired.Namespace)
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

	if err = networking.UpdateService(existing, nr.Client); err != nil {
		return errors.Wrapf(err, "reconcileService: failed to update service %s", existing.Name)
	}

	nr.Logger.Info("service updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteService will delete service with given name.
func (nr *NotificationsReconciler) deleteService(name, namespace string) error {
	if err := networking.DeleteService(name, namespace, nr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteService: failed to delete service %s in namespace %s", name, namespace)
	}
	nr.Logger.Info("service deleted", "name", name, "namespace", namespace)
	return nil
}
