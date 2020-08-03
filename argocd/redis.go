// Copyright 2019 Argo CD Operator Developers
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

package argocd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// getRedisInitScript will load the redis configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisConf(cr *v1alpha1.ArgoCD) string {
	path := fmt.Sprintf("%s/redis.conf.tpl", getRedisConfigPath())
	conf, err := common.LoadTemplateFile(path, map[string]string{})
	if err != nil {
		log.Error(err, "unable to load redis configuration")
		return ""
	}
	return conf
}

// getRedisConfigPath will return the path for the Redis configuration templates.
func getRedisConfigPath() string {
	path := os.Getenv("REDIS_CONFIG_PATH")
	if len(path) > 0 {
		return path
	}
	return common.ArgoCDDefaultRedisConfigPath
}

// getRedisContainerImage will return the container image for the Redis server.
func getRedisContainerImage(cr *v1alpha1.ArgoCD) string {
	img := cr.Spec.Redis.Image
	if len(img) <= 0 {
		img = common.ArgoCDDefaultRedisImage
	}

	tag := cr.Spec.Redis.Version
	if len(tag) <= 0 {
		tag = common.ArgoCDDefaultRedisVersion
	}
	return common.CombineImageTag(img, tag)
}

// getRedisHAContainerImage will return the container image for the Redis server in HA mode.
func getRedisHAContainerImage(cr *v1alpha1.ArgoCD) string {
	img := cr.Spec.Redis.Image
	if len(img) <= 0 {
		img = common.ArgoCDDefaultRedisImage
	}

	tag := cr.Spec.Redis.Version
	if len(tag) <= 0 {
		tag = common.ArgoCDDefaultRedisVersionHA
	}
	return common.CombineImageTag(img, tag)
}

// getRedisHAProxySConfig will load the Redis HA Proxy configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisHAProxyConfig(cr *v1alpha1.ArgoCD) string {
	path := fmt.Sprintf("%s/haproxy.cfg.tpl", getRedisConfigPath())
	vars := map[string]string{
		"ServiceName": common.NameWithSuffix(cr.ObjectMeta, "redis-ha"),
	}

	script, err := common.LoadTemplateFile(path, vars)
	if err != nil {
		log.Error(err, "unable to load redis haproxy configuration")
		return ""
	}
	return script
}

// getRedisHAProxyContainerImage will return the container image for the Redis HA Proxy.
func getRedisHAProxyContainerImage(cr *v1alpha1.ArgoCD) string {
	img := cr.Spec.HA.RedisProxyImage
	if len(img) <= 0 {
		img = common.ArgoCDDefaultRedisHAProxyImage
	}

	tag := cr.Spec.HA.RedisProxyVersion
	if len(tag) <= 0 {
		tag = common.ArgoCDDefaultRedisHAProxyVersion
	}

	return common.CombineImageTag(img, tag)
}

// getRedisHAProxyScript will load the Redis HA Proxy init script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisHAProxyScript(cr *v1alpha1.ArgoCD) string {
	path := fmt.Sprintf("%s/haproxy_init.sh.tpl", getRedisConfigPath())
	vars := map[string]string{
		"ServiceName": common.NameWithSuffix(cr.ObjectMeta, "redis-ha"),
	}

	script, err := common.LoadTemplateFile(path, vars)
	if err != nil {
		log.Error(err, "unable to load redis haproxy init script")
		return ""
	}
	return script
}

// getRedisHAProxyAddress will return the Redis HA Proxy service address for the given ArgoCD.
func getRedisHAProxyAddress(cr *v1alpha1.ArgoCD) string {
	suffix := fmt.Sprintf("redis-ha-haproxy:%v", common.ArgoCDDefaultRedisPort)
	return common.NameWithSuffix(cr.ObjectMeta, suffix)
}

func getRedisHAReplicas(cr *v1alpha1.ArgoCD) *int32 {
	replicas := common.ArgoCDDefaultRedisHAReplicas
	// TODO: Allow override of this value through CR?
	return &replicas
}

