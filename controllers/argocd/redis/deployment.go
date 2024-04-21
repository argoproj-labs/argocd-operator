package redis

import (
	"time"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	haProxyInitShPath = "/readonly/haproxy_init.sh"
)

func (rr *RedisReconciler) reconcileDeployment() error {
	req := rr.getDeploymentRequest()

	ignoreDrift := false
	updateFn := func(existing, desired *appsv1.Deployment, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Spec.Template.Spec.Containers[0].Image, Desired: &desired.Spec.Template.Spec.Containers[0].Image,
				ExtraAction: func() {
					if existing.Spec.Template.ObjectMeta.Labels == nil {
						existing.Spec.Template.ObjectMeta.Labels = map[string]string{}
					}
					existing.Spec.Template.ObjectMeta.Labels[common.ImageUpgradedKey] = time.Now().UTC().Format(common.TimeFormatMST)
				},
			},
			{Existing: &existing.Spec.Template.Spec.NodeSelector, Desired: &desired.Spec.Template.Spec.NodeSelector, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Tolerations, Desired: &desired.Spec.Template.Spec.Tolerations, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Command, Desired: &desired.Spec.Template.Spec.Containers[0].Command, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Args, Desired: &desired.Spec.Template.Spec.Containers[0].Args, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Resources, Desired: &desired.Spec.Template.Spec.Containers[0].Resources, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].VolumeMounts, Desired: &desired.Spec.Template.Spec.Containers[0].VolumeMounts, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.AutomountServiceAccountToken, Desired: &desired.Spec.Template.Spec.AutomountServiceAccountToken, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.ServiceAccountName, Desired: &desired.Spec.Template.Spec.ServiceAccountName, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.SecurityContext, Desired: &desired.Spec.Template.Spec.SecurityContext, ExtraAction: nil},
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			//
		}

		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}

	return rr.reconDeployment(req, argocdcommon.UpdateFnDep(updateFn), ignoreDrift)
}

func (rr *RedisReconciler) reconcileHAProxyDeployment() error {
	req := rr.getHAProxyDeploymentRequest()

	ignoreDrift := false
	updateFn := func(existing, desired *appsv1.Deployment, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Spec.Template.Spec.Containers[0].Image, Desired: &desired.Spec.Template.Spec.Containers[0].Image,
				ExtraAction: func() {
					if existing.Spec.Template.ObjectMeta.Labels == nil {
						existing.Spec.Template.ObjectMeta.Labels = map[string]string{}
					}
					existing.Spec.Template.ObjectMeta.Labels[common.ImageUpgradedKey] = time.Now().UTC().Format(common.TimeFormatMST)
				},
			},
			{Existing: &existing.Spec.Template.Spec.NodeSelector, Desired: &desired.Spec.Template.Spec.NodeSelector, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Tolerations, Desired: &desired.Spec.Template.Spec.Tolerations, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Command, Desired: &desired.Spec.Template.Spec.Containers[0].Command, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Args, Desired: &desired.Spec.Template.Spec.Containers[0].Args, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Env, Desired: &desired.Spec.Template.Spec.Containers[0].Env, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Resources, Desired: &desired.Spec.Template.Spec.Containers[0].Resources, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].VolumeMounts, Desired: &desired.Spec.Template.Spec.Containers[0].VolumeMounts, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.InitContainers, Desired: &desired.Spec.Template.Spec.InitContainers, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.AutomountServiceAccountToken, Desired: &desired.Spec.Template.Spec.AutomountServiceAccountToken, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.ServiceAccountName, Desired: &desired.Spec.Template.Spec.ServiceAccountName, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.SecurityContext, Desired: &desired.Spec.Template.Spec.SecurityContext, ExtraAction: nil},
			// {Existing: &existing.Spec.Replicas, Desired: &desired.Spec.Replicas, ExtraAction: nil},
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
		}

		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}
	return rr.reconDeployment(req, argocdcommon.UpdateFnDep(updateFn), ignoreDrift)
}

