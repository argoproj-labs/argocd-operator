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
		ObjectMeta: argoutil.GetObjMeta(resourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
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
	return rr.reconService(req, argocdcommon.UpdateFnSvc(updateFn), ignoreDrift)
}

// reconcileHAProxyService will ensure that the HA Proxy Service is present for Redis when running in HA mode.
func (rr *RedisReconciler) reconcileHAProxyService() error {
	req := networking.ServiceRequest{
		ObjectMeta: argoutil.GetObjMeta(HAProxyResourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
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
	return rr.reconService(req, argocdcommon.UpdateFnSvc(updateFn), ignoreDrift)
}

func (rr *RedisReconciler) reconcileHAMasterService() error {
	req := networking.ServiceRequest{
		ObjectMeta: argoutil.GetObjMeta(HAResourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
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
	return rr.reconService(req, argocdcommon.UpdateFnSvc(updateFn), ignoreDrift)
}

func (rr *RedisReconciler) reconcileHAAnnounceServices() error {
	var reconcileErrs util.MultiError

	for i := int32(0); i < common.DefaultRedisHAReplicas; i++ {
		name := argoutil.GenerateResourceName(rr.Instance.Name, common.RedisHAAnnouceSuffix, strconv.Itoa(int(i)))

		req := networking.ServiceRequest{
			ObjectMeta: argoutil.GetObjMeta(name, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
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

		err := rr.reconService(req, argocdcommon.UpdateFnSvc(updateFn), ignoreDrift)
		reconcileErrs.Append(err)
	}
	return reconcileErrs.ErrOrNil()
}

func (rr *RedisReconciler) reconService(req networking.ServiceRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := networking.RequestService(req)
	if err != nil {
		rr.Logger.Debug("reconcileService: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconcileService: failed to request Service %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desired, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileService: failed to set owner reference for Service", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := networking.GetService(desired.Name, desired.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileService: failed to retrieve Service %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = networking.CreateService(desired, rr.Client); err != nil {
			return errors.Wrapf(err, "reconcileService: failed to create Service %s in namespace %s", desired.Name, desired.Namespace)
		}
		rr.Logger.Info("service created", "name", desired.Name, "namespace", desired.Namespace)
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

	if err = networking.UpdateService(existing, rr.Client); err != nil {
		return errors.Wrapf(err, "reconcileService: failed to update service %s", existing.Name)
	}

	rr.Logger.Info("service updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
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
