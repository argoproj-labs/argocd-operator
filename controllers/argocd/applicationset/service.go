package applicationset

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

func (asr *ApplicationSetReconciler) reconcileService() error {

	req := networking.ServiceRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, asr.Instance.Namespace, asr.Instance.Name, asr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				common.AppK8sKeyName: resourceName,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       common.Webhook,
					Port:       common.AppSetWebhookPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(common.AppSetWebhookPort),
				},
				{
					Name:       common.ArgoCDMetrics,
					Port:       common.AppSetMetricsPort,
					TargetPort: intstr.FromInt(common.AppSetMetricsPort),
				},
			},
		},
		Client:    asr.Client,
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
	return asr.reconService(req, argocdcommon.UpdateFnSvc(updateFn), ignoreDrift)
}

func (asr *ApplicationSetReconciler) reconService(req networking.ServiceRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := networking.RequestService(req)
	if err != nil {
		asr.Logger.Debug("reconcileService: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconcileService: failed to request Service %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(asr.Instance, desired, asr.Scheme); err != nil {
		asr.Logger.Error(err, "reconcileService: failed to set owner reference for Service", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := networking.GetService(desired.Name, desired.Namespace, asr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileService: failed to retrieve Service %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = networking.CreateService(desired, asr.Client); err != nil {
			return errors.Wrapf(err, "reconcileService: failed to create Service %s in namespace %s", desired.Name, desired.Namespace)
		}
		asr.Logger.Info("service created", "name", desired.Name, "namespace", desired.Namespace)
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

	if err = networking.UpdateService(existing, asr.Client); err != nil {
		return errors.Wrapf(err, "reconcileService: failed to update service %s", existing.Name)
	}

	asr.Logger.Info("service updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteService will delete service with given name.
func (asr *ApplicationSetReconciler) deleteService(name, namespace string) error {
	if err := networking.DeleteService(name, namespace, asr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteService: failed to delete service %s in namespace %s", name, namespace)
	}
	asr.Logger.Info("service deleted", "name", name, "namespace", namespace)
	return nil
}
