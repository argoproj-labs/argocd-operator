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

package argocd

import (
	"context"
	"fmt"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func getRedisHAReplicas(cr *argoprojv1a1.ArgoCD) *int32 {
	replicas := common.ArgoCDDefaultRedisHAReplicas
	// TODO: Allow override of this value through CR?
	return &replicas
}

// newStatefulSet returns a new StatefulSet instance for the given ArgoCD instance.
func newStatefulSet(cr *argoprojv1a1.ArgoCD) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}

// newStatefulSetWithName returns a new StatefulSet instance for the given ArgoCD using the given name.
func newStatefulSetWithName(name string, component string, cr *argoprojv1a1.ArgoCD) *appsv1.StatefulSet {
	ss := newStatefulSet(cr)
	ss.ObjectMeta.Name = name

	lbls := ss.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	lbls[common.ArgoCDKeyComponent] = component
	ss.ObjectMeta.Labels = lbls

	return ss
}

// newStatefulSetWithSuffix returns a new StatefulSet instance for the given ArgoCD using the given suffix.
func newStatefulSetWithSuffix(suffix string, component string, cr *argoprojv1a1.ArgoCD) *appsv1.StatefulSet {
	return newStatefulSetWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), component, cr)
}

func (r *ReconcileArgoCD) reconcileRedisStatefulSet(cr *argoprojv1a1.ArgoCD) error {
	ss := newStatefulSetWithSuffix("redis-ha-server", "redis", cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, ss.Name, ss) {
		if !cr.Spec.HA.Enabled {
			// StatefulSet exists but HA enabled flag has been set to false, delete the StatefulSet
			return r.client.Delete(context.TODO(), ss)
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
			common.ArgoCDKeyName: nameWithSuffix("redis-ha", cr),
		},
	}

	ss.Spec.ServiceName = nameWithSuffix("redis-ha", cr)

	ss.Spec.Template.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"checksum/init-config":  "06440ee4a409be2aa01dfa08c14dd964fe3bad2ada57da1a538ad5cd771a045f", // TODO: Should this be hard-coded?
			"checksum/probe-config": "4b9888f173366e436f167856ee3469e8c1fd5221e29caa2129373db23578ec10", // TODO: Should this be hard-coded?
		},
		Labels: map[string]string{
			common.ArgoCDKeyName: nameWithSuffix("redis-ha", cr),
		},
	}

	ss.Spec.Template.Spec.Affinity = &corev1.Affinity{
		PodAffinity: &corev1.PodAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.ArgoCDKeyName: nameWithSuffix("redis-ha", cr),
						},
					},
					TopologyKey: common.ArgoCDKeyFailureDomainZone,
				},
				Weight: int32(100),
			}},
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						common.ArgoCDKeyName: nameWithSuffix("redis-ha", cr),
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
			Image:           "redis:5.0.3-alpine",
			ImagePullPolicy: corev1.PullIfNotPresent,
			LivenessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"sh",
							"/probes/readiness.sh",
							string(common.ArgoCDDefaultRedisPort),
						},
					},
				},
				InitialDelaySeconds: int32(15),
				PeriodSeconds:       int32(5),
			},
			Name: "redis",
			Ports: []corev1.ContainerPort{{
				ContainerPort: common.ArgoCDDefaultRedisPort,
				Name:          "redis",
			}},
			ReadinessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"sh",
							"/probes/readiness.sh",
							string(common.ArgoCDDefaultRedisPort),
						},
					},
				},
				InitialDelaySeconds: int32(15),
				PeriodSeconds:       int32(5),
			},
			Resources: getRedisResources(cr),
			VolumeMounts: []corev1.VolumeMount{
				{
					MountPath: "/data",
					Name:      "data",
				},
				{
					MountPath: "/probes",
					Name:      "probes",
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
			Image:           "redis:5.0.3-alpine",
			ImagePullPolicy: corev1.PullIfNotPresent,
			LivenessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"sh",
							"/probes/readiness.sh",
							string(common.ArgoCDDefaultRedisSentinelPort),
						},
					},
				},
				InitialDelaySeconds: int32(15),
				PeriodSeconds:       int32(5),
			},
			Name: "sentinel",
			Ports: []corev1.ContainerPort{{
				ContainerPort: common.ArgoCDDefaultRedisSentinelPort,
				Name:          "sentinel",
			}},
			ReadinessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"sh",
							"/probes/readiness.sh",
							string(common.ArgoCDDefaultRedisSentinelPort),
						},
					},
				},
				InitialDelaySeconds: int32(15),
				PeriodSeconds:       int32(5),
			},
			Resources: getRedisResources(cr),
			VolumeMounts: []corev1.VolumeMount{
				{
					MountPath: "/data",
					Name:      "data",
				},
				{
					MountPath: "/probes",
					Name:      "probes",
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
				Value: "e791a161cb06f0d3eb0cc392117d34cf0eae9d71", // TODO: Should this be hard-coded?
			},
			{
				Name:  "SENTINEL_ID_1",
				Value: "d9b3204a90597a7500530efd881942d8145996ac", // TODO: Should this be hard-coded?
			},
			{
				Name:  "SENTINEL_ID_2",
				Value: "d9deb539c0402841c2492e9959c8086602fa4284", // TODO: Should this be hard-coded?
			},
		},
		Image:           "redis:5.0.3-alpine",
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
			Name: "probes",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDRedisProbesConfigMapName,
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

	if err := controllerutil.SetControllerReference(cr, ss, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), ss)
}

// reconcileStatefulSets will ensure that all StatefulSets are present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatefulSets(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileRedisStatefulSet(cr); err != nil {
		return nil
	}
	return nil
}
