package redis

import (
	"fmt"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/networking"
	"github.com/argoproj-labs/argocd-operator/pkg/openshift"
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
	}

	if rr.IsOpenShiftEnv && rr.Instance.Spec.Redis.WantsAutoTLS() {
		svcRequest = openshift.AddAutoTLSAnnotation(svcRequest, common.ArgoCDRedisServerTLSSecretName)
	}

	desiredSvc, err := networking.RequestService(svcRequest)
	if err != nil {
		return errors.Wrap(err, "reconcileService: failed to request service")
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desiredSvc, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileService: failed to set owner reference for service")
	}

	existingSvc, err := networking.GetService(desiredSvc.Name, desiredSvc.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "reconcileService: failed to retrieve service")
		}

		if err = networking.CreateService(desiredSvc, rr.Client); err != nil {
			return errors.Wrap(err, "reconcileService: failed to create service")
		}
		rr.Logger.V(0).Info("service created", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)
		return nil
	}

	svcChanged := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{existingSvc.ObjectMeta, desiredSvc.ObjectMeta, nil},
		{existingSvc.Spec, desiredSvc.Spec, nil},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &svcChanged)
	}

	if !svcChanged {
		return nil
	}

	if err = networking.UpdateService(existingSvc, rr.Client); err != nil {
		return errors.Wrap(err, "reconcileService: failed to update service")
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
	}

	if rr.IsOpenShiftEnv && rr.Instance.Spec.Redis.WantsAutoTLS() {
		svcRequest = openshift.AddAutoTLSAnnotation(svcRequest, common.ArgoCDRedisServerTLSSecretName)
	}

	desiredSvc, err := networking.RequestService(svcRequest)
	if err != nil {
		return errors.Wrap(err, "reconcileHAProxyService: failed to request service")
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desiredSvc, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileHAProxyService: failed to set owner reference for service")
	}

	existingSvc, err := networking.GetService(desiredSvc.Name, desiredSvc.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "reconcileHAProxyService: failed to retrieve service")
		}

		if err = networking.CreateService(desiredSvc, rr.Client); err != nil {
			return errors.Wrap(err, "reconcileHAProxyService: failed to create service")
		}
		rr.Logger.V(0).Info("service created", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)
		return nil
	}

	svcChanged := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{existingSvc.ObjectMeta, desiredSvc.ObjectMeta, nil},
		{existingSvc.Spec, desiredSvc.Spec, nil},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &svcChanged)
	}

	if !svcChanged {
		return nil
	}

	if err = networking.UpdateService(existingSvc, rr.Client); err != nil {
		return errors.Wrap(err, "reconcileHAProxyService: failed to update service")
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
		return errors.Wrap(err, "reconcileHAMasterService: failed to request service")
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desiredSvc, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileHAMasterService: failed to set owner reference for service")
	}

	_, err = networking.GetService(desiredSvc.Name, desiredSvc.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "reconcileHAMasterService: failed to retrieve service")
		}

		if err = networking.CreateService(desiredSvc, rr.Client); err != nil {
			return errors.Wrap(err, "reconcileHAMasterService: failed to create service")
		}
		rr.Logger.V(0).Info("service created", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)
		return nil
	}

	// nothing to do
	return nil
}

func (rr *RedisReconciler) reconcileHAAnnourceServices() []error {
	var reconcileErrs []error

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
			reconcileErrs = append(reconcileErrs, errors.Wrap(err, fmt.Sprintf("reconcileHAAnnourceServices: failed to request service %s", desiredSvc.Name)))
			continue
		}

		if err = controllerutil.SetControllerReference(rr.Instance, desiredSvc, rr.Scheme); err != nil {
			rr.Logger.Error(err, fmt.Sprintf("reconcileHAAnnourceServices: failed to set owner reference for service %s", desiredSvc.Name))
		}

		_, err = networking.GetService(desiredSvc.Name, desiredSvc.Namespace, rr.Client)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				reconcileErrs = append(reconcileErrs, errors.Wrap(err, fmt.Sprintf("reconcileHAAnnourceServices: failed to retrieve service %s", desiredSvc.Name)))
				continue
			}

			if err = networking.CreateService(desiredSvc, rr.Client); err != nil {
				reconcileErrs = append(reconcileErrs, errors.Wrap(err, fmt.Sprintf("reconcileHAAnnourceServices: failed to create service %s", desiredSvc.Name)))
				continue
			}
			rr.Logger.V(0).Info("service created", "name", desiredSvc.Name, "namespace", desiredSvc.Namespace)
			continue
		}
	}

	return reconcileErrs
}
