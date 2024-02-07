package applicationset

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/networking"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (asr *ApplicationSetReconciler) reconcileService() error {

	asr.Logger.Info("reconciling services")

	serviceRequest := networking.ServiceRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        resourceName,
			Namespace:   asr.Instance.Namespace,
			Labels:      resourceLabels,
			Annotations: asr.Instance.Annotations,
		},
		Spec:      GetServiceSpec(),
		Client:    asr.Client,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	desiredService, err := networking.RequestService(serviceRequest)
	if err != nil {
		asr.Logger.Error(err, "reconcileService: failed to request service", "name", desiredService.Name, "namespace", desiredService.Namespace)
		asr.Logger.Debug("reconcileService: one or more mutations could not be applied")
		return err
	}

	namespace, err := cluster.GetNamespace(asr.Instance.Namespace, asr.Client)
	if err != nil {
		asr.Logger.Error(err, "reconcileService: failed to retrieve namespace", "name", asr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := asr.deleteService(desiredService.Name, desiredService.Namespace); err != nil {
			asr.Logger.Error(err, "reconcileService: failed to delete service", "name", desiredService.Name, "namespace", desiredService.Namespace)
		}
		return err
	}

	_, err = networking.GetService(desiredService.Name, desiredService.Namespace, asr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			asr.Logger.Error(err, "reconcileService: failed to retrieve service", "name", desiredService.Name, "namespace", desiredService.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(asr.Instance, desiredService, asr.Scheme); err != nil {
			asr.Logger.Error(err, "reconcileService: failed to set owner reference for service", "name", desiredService.Name, "namespace", desiredService.Namespace)
		}

		if err = networking.CreateService(desiredService, asr.Client); err != nil {
			asr.Logger.Error(err, "reconcileService: failed to create service", "name", desiredService.Name, "namespace", desiredService.Namespace)
			return err
		}
		asr.Logger.Info("service created", "name", desiredService.Name, "namespace", desiredService.Namespace)
		return nil
	}

	return nil
}

func (asr *ApplicationSetReconciler) deleteService(name, namespace string) error {
	if err := networking.DeleteService(name, namespace, asr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		asr.Logger.Error(err, "DeleteService: failed to delete service", "name", name, "namespace", namespace)
		return err
	}
	asr.Logger.Info("service deleted", "name", name, "namespace", namespace)
	return nil
}

func GetServiceSpec() corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Name:       common.Webhook,
				Port:       7000,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(7000),
			},
			{
				Name:       common.ArgoCDMetrics,
				Port:       8080,
				TargetPort: intstr.FromInt(8080),
			},
		},
		Selector: map[string]string{
			common.AppK8sKeyName: resourceName,
		},
	}
}
