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
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
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
			Labels:    argoutil.LabelsForCluster(cr),
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

	ss.Spec = appsv1.StatefulSetSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				common.ArgoCDKeyName: name,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					common.ArgoCDKeyName: name,
				},
			},
		},
	}
	if cr.Spec.NodePlacement != nil {
		ss.Spec.Template.Spec.NodeSelector = cr.Spec.NodePlacement.NodeSelector
		ss.Spec.Template.Spec.Tolerations = cr.Spec.NodePlacement.Tolerations
	}
	ss.Spec.ServiceName = name

	return ss
}

// newStatefulSetWithSuffix returns a new StatefulSet instance for the given ArgoCD using the given suffix.
func newStatefulSetWithSuffix(suffix string, component string, cr *argoprojv1a1.ArgoCD) *appsv1.StatefulSet {
	return newStatefulSetWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), component, cr)
}

func (r *ReconcileArgoCD) reconcileRedisStatefulSet(cr *argoprojv1a1.ArgoCD) error {
	ss := newStatefulSetWithSuffix("redis-ha-server", "redis", cr)

	existing := newStatefulSetWithSuffix("redis-ha-server", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		if !cr.Spec.HA.Enabled {
			// StatefulSet exists but HA enabled flag has been set to false, delete the StatefulSet
			return r.Client.Delete(context.TODO(), existing)
		}

		desiredImage := getRedisHAContainerImage(cr)
		changed := false
		updateNodePlacementStateful(existing, ss, &changed)
		for i, container := range existing.Spec.Template.Spec.Containers {
			if container.Image != desiredImage {
				existing.Spec.Template.Spec.Containers[i].Image = getRedisHAContainerImage(cr)
				existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
				changed = true
			}
		}

		if changed {
			return r.Client.Update(context.TODO(), existing)
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
			"checksum/init-config": "552ee3bec8fe5d9d865e371f7b615c6d472253649eb65d53ed4ae874f782647c", // TODO: Should this be hard-coded?
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

	ss.Spec.Template.Spec.ServiceAccountName = nameWithSuffix("argocd-redis-ha", cr)

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

	if err := applyReconcilerHook(cr, ss, ""); err != nil {
		return err
	}

	if err := controllerutil.SetControllerReference(cr, ss, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ss)
}

func getArgoControllerContainerEnv(cr *argoprojv1a1.ArgoCD) []corev1.EnvVar {
	env := make([]corev1.EnvVar, 0)

	if cr.Spec.Controller.Sharding.Enabled {
		env = append(env, corev1.EnvVar{
			Name:  "ARGOCD_CONTROLLER_REPLICAS",
			Value: fmt.Sprint(cr.Spec.Controller.Sharding.Replicas),
		})
	}

	return env
}

func (r *ReconcileArgoCD) reconcileApplicationControllerStatefulSet(cr *argoprojv1a1.ArgoCD) error {
	var replicas int32 = common.ArgocdApplicationControllerDefaultReplicas

	if cr.Spec.Controller.Sharding.Replicas != 0 && cr.Spec.Controller.Sharding.Enabled {
		replicas = cr.Spec.Controller.Sharding.Replicas
	}

	ss := newStatefulSetWithSuffix("application-controller", "application-controller", cr)
	ss.Spec.Replicas = &replicas

	podSpec := &ss.Spec.Template.Spec
	podSpec.Containers = []corev1.Container{{
		Command:         getArgoApplicationControllerCommand(cr),
		Image:           getArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "argocd-application-controller",
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8082),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		Env: proxyEnvVars(getArgoControllerContainerEnv(cr)...),
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 8082,
			},
		},
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8082),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		Resources: getArgoApplicationControllerResources(cr),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "argocd-repo-server-tls",
				MountPath: "/app/config/controller/tls",
			},
		},
	}}
	podSpec.ServiceAccountName = nameWithSuffix("argocd-application-controller", cr)
	podSpec.Volumes = []corev1.Volume{
		{
			Name: "argocd-repo-server-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRepoServerTLSSecretName,
					Optional:   boolPtr(true),
				},
			},
		},
	}

	ss.Spec.Template.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.ArgoCDKeyName: nameWithSuffix("argocd-application-controller", cr),
						},
					},
					TopologyKey: common.ArgoCDKeyHostname,
				},
				Weight: int32(100),
			},
				{
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								common.ArgoCDKeyPartOf: common.ArgoCDAppName,
							},
						},
						TopologyKey: common.ArgoCDKeyHostname,
					},
					Weight: int32(5),
				}},
		},
	}

	// Handle import/restore from ArgoCDExport
	export := r.getArgoCDExport(cr)
	if export == nil {
		log.Info("existing argocd export not found, skipping import")
	} else {
		podSpec.InitContainers = []corev1.Container{{
			Command:         getArgoImportCommand(r.Client, cr),
			Env:             proxyEnvVars(getArgoImportContainerEnv(export)...),
			Resources:       getArgoApplicationControllerResources(cr),
			Image:           getArgoImportContainerImage(export),
			ImagePullPolicy: corev1.PullAlways,
			Name:            "argocd-import",
			VolumeMounts:    getArgoImportVolumeMounts(export),
		}}

		podSpec.Volumes = getArgoImportVolumes(export)
	}

	existing := newStatefulSetWithSuffix("application-controller", "application-controller", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getArgoContainerImage(cr)
		changed := false
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}
		desiredCommand := getArgoApplicationControllerCommand(cr)
		if isRepoServerTLSVerificationRequested(cr) {
			desiredCommand = append(desiredCommand, "--repo-server-strict-tls")
		}
		updateNodePlacementStateful(existing, ss, &changed)
		if !reflect.DeepEqual(desiredCommand, existing.Spec.Template.Spec.Containers[0].Command) {
			existing.Spec.Template.Spec.Containers[0].Command = desiredCommand
			changed = true
		}

		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			ss.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = ss.Spec.Template.Spec.Containers[0].Env
			changed = true
		}
		if !reflect.DeepEqual(ss.Spec.Template.Spec.Volumes, existing.Spec.Template.Spec.Volumes) {
			existing.Spec.Template.Spec.Volumes = ss.Spec.Template.Spec.Volumes
			changed = true
		}
		if !reflect.DeepEqual(ss.Spec.Template.Spec.Containers[0].VolumeMounts,
			existing.Spec.Template.Spec.Containers[0].VolumeMounts) {
			existing.Spec.Template.Spec.Containers[0].VolumeMounts = ss.Spec.Template.Spec.Containers[0].VolumeMounts
			changed = true
		}
		if !reflect.DeepEqual(ss.Spec.Template.Spec.Containers[0].Resources, existing.Spec.Template.Spec.Containers[0].Resources) {
			existing.Spec.Template.Spec.Containers[0].Resources = ss.Spec.Template.Spec.Containers[0].Resources
			changed = true
		}
		if !reflect.DeepEqual(ss.Spec.Replicas, existing.Spec.Replicas) {
			existing.Spec.Replicas = ss.Spec.Replicas
			changed = true
		}

		if changed {
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // StatefulSet found with nothing to do, move along...
	}

	// Delete existing deployment for Application Controller, if any ..
	deploy := newDeploymentWithSuffix("application-controller", "application-controller", cr)
	if argoutil.IsObjectFound(r.Client, deploy.Namespace, deploy.Name, deploy) {
		if err := r.Client.Delete(context.TODO(), deploy); err != nil {
			return err
		}
	}

	if err := controllerutil.SetControllerReference(cr, ss, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ss)
}

