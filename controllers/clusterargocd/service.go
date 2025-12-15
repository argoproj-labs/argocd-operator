// Copyright 2019 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clusterargocd

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// getArgoServerServiceType will return the server Service type for the ArgoCD.
func getArgoServerServiceType(cr *argoproj.ClusterArgoCD) corev1.ServiceType {
	if len(cr.Spec.Server.Service.Type) > 0 {
		return cr.Spec.Server.Service.Type
	}

	return corev1.ServiceTypeClusterIP
}

// newService returns a new Service for the given ArgoCD instance.
func newService(cr *argoproj.ClusterArgoCD) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Spec.ControlPlaneNamespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
	}
}

// newServiceWithName returns a new Service instance for the given ArgoCD using the given name.
func newServiceWithName(name string, component string, cr *argoproj.ClusterArgoCD) *corev1.Service {
	svc := newService(cr)

	// The name is already truncated by nameWithSuffix, so use it directly
	svc.Name = name

	lbls := svc.Labels
	lbls[common.ArgoCDKeyName] = name
	lbls[common.ArgoCDKeyComponent] = component
	svc.Labels = lbls

	return svc
}

// newServiceWithSuffix returns a new Service instance for the given ArgoCD using the given suffix.
func newServiceWithSuffix(suffix string, component string, cr *argoproj.ClusterArgoCD) *corev1.Service {
	return newServiceWithName(nameWithSuffix(suffix, cr), component, cr)
}

// reconcileGrafanaService will ensure that the Service for Grafana is present.
func (r *ReconcileClusterArgoCD) reconcileGrafanaService(cr *argoproj.ClusterArgoCD) error {
	svc := newServiceWithSuffix("grafana", "grafana", cr)
	svcExists, err := argoutil.IsObjectFound(r.Client, cr.Spec.ControlPlaneNamespace, svc.Name, svc)
	if err != nil {
		return err
	}
	if svcExists {
		//lint:ignore SA1019 known to be deprecated
		if !cr.Spec.Grafana.Enabled { //nolint:staticcheck // SA1019: We must test deprecated fields.
			// Service exists but enabled flag has been set to false, delete the Service
			argoutil.LogResourceDeletion(log, svc, "grafana is disabled")
			return r.Delete(context.TODO(), svc)
		}
		log.Info(grafanaDeprecatedWarning)
		return nil // Service found, do nothing
	}

	//lint:ignore SA1019 known to be deprecated
	if !cr.Spec.Grafana.Enabled { //nolint:staticcheck // SA1019: We must test deprecated fields.
		return nil // Grafana not enabled, do nothing.
	}

	log.Info(grafanaDeprecatedWarning)
	return nil
}

// reconcileMetricsService will ensure that the Service for the Argo CD application controller metrics is present.
func (r *ReconcileClusterArgoCD) reconcileMetricsService(cr *argoproj.ClusterArgoCD) error {
	svc := newServiceWithSuffix("metrics", "metrics", cr)
	svcExists, err := argoutil.IsObjectFound(r.Client, cr.Spec.ControlPlaneNamespace, svc.Name, svc)
	if err != nil {
		return err
	}
	if svcExists {
		// Service found, do nothing
		return nil
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("application-controller", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       8082,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8082),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, svc)
	return r.Create(context.TODO(), svc)
}

