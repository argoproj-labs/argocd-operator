package redis

import (
	"fmt"

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
	svcRequest := networking.ServiceRequest{
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
		}),
		Client: rr.Client,
	}

	desiredSvc, err := networking.RequestService(svcRequest)
	if err != nil {
		return errors.Wrapf(err, "reconcileService: failed to request service %s", desiredSvc.Name)
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desiredSvc, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileService: failed to set owner reference for service", "name", desiredSvc.Name)
	}

	existingSvc, err := networking.GetService(desiredSvc.Name, desiredSvc.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileService: failed to retrieve service %s", desiredSvc.Name)
		}

		if err = networking.CreateService(desiredSvc, rr.Client); err != nil {
			return errors.Wrapf(err, "reconcileService: failed to create service %s", desiredSvc.Name)
		}
		rr.Logger.V(0).Info("service created", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)
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

	if err = networking.UpdateService(existingSvc, rr.Client); err != nil {
		return errors.Wrapf(err, "reconcileService: failed to update service %s", existingSvc.Name)
	}

	rr.Logger.V(0).Info("service updated", "name", existingSvc.Name, "namespace", existingSvc.Namespace)
	return nil
}

// reconcileHAProxyService will ensure that the HA Proxy Service is present for Redis when running in HA mode.
func (rr *RedisReconciler) reconcileHAProxyService() error {
	svcRequest := networking.ServiceRequest{
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

	desiredSvc, err := networking.RequestService(svcRequest)
	if err != nil {
		return errors.Wrapf(err, "reconcileHAProxyService: failed to request service %s", desiredSvc.Name)
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desiredSvc, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileHAProxyService: failed to set owner reference for service", "name", desiredSvc.Name)
	}

	existingSvc, err := networking.GetService(desiredSvc.Name, desiredSvc.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileHAProxyService: failed to retrieve service %s", desiredSvc.Name)
		}

		if err = networking.CreateService(desiredSvc, rr.Client); err != nil {
			return errors.Wrapf(err, "reconcileHAProxyService: failed to create service %s", desiredSvc.Name)
		}
		rr.Logger.V(0).Info("service created", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)
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

	if err = networking.UpdateService(existingSvc, rr.Client); err != nil {
		return errors.Wrapf(err, "reconcileHAProxyService: failed to update service %s", existingSvc.Name)
	}

	rr.Logger.V(0).Info("service updated", "name", existingSvc.Name, "namespace", existingSvc.Namespace)
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

	desiredSvc, err := networking.RequestService(svcRequest)
	if err != nil {
		return errors.Wrapf(err, "failed to request service %s", desiredSvc.Name)
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desiredSvc, rr.Scheme); err != nil {
		rr.Logger.Error(err, "failed to set owner reference for service", "name", desiredSvc.Name)
	}

	_, err = networking.GetService(desiredSvc.Name, desiredSvc.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to retrieve service %s", desiredSvc.Name)
		}

		if err = networking.CreateService(desiredSvc, rr.Client); err != nil {
			return errors.Wrapf(err, "failed to create service %s", desiredSvc.Name)
		}
		rr.Logger.V(0).Info("service created", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)
		return nil
	}

	// nothing to do
	return nil
}

func (rr *RedisReconciler) reconcileHAAnnourceServices() error {
	var reconcileErrs util.MultiError

	for i := int32(0); i < common.DefaultRedisHAReplicas; i++ {
		svcRequest := networking.ServiceRequest{
			ObjectMeta: argoutil.GetObjMeta(argoutil.GenerateResourceName(rr.Instance.Name, fmt.Sprintf("%s-%d", common.RedisHAAnnouceSuffix, i)), rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component),
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					common.AppK8sKeyName:            HAResourceName,
					common.StatefulSetK8sKeyPodName: argoutil.GenerateResourceName(rr.Instance.Name, fmt.Sprintf("%s-%d", common.RedisHASuffix, i)),
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

		svcRequest.ObjectMeta.Annotations[common.ServiceAlphaK8sKeyTolerateUnreadyEndpoints] = "true"

		desiredSvc, err := networking.RequestService(svcRequest)
		if err != nil {
			reconcileErrs.Append(errors.Wrapf(err, "reconcileHAAnnourceServices: failed to request service %s", desiredSvc.Name))
			continue
		}

		if err = controllerutil.SetControllerReference(rr.Instance, desiredSvc, rr.Scheme); err != nil {
			rr.Logger.Error(err, "reconcileHAAnnourceServices: failed to set owner reference for service", "name", desiredSvc.Name)
		}

		_, err = networking.GetService(desiredSvc.Name, desiredSvc.Namespace, rr.Client)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				reconcileErrs.Append(errors.Wrapf(err, "reconcileHAAnnourceServices: failed to retrieve service %s", desiredSvc.Name))
				continue
			}

			if err = networking.CreateService(desiredSvc, rr.Client); err != nil {
				reconcileErrs.Append(errors.Wrapf(err, "reconcileHAAnnourceServices: failed to create service %s", desiredSvc.Name))
				continue
			}
			rr.Logger.V(0).Info("service created", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)
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
	rr.Logger.V(0).Info("service deleted", "name", name, "namespace", namespace)
	return nil
}
