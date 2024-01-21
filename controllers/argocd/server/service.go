package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/networking"
	"github.com/argoproj-labs/argocd-operator/pkg/openshift"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1 "k8s.io/api/core/v1"
)

// reconcileService will ensure that the Service is present for the Argo CD server.
func (sr *ServerReconciler) reconcileService() error {

	sr.Logger.Info("reconciling services")

	svcName := getServiceName(sr.Instance.Name)
	svcLabels := common.DefaultResourceLabels(svcName, sr.Instance.Name, ServerControllerComponent)

	// the server Service type for the ArgoCD
	svcType := corev1.ServiceTypeClusterIP
	if len(sr.Instance.Spec.Server.Service.Type) > 0 {
		svcType = sr.Instance.Spec.Server.Service.Type
	}

	svcRequest := networking.ServiceRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svcName,
			Namespace:   sr.Instance.Namespace,
			Labels:      svcLabels,
			Annotations: sr.Instance.Annotations,
		},
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
				common.AppK8sKeyName: getDeploymentName(sr.Instance.Name),
			},
			Type: svcType,
		},
		Client:    sr.Client,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	desiredSvc, err := networking.RequestService(svcRequest)
	if err != nil {
		sr.Logger.Error(err, "reconcileService: failed to request service", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)
		sr.Logger.V(1).Info("reconcileService: one or more mutations could not be applied")
		return err
	}

	// update service annotations if auto TLS is enabled
	openshift.EnsureAutoTLSAnnotation(desiredSvc, common.ArgoCDServerTLSSecretName, sr.Instance.Spec.Server.WantsAutoTLS())

	existingSvc, err := networking.GetService(desiredSvc.Name, desiredSvc.Namespace, sr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			sr.Logger.Error(err, "reconcileService: failed to retrieve service", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(sr.Instance, desiredSvc, sr.Scheme); err != nil {
			sr.Logger.Error(err, "reconcileService: failed to set owner reference for service", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)
		}

		if err = networking.CreateService(desiredSvc, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileService: failed to create service", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)
			return err
		}
		sr.Logger.V(0).Info("reconcileService: service created", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)
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

	if changed {
		if err = networking.UpdateService(existingSvc, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileService: failed to update service", "name", existingSvc.Name, "namespace", existingSvc.Namespace)
			return err
		}
		sr.Logger.V(0).Info("reconcileService: service updated", "name", existingSvc.Name, "namespace", existingSvc.Namespace)
	}

	// service found, no changes detected
	return nil
}

// deleteService will delete service with given name.
func (sr *ServerReconciler) deleteService(name, namespace string) error {
	if err := networking.DeleteService(name, namespace, sr.Client); err != nil {
		sr.Logger.Error(err, "deleteService: failed to delete service", "name", name, "namespace", namespace)
		return err
	}
	sr.Logger.V(0).Info("deleteService: service deleted", "name", name, "namespace", namespace)
	return nil
}