// reconcileRedisHAAnnounceServices will ensure that the announce Services are present for Redis when running in HA mode.
func (r *ReconcileClusterArgoCD) reconcileRedisHAAnnounceServices(cr *argoproj.ClusterArgoCD) error {
	for i := int32(0); i < common.ArgoCDDefaultRedisHAReplicas; i++ {
		svc := newServiceWithSuffix(fmt.Sprintf("redis-ha-announce-%d", i), "redis", cr)
		svcExists, err := argoutil.IsObjectFound(r.Client, cr.Spec.ControlPlaneNamespace, svc.Name, svc)
		if err != nil {
			return err
		}
		if svcExists {
			if !cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
				var explanation string
				if !cr.Spec.HA.Enabled {
					explanation = "ha is disabled"
				} else {
					explanation = "redis is disabled"
				}
				argoutil.LogResourceDeletion(log, svc, explanation)
				return r.Delete(context.TODO(), svc)
			}
			return nil // Service found, do nothing
		}

		if !cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
			return nil //return as Ha is not enabled do nothing
		}

		svc.Annotations = map[string]string{
			common.ArgoCDKeyTolerateUnreadyEndpounts: "true",
		}

		svc.Spec.PublishNotReadyAddresses = true

		svc.Spec.Selector = map[string]string{
			common.ArgoCDKeyName:               nameWithSuffix("redis-ha", cr),
			common.ArgoCDKeyStatefulSetPodName: nameWithSuffix(fmt.Sprintf("redis-ha-server-%d", i), cr),
		}

		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "server",
				Port:       common.ArgoCDDefaultRedisPort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromString("redis"),
			}, {
				Name:       "sentinel",
				Port:       common.ArgoCDDefaultRedisSentinelPort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromString("sentinel"),
			},
		}

		if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
			return err
		}

		argoutil.LogResourceCreation(log, svc)
		if err := r.Create(context.TODO(), svc); err != nil {
			return err
		}
	}
	return nil
}