func (rr *RedisReconciler) reconDeployment(req workloads.DeploymentRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := workloads.RequestDeployment(req)
	if err != nil {
		rr.Logger.Debug("reconDeployment: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconDeployment: failed to request Deployment %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desired, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconDeployment: failed to set owner reference for Deployment", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := workloads.GetDeployment(desired.Name, desired.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconDeployment: failed to retrieve Deployment %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = workloads.CreateDeployment(desired, rr.Client); err != nil {
			return errors.Wrapf(err, "reconDeployment: failed to create Deployment %s in namespace %s", desired.Name, desired.Namespace)
		}
		rr.Logger.Info("Deployment created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// Deployment found, no update required - nothing to do
	if ignoreDrift {
		return nil
	}

	changed := false

	// execute supplied update function
	if updateFn != nil {
		if fn, ok := updateFn.(argocdcommon.UpdateFnDep); ok {
			if err := fn(existing, desired, &changed); err != nil {
				return errors.Wrapf(err, "reconDeployment: failed to execute update function for %s in namespace %s", existing.Name, existing.Namespace)
			}
		}
	}

	if !changed {
		return nil
	}

	if err = workloads.UpdateDeployment(existing, rr.Client); err != nil {
		return errors.Wrapf(err, "reconDeployment: failed to update Deployment %s", existing.Name)
	}

	rr.Logger.Info("Deployment updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

func (rr *RedisReconciler) deleteDeployment(name, namespace string) error {
	if err := workloads.DeleteDeployment(name, namespace, rr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteDeployment: failed to delete deployment %s", name)
	}
	rr.Logger.Info("deployment deleted", "name", name, "namespace", namespace)
	return nil
}

// TriggerDeploymentRollout starts redis deployment rollout by updating the given key
func (rr *RedisReconciler) TriggerDeploymentRollout(name, namespace, key string) error {
	err := argocdcommon.TriggerDeploymentRollout(name, namespace, key, rr.Client)
	if err != nil {
		return errors.Wrapf(err, "TriggerDeploymentRollout: failed to rollout deployment %s in namespace %s", name, namespace)
	}
	return nil
}

func (rr *RedisReconciler) getDeploymentRequest() workloads.DeploymentRequest {
	req := workloads.DeploymentRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					common.AppK8sKeyName: resourceName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						common.AppK8sKeyName: resourceName,
					},
				},
				Spec: corev1.PodSpec{
					NodeSelector:       common.DefaultNodeSelector(),
					Containers:         rr.getDeploymentContainers(),
					ServiceAccountName: resourceName,
					Volumes: []corev1.Volume{
						{
							Name: common.ArgoCDRedisServerTLSSecretName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: common.ArgoCDRedisServerTLSSecretName,
									Optional:   util.BoolPtr(true),
								},
							},
						},
					},
				},
			},
		},
		Instance:  rr.Instance,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Client:    rr.Client,
	}

	return req
}

func (rr *RedisReconciler) getDeploymentContainers() []corev1.Container {
	containers := []corev1.Container{}

	redisContainer := corev1.Container{
		Args:            rr.getCmd(),
		Image:           rr.getContainerImage(),
		ImagePullPolicy: corev1.PullAlways,
		Name:            redisName,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.DefaultRedisPort,
			},
		},
		Resources: rr.getResources(),
		Env:       util.ProxyEnvVars(),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: util.BoolPtr(allowPrivilegeEscalation),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			RunAsNonRoot: util.BoolPtr(runAsNonRoot),
			RunAsUser:    util.Int64Ptr(999),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      common.ArgoCDRedisServerTLSSecretName,
				MountPath: TLSPath,
			},
		},
	}

	containers = append(containers, redisContainer)
	return containers
}

func (rr *RedisReconciler) getHAProxyDeploymentRequest() workloads.DeploymentRequest {
	depReq := workloads.DeploymentRequest{
		ObjectMeta: argoutil.GetObjMeta(HAProxyResourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					common.AppK8sKeyName: HAProxyResourceName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						common.AppK8sKeyName: HAProxyResourceName,
					},
				},
				Spec: corev1.PodSpec{
					NodeSelector:   common.DefaultNodeSelector(),
					InitContainers: rr.getHAProxyDeploymentInitContainers(),
					Containers:     rr.getHAProxyDeploymentContainers(),
				},
			},
		},
		Instance:  rr.Instance,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Client:    rr.Client,
	}

	depReq.Spec.Template.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								common.AppK8sKeyName: HAProxyResourceName,
							},
						},
						TopologyKey: common.FailureDomainBetaK8sKeyZone,
					},
					Weight: int32(100),
				},
			},
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.AppK8sKeyName: HAProxyResourceName,
						},
					},
					TopologyKey: common.K8sKeyHostname,
				},
			},
		},
	}

	depReq.Spec.Template.Spec.Volumes = []corev1.Volume{
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
		{
			Name: common.ArgoCDRedisServerTLSSecretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRedisServerTLSSecretName,
					Optional:   util.BoolPtr(true),
				},
			},
		},
	}

	depReq.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
		RunAsNonRoot: util.BoolPtr(runAsNonRoot),
		RunAsUser:    util.Int64Ptr(runAsUser),
		FSGroup:      util.Int64Ptr(fsGroup),
	}

	depReq.Spec.Template.Spec.ServiceAccountName = resourceName

	return depReq
}

func (rr *RedisReconciler) getHAProxyDeploymentContainers() []corev1.Container {
	containers := []corev1.Container{}

	haproxyContainer := corev1.Container{
		Image:           rr.getHAProxyContainerImage(),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Name:            common.HAProxyName,
		Env:             util.ProxyEnvVars(),
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
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
				ContainerPort: common.DefaultRedisPort,
				Name:          redisName,
			},
		},
		Resources: rr.getHAResources(),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: util.BoolPtr(allowPrivilegeEscalation),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			RunAsNonRoot: util.BoolPtr(runAsNonRoot),
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
			{
				Name:      common.ArgoCDRedisServerTLSSecretName,
				MountPath: TLSPath,
			},
		},
	}

	containers = append(containers, haproxyContainer)
	return containers
}

func (rr *RedisReconciler) getHAProxyDeploymentInitContainers() []corev1.Container {
	containers := []corev1.Container{}

	initc := corev1.Container{
		Args: []string{
			haProxyInitShPath,
		},
		Command: []string{
			"sh",
		},
		Image:           rr.getHAProxyContainerImage(),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Name:            "config-init",
		Env:             util.ProxyEnvVars(),
		Resources:       rr.getHAResources(),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: util.BoolPtr(allowPrivilegeEscalation),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
		},
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
	}

	containers = append(containers, initc)
	return containers
}