// getRedisInitScript will load the redis init script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisInitScript(cr *v1alpha1.ArgoCD) string {
	path := fmt.Sprintf("%s/init.sh.tpl", getRedisConfigPath())
	vars := map[string]string{
		"ServiceName": common.NameWithSuffix(cr.ObjectMeta, "redis-ha"),
	}

	script, err := common.LoadTemplateFile(path, vars)
	if err != nil {
		log.Error(err, "unable to load redis init-script")
		return ""
	}
	return script
}

// getRedisResources will return the ResourceRequirements for the Redis container.
func getRedisResources(cr *v1alpha1.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Redis.Resources != nil {
		resources = *cr.Spec.Redis.Resources
	}

	return resources
}

// getRedisSentinelConf will load the redis sentinel configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisSentinelConf(cr *v1alpha1.ArgoCD) string {
	path := fmt.Sprintf("%s/sentinel.conf.tpl", getRedisConfigPath())
	conf, err := common.LoadTemplateFile(path, map[string]string{})
	if err != nil {
		log.Error(err, "unable to load redis sentinel configuration")
		return ""
	}
	return conf
}

// getRedisServerAddress will return the Redis service address for the given ArgoCD.
func getRedisServerAddress(cr *v1alpha1.ArgoCD) string {
	if cr.Spec.HA.Enabled {
		return getRedisHAProxyAddress(cr)
	}

	suffix := fmt.Sprintf("%s:%v", common.ArgoCDDefaultRedisSuffix, common.ArgoCDDefaultRedisPort)
	return common.NameWithSuffix(cr.ObjectMeta, suffix)
}

// reconcileRedisConfiguration will ensure that all of the Redis ConfigMaps are present for the given ArgoCD.
func (r *ArgoClusterReconciler) reconcileRedisConfiguration(cr *v1alpha1.ArgoCD) error {
	if err := r.reconcileRedisHAConfigMap(cr); err != nil {
		return err
	}
	return nil
}

// reconcileRedisHAConfigMap will ensure that the Redis HA ConfigMap is present for the given ArgoCD.
func (r *ArgoClusterReconciler) reconcileRedisHAConfigMap(cr *v1alpha1.ArgoCD) error {
	cm := resources.NewConfigMapWithName(cr.ObjectMeta, common.ArgoCDRedisHAConfigMapName)
	if resources.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if !cr.Spec.HA.Enabled {
			// ConfigMap exists but HA enabled flag has been set to false, delete the ConfigMap
			return r.Client.Delete(context.TODO(), cm)
		}
		return nil // ConfigMap found with nothing changed, move along...
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

	cm.Data = map[string]string{
		"haproxy.cfg":     getRedisHAProxyConfig(cr),
		"haproxy_init.sh": getRedisHAProxyScript(cr),
		"init.sh":         getRedisInitScript(cr),
		"redis.conf":      getRedisConf(cr),
		"sentinel.conf":   getRedisSentinelConf(cr),
	}

	ctrl.SetControllerReference(cr, cm, r.Scheme)
	return r.Client.Create(context.TODO(), cm)
}

