package server

import (
	"strconv"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/networking"
	"github.com/argoproj-labs/argocd-operator/pkg/openshift"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

// reconcileService will ensure that the Service is present for the Argo CD server.
func (sr *ServerReconciler) reconcileService() error {

	svcReq := networking.ServiceRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, sr.Instance.Namespace, sr.Instance.Name, sr.Instance.Namespace, component),
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(8080),
				}, {
					Name:       "https",
					Port:       443,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(8080),
				},
			},
			Selector: map[string]string{
				common.AppK8sKeyName: resourceName,
			},
			Type: corev1.ServiceTypeClusterIP,
		},
		Client:    sr.Client,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	// override service type if set in ArgoCD CR
	if len(sr.Instance.Spec.Server.Service.Type) > 0 {
		svcReq.Spec.Type = sr.Instance.Spec.Server.Service.Type
	}

	desiredSvc, err := networking.RequestService(svcReq)
	if err != nil {
		return errors.Wrapf(err, "reconcileService: failed to request service %s in namespace %s", desiredSvc.Name, desiredSvc.Namespace)
	}

	// update service annotations if autoTLS is enabled
	openshift.AddAutoTLSAnnotationForOpenShift(
		sr.Instance, desiredSvc, sr.Client, 
		map[string]string{
			common.WantAutoTLSKey: strconv.FormatBool(sr.Instance.Spec.Server.WantsAutoTLS()), 
			common.TLSSecretNameKey: common.ArgoCDServerTLSSecretName,
		},
	)

	if err := controllerutil.SetControllerReference(sr.Instance, desiredSvc, sr.Scheme); err != nil {
		sr.Logger.Error(err, "reconcileService: failed to set owner reference for service", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)
	}

	existingSvc, err := networking.GetService(desiredSvc.Name, desiredSvc.Namespace, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileService: failed to retrieve service %s in namespace %s", desiredSvc.Name, desiredSvc.Namespace)
		}

		if err = networking.CreateService(desiredSvc, sr.Client); err != nil {
			return errors.Wrapf(err, "reconcileService: failed to create service %s in namespace %s", desiredSvc.Name, desiredSvc.Namespace)
		}

		sr.Logger.V(0).Info("service created", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)
		return nil
	}

	// difference in existing & desired ingress, update it
	changed := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{&existingSvc.ObjectMeta.Annotations, &desiredSvc.ObjectMeta.Annotations, nil},
		{&existingSvc.Spec, &desiredSvc.Spec, nil},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &changed)
	}

	// nothing changed, exit reconciliation
	if !changed {
		return nil
	}

	if err = networking.UpdateService(existingSvc, sr.Client); err != nil {
		return errors.Wrapf(err, "reconcileService: failed to update service %s in namespace %s", existingSvc.Name, existingSvc.Namespace)
	}

	sr.Logger.V(0).Info("service updated", "name", existingSvc.Name, "namespace", existingSvc.Namespace)
	return nil
}

// deleteService will delete service with given name.
func (sr *ServerReconciler) deleteService(name, namespace string) error {
	if err := networking.DeleteService(name, namespace, sr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteService: failed to delete service %s in namespace %s", name, namespace)
	}
	sr.Logger.V(0).Info("service deleted", "name", name, "namespace", namespace)
	return nil
}
