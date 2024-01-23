package reposerver

import (
	"strconv"

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

func (rsr *RepoServerReconciler) reconcileService() error {

	rsr.Logger.Info("reconciling service")

	svcRequest := networking.ServiceRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, rsr.Instance.Namespace, rsr.Instance.Name, rsr.Instance.Namespace, component),
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "server",
					Port:       common.DefaultRepoServerPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(common.DefaultRepoServerPort),
				},
				{
					Name:       common.ArgoCDMetrics,
					Port:       common.DefaultRepoServerMetricsPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(common.DefaultRepoServerMetricsPort),
				},
			},
			Selector: map[string]string{
				common.AppK8sKeyName: resourceName,
			},
		},
		Instance:  rsr.Instance,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		MutationArgs: util.ConvertStringMapToInterfaces(map[string]string{
			common.TLSSecretNameKey: common.ArgoCDRepoServerTLSSecretName,
			common.WantAutoTLSKey:   strconv.FormatBool(rsr.Instance.Spec.Repo.WantsAutoTLS()),
		}),
		Client: rsr.Client,
	}

	desiredSvc, err := networking.RequestService(svcRequest)
	if err != nil {
		return errors.Wrapf(err, "reconcileService: failed to request service %s", desiredSvc.Name)
	}

	if err = controllerutil.SetControllerReference(rsr.Instance, desiredSvc, rsr.Scheme); err != nil {
		rsr.Logger.Error(err, "reconcileService: failed to set owner reference for service", "name", desiredSvc.Name)
	}

	existingSvc, err := networking.GetService(desiredSvc.Name, desiredSvc.Namespace, rsr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileService: failed to retrieve service %s", desiredSvc.Name)
		}

		if err = networking.CreateService(desiredSvc, rsr.Client); err != nil {
			return errors.Wrapf(err, "reconcileService: failed to create service %s", desiredSvc.Name)
		}
		rsr.Logger.V(0).Info("service created", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)
		return nil
	}

	svcChanged := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{&existingSvc.Annotations, &desiredSvc.Annotations, nil},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &svcChanged)
	}

	if !svcChanged {
		return nil
	}

	if err = networking.UpdateService(existingSvc, rsr.Client); err != nil {
		return errors.Wrapf(err, "reconcileService: failed to update service %s", existingSvc.Name)
	}

	rsr.Logger.V(0).Info("service updated", "name", existingSvc.Name, "namespace", existingSvc.Namespace)
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