// reconcileRedisDeployment will ensure the Deployment resource is present for the ArgoCD Redis component.
func (r *ArgoClusterReconciler) reconcileRedisDeployment(cr *v1alpha1.ArgoCD) error {
	deploy := resources.NewDeploymentWithSuffix(cr.ObjectMeta, "redis", "redis")
	if resources.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		if cr.Spec.HA.Enabled {
			// Deployment exists but HA enabled flag has been set to true, delete the Deployment
			return r.Client.Delete(context.TODO(), deploy)
		}

		actualImage := deploy.Spec.Template.Spec.Containers[0].Image
		desiredImage := getRedisContainerImage(cr)
		if actualImage != desiredImage {
			deploy.Spec.Template.Spec.Containers[0].Image = desiredImage
			deploy.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			return r.Client.Update(context.TODO(), deploy)
		}
		return nil // Deployment found, do nothing
	}

	if cr.Spec.HA.Enabled {
		return nil // HA enabled, do nothing.
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Args: []string{
			"--save",
			"",
			"--appendonly",
			"no",
		},
		Image:           getRedisContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "redis",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultRedisPort,
			},
		},
		Resources: getRedisResources(cr),
	}}

	ctrl.SetControllerReference(cr, deploy, r.Scheme)
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileRedisHAProxyDeployment will ensure the Deployment resource is present for the Redis HA Proxy component.
func (r *ArgoClusterReconciler) reconcileRedisHAProxyDeployment(cr *v1alpha1.ArgoCD) error {
	deploy := resources.NewDeploymentWithSuffix(cr.ObjectMeta, "redis-ha-haproxy", "redis")
	if resources.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		if !cr.Spec.HA.Enabled {
			// Deployment exists but HA enabled flag has been set to false, delete the Deployment
			return r.Client.Delete(context.TODO(), deploy)
		}

		actualImage := deploy.Spec.Template.Spec.Containers[0].Image
		desiredImage := getRedisHAProxyContainerImage(cr)

		if actualImage != desiredImage {
			deploy.Spec.Template.Spec.Containers[0].Image = desiredImage
			deploy.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			return r.Client.Update(context.TODO(), deploy)
		}
		return nil // Deployment found, do nothing
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

	deploy.Spec.Template.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								common.ArgoCDKeyName: common.NameWithSuffix(cr.ObjectMeta, "redis-ha-haproxy"),
							},
						},
						TopologyKey: common.ArgoCDKeyFailureDomainZone,
					},
					Weight: int32(100),
				},
			},
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.ArgoCDKeyName: common.NameWithSuffix(cr.ObjectMeta, "redis-ha-haproxy"),
						},
					},
					TopologyKey: common.ArgoCDKeyHostname,
				},
			},
		},
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Image:           getRedisHAProxyContainerImage(cr),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Name:            "haproxy",
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8888),
				},
			},
			InitialDelaySeconds: int32(5),
			PeriodSeconds:       int32(3),
		},
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultRedisPort,
				Name:          "redis",
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "data",
				MountPath: "/usr/local/etc/haproxy",
			},
			{
				Name:      "shared-socket",
				MountPath: "/run/haproxy",
			},
		},
	}}

	deploy.Spec.Template.Spec.InitContainers = []corev1.Container{{
		Args: []string{
			"/readonly/haproxy_init.sh",
		},
		Command: []string{
			"sh",
		},
		Image:           getRedisHAProxyContainerImage(cr),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Name:            "config-init",
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "config-volume",
				MountPath: "/readonly",
				ReadOnly:  true,
			},
			{
				Name:      "data",
				MountPath: "/data",
			},
		},
	}}

	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDRedisHAConfigMapName,
					},
				},
			},
		},
		{
			Name: "shared-socket",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	deploy.Spec.Template.Spec.ServiceAccountName = "argocd-redis-ha"

	ctrl.SetControllerReference(cr, deploy, r.Scheme)
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileRedisHAAnnounceServices will ensure that the announce Services are present for Redis when running in HA mode.
func (r *ArgoClusterReconciler) reconcileRedisHAAnnounceServices(cr *v1alpha1.ArgoCD) error {
	for i := int32(0); i < common.ArgoCDDefaultRedisHAReplicas; i++ {
		svc := resources.NewServiceWithSuffix(cr.ObjectMeta, fmt.Sprintf("redis-ha-announce-%d", i), "redis")
		if resources.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
			return nil // Service found, do nothing
		}

		svc.ObjectMeta.Annotations = map[string]string{
			common.ArgoCDKeyTolerateUnreadyEndpounts: "true",
		}

		svc.Spec.PublishNotReadyAddresses = true

		svc.Spec.Selector = map[string]string{
			common.ArgoCDKeyName:               common.NameWithSuffix(cr.ObjectMeta, "redis-ha"),
			common.ArgoCDKeyStatefulSetPodName: common.NameWithSuffix(cr.ObjectMeta, fmt.Sprintf("redis-ha-server-%d", i)),
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

		ctrl.SetControllerReference(cr, svc, r.Scheme)
		if err := r.Client.Create(context.TODO(), svc); err != nil {
			return err
		}
	}
	return nil
}

// reconcileRedisHAMasterService will ensure that the "master" Service is present for Redis when running in HA mode.
func (r *ArgoClusterReconciler) reconcileRedisHAMasterService(cr *v1alpha1.ArgoCD) error {
	svc := resources.NewServiceWithSuffix(cr.ObjectMeta, "redis-ha", "redis")
	if resources.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: common.NameWithSuffix(cr.ObjectMeta, "redis-ha"),
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

	ctrl.SetControllerReference(cr, svc, r.Scheme)
	return r.Client.Create(context.TODO(), svc)
}

// reconcileRedisHAProxyService will ensure that the HA Proxy Service is present for Redis when running in HA mode.
func (r *ArgoClusterReconciler) reconcileRedisHAProxyService(cr *v1alpha1.ArgoCD) error {
	svc := resources.NewServiceWithSuffix(cr.ObjectMeta, "redis-ha-haproxy", "redis")
	if resources.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: common.NameWithSuffix(cr.ObjectMeta, "redis-ha-haproxy"),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "haproxy",
			Port:       common.ArgoCDDefaultRedisPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString("redis"),
		},
	}

	ctrl.SetControllerReference(cr, svc, r.Scheme)
	return r.Client.Create(context.TODO(), svc)
}

