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
	req := networking.ServiceRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, rsr.Instance.Namespace, rsr.Instance.Name, rsr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
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

	desired, err := networking.RequestService(req)
	if err != nil {
		return errors.Wrapf(err, "reconcileService: failed to request service %s", desired.Name)
	}

	if err = controllerutil.SetControllerReference(rsr.Instance, desired, rsr.Scheme); err != nil {
		rsr.Logger.Error(err, "reconcileService: failed to set owner reference for service", "name", desired.Name)
	}

	existing, err := networking.GetService(desired.Name, desired.Namespace, rsr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileService: failed to retrieve service %s", desired.Name)
		}

		if err = networking.CreateService(desired, rsr.Client); err != nil {
			return errors.Wrapf(err, "reconcileService: failed to create service %s", desired.Name)
		}
		rsr.Logger.Info("service created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	changed := false

	fieldsToCompare := []argocdcommon.FieldToCompare{
		{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
		{Existing: &existing.Spec.Ports, Desired: &desired.Spec.Ports, ExtraAction: nil},
	}

	argocdcommon.UpdateIfChanged(fieldsToCompare, &changed)

	if !changed {
		return nil
	}

	if err = networking.UpdateService(existing, rsr.Client); err != nil {
		return errors.Wrapf(err, "reconcileService: failed to update service %s", existing.Name)
	}

	rsr.Logger.Info("service updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

func (rsr *RepoServerReconciler) deleteService(name, namespace string) error {
	if err := networking.DeleteService(name, namespace, rsr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteService: failed to delete service %s in namespace %s", name, namespace)
	}
	rsr.Logger.Info("service deleted", "name", name, "namespace", namespace)
	return nil
}
