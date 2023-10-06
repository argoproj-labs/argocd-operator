package reposerver

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/networking"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (rsr *RepoServerReconciler) reconcileService() error {

	rsr.Logger.Info("reconciling service")

	serviceRequest := networking.ServiceRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        resourceName,
			Namespace:   rsr.Instance.Namespace,
			Labels:      resourceLabels,
			Annotations: rsr.Instance.Annotations,
		},
		Spec:      GetServiceSpec(),
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Client:    rsr.Client,
	}

	desiredService, err := networking.RequestService(serviceRequest)
	if err != nil {
		rsr.Logger.Error(err, "reconcileService: failed to request service", "name", desiredService.Name, "namespace", desiredService.Namespace)
		return err
	}

	namespace, err := cluster.GetNamespace(rsr.Instance.Namespace, rsr.Client)
	if err != nil {
		rsr.Logger.Error(err, "reconcileService: failed to retrieve namespace", "name", rsr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := rsr.deleteService(desiredService.Name, desiredService.Namespace); err != nil {
			rsr.Logger.Error(err, "reconcileService: failed to delete service", "name", desiredService.Name, "namespace", desiredService.Namespace)
		}
		return err
	}

	existingService, err := networking.GetService(desiredService.Name, desiredService.Namespace, rsr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			rsr.Logger.Error(err, "reconcileService: failed to retrieve service", "name", desiredService.Name, "namespace", desiredService.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(rsr.Instance, desiredService, rsr.Scheme); err != nil {
			rsr.Logger.Error(err, "reconcileService: failed to set owner reference for service", "name", desiredService.Name, "namespace", desiredService.Namespace)
		}

		networking.EnsureAutoTLSAnnotation(existingService, common.ArgoCDRepoServerTLSSecretName, rsr.Instance.Spec.Repo.WantsAutoTLS(), rsr.Logger)

		if err = networking.CreateService(desiredService, rsr.Client); err != nil {
			rsr.Logger.Error(err, "reconcileService: failed to create service", "name", desiredService.Name, "namespace", desiredService.Namespace)
			return err
		}
		rsr.Logger.V(0).Info("reconcileService: service created", "name", desiredService.Name, "namespace", desiredService.Namespace)
		return nil
	}

	if networking.EnsureAutoTLSAnnotation(existingService, common.ArgoCDRepoServerTLSSecretName, rsr.Instance.Spec.Repo.WantsAutoTLS(), rsr.Logger) {
		if err = networking.UpdateService(existingService, rsr.Client); err != nil {
			rsr.Logger.Error(err, "reconcileService: failed to update service", "name", existingService.Name, "namespace", existingService.Namespace)
			return err
		}
	}
	rsr.Logger.V(0).Info("reconcileService: service updated", "name", existingService.Name, "namespace", existingService.Namespace)

	return nil
}

func (rsr *RepoServerReconciler) deleteService(name, namespace string) error {
	if err := networking.DeleteService(name, namespace, rsr.Client); err != nil {
		rsr.Logger.Error(err, "DeleteService: failed to delete service", "name", name, "namespace", namespace)
		return err
	}
	rsr.Logger.V(0).Info("DeleteService: service deleted", "name", name, "namespace", namespace)
	return nil
}

func GetServiceSpec() corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Name:       common.Server,
				Port:       common.ArgoCDDefaultRepoServerPort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(common.ArgoCDDefaultRepoServerPort),
			},
			{
				Name:       common.ArgoCDMetrics,
				Port:       common.ArgoCDDefaultRepoMetricsPort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(common.ArgoCDDefaultRepoMetricsPort),
			},
		},
		Selector: map[string]string{
			common.AppK8sKeyName: resourceName,
		},
	}
}