// reconcileRedisHAServices will ensure that all required Services are present for Redis when running in HA mode.
func (r *ArgoClusterReconciler) reconcileRedisHAServices(cr *v1alpha1.ArgoCD) error {
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
func (r *ArgoClusterReconciler) reconcileRedisService(cr *v1alpha1.ArgoCD) error {
	svc := resources.NewServiceWithSuffix(cr.ObjectMeta, "redis", "redis")
	if resources.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: common.NameWithSuffix(cr.ObjectMeta, "redis"),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "tcp-redis",
			Port:       common.ArgoCDDefaultRedisPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ArgoCDDefaultRedisPort),
		},
	}

	ctrl.SetControllerReference(cr, svc, r.Scheme)
	return r.Client.Create(context.TODO(), svc)
}

func (r *ArgoClusterReconciler) reconcileRedisStatefulSet(cr *v1alpha1.ArgoCD) error {
	ss := resources.NewStatefulSetWithSuffix(cr.ObjectMeta, "redis-ha-server", "redis")
	if resources.IsObjectFound(r.Client, cr.Namespace, ss.Name, ss) {
		if !cr.Spec.HA.Enabled {
			// StatefulSet exists but HA enabled flag has been set to false, delete the StatefulSet
			return r.Client.Delete(context.TODO(), ss)
		}

		desiredImage := getRedisContainerImage(cr)
		changed := false

		for _, container := range ss.Spec.Template.Spec.Containers {
			if container.Image != desiredImage {
				container.Image = getRedisHAContainerImage(cr)
			}
		}

		if changed {
			ss.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			return r.Client.Update(context.TODO(), ss)
		}

		return nil // StatefulSet found, do nothing
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

	ss.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
	ss.Spec.Replicas = getRedisHAReplicas(cr)
	ss.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.ArgoCDKeyName: common.NameWithSuffix(cr.ObjectMeta, "redis-ha"),
		},
	}

	ss.Spec.ServiceName = common.NameWithSuffix(cr.ObjectMeta, "redis-ha")

	ss.Spec.Template.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"checksum/init-config": "552ee3bec8fe5d9d865e371f7b615c6d472253649eb65d53ed4ae874f782647c", // TODO: Should this be hard-coded?
		},
		Labels: map[string]string{
			common.ArgoCDKeyName: common.NameWithSuffix(cr.ObjectMeta, "redis-ha"),
		},
	}

	ss.Spec.Template.Spec.Affinity = &corev1.Affinity{
		PodAffinity: &corev1.PodAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.ArgoCDKeyName: common.NameWithSuffix(cr.ObjectMeta, "redis-ha"),
						},
					},
					TopologyKey: common.ArgoCDKeyFailureDomainZone,
				},
				Weight: int32(100),
			}},
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						common.ArgoCDKeyName: common.NameWithSuffix(cr.ObjectMeta, "redis-ha"),
					},
				},
				TopologyKey: common.ArgoCDKeyHostname,
			}},
		},
	}

	ss.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Args: []string{
				"/data/conf/redis.conf",
			},
			Command: []string{
				"redis-server",
			},
			Image:           getRedisHAContainerImage(cr),
			ImagePullPolicy: corev1.PullIfNotPresent,
			LivenessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(common.ArgoCDDefaultRedisPort),
					},
				},
				InitialDelaySeconds: int32(15),
			},
			Name: "redis",
			Ports: []corev1.ContainerPort{{
				ContainerPort: common.ArgoCDDefaultRedisPort,
				Name:          "redis",
			}},
			Resources: getRedisResources(cr),
			VolumeMounts: []corev1.VolumeMount{
				{
					MountPath: "/data",
					Name:      "data",
				},
			},
		},
		{
			Args: []string{
				"/data/conf/sentinel.conf",
			},
			Command: []string{
				"redis-sentinel",
			},
			Image:           getRedisHAContainerImage(cr),
			ImagePullPolicy: corev1.PullIfNotPresent,
			LivenessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(common.ArgoCDDefaultRedisSentinelPort),
					},
				},
				InitialDelaySeconds: int32(15),
			},
			Name: "sentinel",
			Ports: []corev1.ContainerPort{{
				ContainerPort: common.ArgoCDDefaultRedisSentinelPort,
				Name:          "sentinel",
			}},
			Resources: getRedisResources(cr),
			VolumeMounts: []corev1.VolumeMount{
				{
					MountPath: "/data",
					Name:      "data",
				},
			},
		},
	}

	ss.Spec.Template.Spec.InitContainers = []corev1.Container{{
		Args: []string{
			"/readonly-config/init.sh",
		},
		Command: []string{
			"sh",
		},
		Env: []corev1.EnvVar{
			{
				Name:  "SENTINEL_ID_0",
				Value: "25b71bd9d0e4a51945d8422cab53f27027397c12", // TODO: Should this be hard-coded?
			},
			{
				Name:  "SENTINEL_ID_1",
				Value: "896627000a81c7bdad8dbdcffd39728c9c17b309", // TODO: Should this be hard-coded?
			},
			{
				Name:  "SENTINEL_ID_2",
				Value: "3acbca861108bc47379b71b1d87d1c137dce591f", // TODO: Should this be hard-coded?
			},
		},
		Image:           getRedisHAContainerImage(cr),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Name:            "config-init",
		Resources:       getRedisResources(cr),
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/readonly-config",
				Name:      "config",
				ReadOnly:  true,
			},
			{
				MountPath: "/data",
				Name:      "data",
			},
		},
	}}

	var fsGroup int64 = 1000
	var runAsNonRoot bool = true
	var runAsUser int64 = 1000

	ss.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
		FSGroup:      &fsGroup,
		RunAsNonRoot: &runAsNonRoot,
		RunAsUser:    &runAsUser,
	}

	ss.Spec.Template.Spec.ServiceAccountName = "argocd-redis-ha"

	ss.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDRedisHAConfigMapName,
					},
				},
			},
		}, {
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	ss.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
		Type: appsv1.RollingUpdateStatefulSetStrategyType,
	}

	ctrl.SetControllerReference(cr, ss, r.Scheme)
	return r.Client.Create(context.TODO(), ss)
}

// reconcileStatusRedis will ensure that the Redis status is updated for the given ArgoCD.
func (r *ArgoClusterReconciler) reconcileStatusRedis(cr *v1alpha1.ArgoCD) error {
	status := "Unknown"

	if !cr.Spec.HA.Enabled {
		deploy := resources.NewDeploymentWithSuffix(cr.ObjectMeta, "redis", "redis")
		if resources.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
			status = "Pending"

			if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
				status = "Running"
			}
		}
	} else {
		ss := resources.NewStatefulSetWithSuffix(cr.ObjectMeta, "redis-ha-server", "redis-ha-server")
		if resources.IsObjectFound(r.Client, cr.Namespace, ss.Name, ss) {
			status = "Pending"

			if ss.Status.ReadyReplicas == *ss.Spec.Replicas {
				status = "Running"
			}
		}
		// TODO: Add check for HA proxy deployment here as well?
	}

	if cr.Status.Redis != status {
		cr.Status.Redis = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}
