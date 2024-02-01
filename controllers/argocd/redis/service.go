package redis

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

func (rr *RedisReconciler) reconcileService() error {
	req := networking.ServiceRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component),
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				common.AppK8sKeyName: resourceName,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "tcp-redis",
					Port:       common.DefaultRedisPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(common.DefaultRedisPort),
				},
			},
		},
		Instance:  rr.Instance,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		MutationArgs: util.ConvertStringMapToInterfaces(map[string]string{
			common.TLSSecretNameKey: common.ArgoCDRedisServerTLSSecretName,
			common.WantAutoTLSKey:   strconv.FormatBool(rr.Instance.Spec.Redis.WantsAutoTLS()),
		}),
		Client: rr.Client,
	}

	desired, err := networking.RequestService(req)
	if err != nil {
		return errors.Wrapf(err, "reconcileService: failed to request service %s", desired.Name)
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desired, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileService: failed to set owner reference for service", "name", desired.Name)
	}

	existing, err := networking.GetService(desired.Name, desired.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileService: failed to retrieve service %s", desired.Name)
		}

		if err = networking.CreateService(desired, rr.Client); err != nil {
			return errors.Wrapf(err, "reconcileService: failed to create service %s", desired.Name)
		}
		rr.Logger.Info("service created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	changed := false

	fieldsToCompare := []argocdcommon.FieldToCompare{
		{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
		{Existing: &existing.Annotations, Desired: &desired.Annotations, ExtraAction: nil},
		{Existing: &existing.Spec, Desired: &desired.Spec, ExtraAction: nil},
	}

	argocdcommon.UpdateIfChanged(fieldsToCompare, &changed)

	if !changed {
		return nil
	}

	if err = networking.UpdateService(existing, rr.Client); err != nil {
		return errors.Wrapf(err, "reconcileService: failed to update service %s", existing.Name)
	}

	rr.Logger.Info("service updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// reconcileHAProxyService will ensure that the HA Proxy Service is present for Redis when running in HA mode.
func (rr *RedisReconciler) reconcileHAProxyService() error {
	req := networking.ServiceRequest{
		ObjectMeta: argoutil.GetObjMeta(HAProxyResourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component),
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				common.AppK8sKeyName: HAProxyResourceName,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "haproxy",
					Port:       common.DefaultRedisPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromString("redis"),
				},
			},
		},
		Instance:  rr.Instance,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		MutationArgs: util.ConvertStringMapToInterfaces(map[string]string{
			common.TLSSecretNameKey: common.ArgoCDRedisServerTLSSecretName,
		}),
		Client: rr.Client,
	}

	desired, err := networking.RequestService(req)
	if err != nil {
		return errors.Wrapf(err, "reconcileHAProxyService: failed to request service %s", desired.Name)
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desired, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileHAProxyService: failed to set owner reference for service", "name", desired.Name)
	}

	existing, err := networking.GetService(desired.Name, desired.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileHAProxyService: failed to retrieve service %s", desired.Name)
		}

		if err = networking.CreateService(desired, rr.Client); err != nil {
			return errors.Wrapf(err, "reconcileHAProxyService: failed to create service %s", desired.Name)
		}
		rr.Logger.Info("service created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	changed := false

	fieldsToCompare := []argocdcommon.FieldToCompare{
		{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
		{Existing: &existing.Annotations, Desired: &desired.Annotations, ExtraAction: nil},
		{Existing: &existing.Spec, Desired: &desired.Spec, ExtraAction: nil},
	}

	argocdcommon.UpdateIfChanged(fieldsToCompare, &changed)

	if !changed {
		return nil
	}

	if err = networking.UpdateService(existing, rr.Client); err != nil {
		return errors.Wrapf(err, "reconcileHAProxyService: failed to update service %s", existing.Name)
	}

	rr.Logger.Info("service updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

func (rr *RedisReconciler) reconcileHAMasterService() error {
	svcRequest := networking.ServiceRequest{
		ObjectMeta: argoutil.GetObjMeta(HAResourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component),
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				common.AppK8sKeyName: HAResourceName,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "server",
					Port:       common.DefaultRedisPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromString("redis"),
				}, {
					Name:       "sentinel",
					Port:       common.DefaultRedisSentinelPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromString("sentinel"),
				},
			},
		},
	}

	desired, err := networking.RequestService(svcRequest)
	if err != nil {
		return errors.Wrapf(err, "failed to request service %s", desired.Name)
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desired, rr.Scheme); err != nil {
		rr.Logger.Error(err, "failed to set owner reference for service", "name", desired.Name)
	}

	existing, err := networking.GetService(desired.Name, desired.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to retrieve service %s", desired.Name)
		}

		if err = networking.CreateService(desired, rr.Client); err != nil {
			return errors.Wrapf(err, "failed to create service %s", desired.Name)
		}
		rr.Logger.Info("service created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	changed := false

	fieldsToCompare := []argocdcommon.FieldToCompare{
		{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
		{Existing: &existing.Annotations, Desired: &desired.Annotations, ExtraAction: nil},
		{Existing: &existing.Spec, Desired: &desired.Spec, ExtraAction: nil},
	}

	argocdcommon.UpdateIfChanged(fieldsToCompare, &changed)

	if !changed {
		return nil
	}

	// nothing to do
	return nil
}

func (rr *RedisReconciler) reconcileHAAnnounceServices() error {
	var reconcileErrs util.MultiError

	for i := int32(0); i < common.DefaultRedisHAReplicas; i++ {
		name := argoutil.GenerateResourceName(rr.Instance.Name, common.RedisHAAnnouceSuffix, string(i))

		req := networking.ServiceRequest{
			ObjectMeta: argoutil.GetObjMeta(name, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component),
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					common.AppK8sKeyName:            HAResourceName,
					common.StatefulSetK8sKeyPodName: name,
				},
				PublishNotReadyAddresses: true,
				Ports: []corev1.ServicePort{
					{
						Name:       "server",
						Port:       common.DefaultRedisPort,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromString("redis"),
					}, {
						Name:       "sentinel",
						Port:       common.DefaultRedisSentinelPort,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromString("sentinel"),
					},
				},
			},
		}

		req.ObjectMeta.Annotations[common.ServiceAlphaK8sKeyTolerateUnreadyEndpoints] = "true"

		desired, err := networking.RequestService(req)
		if err != nil {
			reconcileErrs.Append(errors.Wrapf(err, "reconcileHAAnnourceServices: failed to request service %s", desired.Name))
			continue
		}

		if err = controllerutil.SetControllerReference(rr.Instance, desired, rr.Scheme); err != nil {
			rr.Logger.Error(err, "reconcileHAAnnourceServices: failed to set owner reference for service", "name", desired.Name)
		}

		_, err = networking.GetService(desired.Name, desired.Namespace, rr.Client)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				reconcileErrs.Append(errors.Wrapf(err, "reconcileHAAnnourceServices: failed to retrieve service %s", desired.Name))
				continue
			}

			if err = networking.CreateService(desired, rr.Client); err != nil {
				reconcileErrs.Append(errors.Wrapf(err, "reconcileHAAnnourceServices: failed to create service %s", desired.Name))
				continue
			}
			rr.Logger.Info("service created", "name", desired.Name, "namespace", desired.Namespace)
			continue
		}
	}

	return reconcileErrs.ErrOrNil()
}

func (rr *RedisReconciler) deleteService(name, namespace string) error {
	if err := networking.DeleteService(name, namespace, rr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteService: failed to delete service %s", name)
	}
	rr.Logger.Info("service deleted", "name", name, "namespace", namespace)
	return nil
}