// reconcileStatefulSets will ensure that all StatefulSets are present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatefulSets(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileApplicationControllerStatefulSet(cr); err != nil {
		return err
	}
	if err := r.reconcileRedisStatefulSet(cr); err != nil {
		return err
	}
	return nil
}

// triggerStatefulSetRollout will update the label with the given key to trigger a new rollout of the StatefulSet.
func (r *ReconcileArgoCD) triggerStatefulSetRollout(sts *appsv1.StatefulSet, key string) error {
	if !argoutil.IsObjectFound(r.Client, sts.Namespace, sts.Name, sts) {
		log.Info(fmt.Sprintf("unable to locate deployment with name: %s", sts.Name))
		return nil
	}

	sts.Spec.Template.ObjectMeta.Labels[key] = nowNano()
	return r.Client.Update(context.TODO(), sts)
}

//to update nodeSelector and tolerations in reconciler
func updateNodePlacementStateful(existing *appsv1.StatefulSet, ss *appsv1.StatefulSet, changed *bool) {
	if !reflect.DeepEqual(existing.Spec.Template.Spec.NodeSelector, ss.Spec.Template.Spec.NodeSelector) {
		existing.Spec.Template.Spec.NodeSelector = ss.Spec.Template.Spec.NodeSelector
		*changed = true
	}
	if !reflect.DeepEqual(existing.Spec.Template.Spec.Tolerations, ss.Spec.Template.Spec.Tolerations) {
		existing.Spec.Template.Spec.Tolerations = ss.Spec.Template.Spec.Tolerations
		*changed = true
	}
}