// reconcileRedisHAMasterService will ensure that the "master" Service is present for Redis when running in HA mode.
func (r *ReconcileClusterArgoCD) reconcileRedisHAMasterService(cr *argoproj.ClusterArgoCD) error {
	svc := newServiceWithSuffix("redis-ha", "redis", cr)
	svcExists, err := argoutil.IsObjectFound(r.Client, cr.Spec.ControlPlaneNamespace, svc.Name, svc)
	if err != nil {
		return err
	}
	if svcExists {
		if !cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
			var explanation string
			if !cr.Spec.HA.Enabled {
				explanation = "ha is disabled"
			} else {
				explanation = "redis is disabled"
			}
			argoutil.LogResourceDeletion(log, svc, explanation)
			return r.Delete(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	if !cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
		return nil //return as Ha is not enabled do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("redis-ha", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "server",
			Port:       common.ArgoCDDefaultRedisPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString("redis"),
		}, {
			Name:       "sentinel",
			Port:       common.ArgoCDDefaultRedisSentinelPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString("sentinel"),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, svc)
	return r.Create(context.TODO(), svc)
}

// reconcileRedisHAProxyService will ensure that the HA Proxy Service is present for Redis when running in HA mode.
func (r *ReconcileClusterArgoCD) reconcileRedisHAProxyService(cr *argoproj.ClusterArgoCD) error {
	svc := newServiceWithSuffix("redis-ha-haproxy", "redis", cr)
	svcExists, err := argoutil.IsObjectFound(r.Client, cr.Spec.ControlPlaneNamespace, svc.Name, svc)
	if err != nil {
		return err
	}
	if svcExists {

		if !cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
			var explanation string
			if !cr.Spec.HA.Enabled {
				explanation = "ha is disabled"
			} else {
				explanation = "redis is disabled"
			}
			argoutil.LogResourceDeletion(log, svc, explanation)
			return r.Delete(context.TODO(), svc)
		}

		updated, err := ensureAutoTLSAnnotation(r.Client, svc, common.ArgoCDRedisServerTLSSecretName, cr.Spec.Redis.WantsAutoTLS())
		if err != nil {
			return err
		}
		if updated {
			argoutil.LogResourceUpdate(log, svc, "updating auto tls annotation")
			return r.Update(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	if !cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
		return nil //return as Ha is not enabled do nothing
	}

	_, err = ensureAutoTLSAnnotation(r.Client, svc, common.ArgoCDRedisServerTLSSecretName, cr.Spec.Redis.WantsAutoTLS())
	if err != nil {
		return fmt.Errorf("unable to ensure AutoTLS annotation: %w", err)
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("redis-ha-haproxy", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "haproxy",
			Port:       common.ArgoCDDefaultRedisPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString("redis"),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, svc)
	return r.Create(context.TODO(), svc)
}

// reconcileRedisHAServices will ensure that all required Services are present for Redis when running in HA mode.
func (r *ReconcileClusterArgoCD) reconcileRedisHAServices(cr *argoproj.ClusterArgoCD) error {

	if err := r.reconcileRedisHAAnnounceServices(cr); err != nil {
		return err
	}

	if err := r.reconcileRedisHAMasterService(cr); err != nil {
		return err
	}

	if err := r.reconcileRedisHAProxyService(cr); err != nil {
		return err
	}
	return nil
}

// reconcileRedisService will ensure that the Service for Redis is present.
func (r *ReconcileClusterArgoCD) reconcileRedisService(cr *argoproj.ClusterArgoCD) error {
	svc := newServiceWithSuffix("redis", "redis", cr)

	svcExists, err := argoutil.IsObjectFound(r.Client, cr.Spec.ControlPlaneNamespace, svc.Name, svc)
	if err != nil {
		return err
	}
	if svcExists {
		if !cr.Spec.Redis.IsEnabled() {
			argoutil.LogResourceDeletion(log, svc, "redis is disabled")
			return r.Delete(context.TODO(), svc)
		}
		update, err := ensureAutoTLSAnnotation(r.Client, svc, common.ArgoCDRedisServerTLSSecretName, cr.Spec.Redis.WantsAutoTLS())
		if err != nil {
			return err
		}
		if update {
			argoutil.LogResourceUpdate(log, svc, "updating auto tls annotation")
			return r.Update(context.TODO(), svc)
		}
		if cr.Spec.HA.Enabled {
			argoutil.LogResourceDeletion(log, svc, "ha is disabled")
			return r.Delete(context.TODO(), svc)
		}
		if cr.Spec.Redis.IsRemote() {
			argoutil.LogResourceDeletion(log, svc, "remote redis is configured")
			return r.Delete(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	svc.Spec.Type = corev1.ServiceTypeClusterIP

	if cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
		return nil //return as Ha is enabled do nothing
	}

	_, err = ensureAutoTLSAnnotation(r.Client, svc, common.ArgoCDRedisServerTLSSecretName, cr.Spec.Redis.WantsAutoTLS())
	if err != nil {
		return fmt.Errorf("unable to ensure AutoTLS annotation: %w", err)
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("redis", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "tcp-redis",
			Port:       common.ArgoCDDefaultRedisPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ArgoCDDefaultRedisPort),
		},
	}

	if cr.Spec.Redis.IsEnabled() && cr.Spec.Redis.IsRemote() {
		log.Info("Skipping service creation, redis remote is enabled")
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, svc)
	return r.Create(context.TODO(), svc)
}

// ensureAutoTLSAnnotation ensures that the service svc has the desired state
// of the auto TLS annotation set, which is either set (when enabled is true)
// or unset (when enabled is false).
//
// Returns true when annotations have been updated, otherwise returns false.
//
// When this method returns true, the svc resource will need to be updated on
// the cluster.
func ensureAutoTLSAnnotation(k8sClient client.Client, svc *corev1.Service, secretName string, enabled bool) (bool, error) {
	var autoTLSAnnotationName, autoTLSAnnotationValue string

	// We currently only support OpenShift for automatic TLS
	if argoutil.IsRouteAPIAvailable() {
		autoTLSAnnotationName = common.AnnotationOpenShiftServiceCA
		if svc.Annotations == nil {
			svc.Annotations = make(map[string]string)
		}
		autoTLSAnnotationValue = secretName
	}

	if autoTLSAnnotationName != "" {
		val, ok := svc.Annotations[autoTLSAnnotationName]
		if enabled {
			// Don't request a TLS certificate from the OpenShift Service CA if the secret already exists.
			isTLSSecretFound, err := argoutil.IsObjectFound(k8sClient, svc.Namespace, secretName, &corev1.Secret{})
			if err != nil {
				return false, err
			}
			if !ok && isTLSSecretFound {
				log.Info(fmt.Sprintf("skipping AutoTLS on service %s since the TLS secret is already present", svc.Name))
				return false, nil
			}
			if !ok || val != secretName {
				log.Info(fmt.Sprintf("requesting AutoTLS on service %s", svc.Name))
				svc.Annotations[autoTLSAnnotationName] = autoTLSAnnotationValue
				return true, nil
			}
		} else {
			if ok {
				log.Info(fmt.Sprintf("removing AutoTLS from service %s", svc.Name))
				delete(svc.Annotations, autoTLSAnnotationName)
				return true, nil
			}
		}
	}

	return false, nil
}

// reconcileServerMetricsService will ensure that the Service for the Argo CD server metrics is present.
func (r *ReconcileClusterArgoCD) reconcileServerMetricsService(cr *argoproj.ClusterArgoCD) error {
	svc := newServiceWithSuffix("server-metrics", "server", cr)
	svcExists, err := argoutil.IsObjectFound(r.Client, cr.Spec.ControlPlaneNamespace, svc.Name, svc)
	if err != nil {
		return err
	}
	if svcExists {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("server", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       8083,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8083),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, svc)
	return r.Create(context.TODO(), svc)
}

// reconcileServerService will ensure that the Service is present for the Argo CD server component.
func (r *ReconcileClusterArgoCD) reconcileServerService(cr *argoproj.ClusterArgoCD) error {
	svc := newServiceWithSuffix("server", "server", cr)

	if _, err := ensureAutoTLSAnnotation(r.Client, svc, common.ArgoCDServerTLSSecretName, cr.Spec.Server.WantsAutoTLS()); err != nil {
		return fmt.Errorf("unable to ensure AutoTLS annotation: %w", err)
	}

	svc.Spec.Ports = []corev1.ServicePort{
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
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("server", cr),
	}

	svc.Spec.Type = getArgoServerServiceType(cr)

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}

	existingSVC := &corev1.Service{}
	svcExists, err := argoutil.IsObjectFound(r.Client, cr.Spec.ControlPlaneNamespace, svc.Name, existingSVC)
	if err != nil {
		return err
	}
	if svcExists {
		changed := false
		explanation := ""
		if !cr.Spec.Server.IsEnabled() {
			argoutil.LogResourceDeletion(log, svc, "argocd server is disabled")
			return r.Delete(context.TODO(), svc)
		}
		update, err := ensureAutoTLSAnnotation(r.Client, existingSVC, common.ArgoCDServerTLSSecretName, cr.Spec.Server.WantsAutoTLS())
		if err != nil {
			return err
		}
		if update {
			explanation = "auto tls annotation"
			changed = true
		}
		if !reflect.DeepEqual(svc.Spec.Type, existingSVC.Spec.Type) {
			existingSVC.Spec.Type = svc.Spec.Type
			if changed {
				explanation += ", "
			}
			explanation += "service type"
			changed = true
		}
		if changed {
			argoutil.LogResourceUpdate(log, existingSVC, "updating", explanation)
			return r.Update(context.TODO(), existingSVC)
		}
		return nil
	}

	if !cr.Spec.Server.IsEnabled() {
		return nil
	}
	argoutil.LogResourceCreation(log, svc)
	return r.Create(context.TODO(), svc)
}

// reconcileServices will ensure that all Services are present for the given ArgoCD.
func (r *ReconcileClusterArgoCD) reconcileServices(cr *argoproj.ClusterArgoCD) error {

	if err := r.reconcileDexService(cr); err != nil {
		log.Error(err, "error reconciling dex service")
	}

	err := r.reconcileGrafanaService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileMetricsService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRedisHAServices(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRedisService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRepoService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileServerMetricsService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileServerService(cr)
	if err != nil {
		return err
	}
	return nil
}
