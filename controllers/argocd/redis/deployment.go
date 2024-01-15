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
	deployReq := rr.getDeploymentRequest()

	desiredDeploy, err := workloads.RequestDeployment(deployReq)
	if err != nil {
		return errors.Wrapf(err, "reconcileDeployment: failed to reconcile deployment %s", desiredDeploy.Name)
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desiredDeploy, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileDeployment: failed to set owner reference for deployment", "name", desiredDeploy.Name, "namespace", desiredDeploy.Namespace)
	}

	existingDeploy, err := workloads.GetDeployment(desiredDeploy.Name, desiredDeploy.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileDeployment: failed to retrieve deployment %s", desiredDeploy.Name)
		}

		if err = workloads.CreateDeployment(desiredDeploy, rr.Client); err != nil {
			return errors.Wrapf(err, "reconcileDeployment: failed to create deployment %s in namespace %s", desiredDeploy.Name, desiredDeploy.Namespace)
		}
		rr.Logger.V(0).Info("reconcileDeployment: deployment created", "name", desiredDeploy.Name, "namespace", desiredDeploy.Namespace)
		return nil
	}

	deployChanged := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{&existingDeploy.Spec.Template.Spec.Containers[0].Image, &desiredDeploy.Spec.Template.Spec.Containers[0].Image,
			func() {
				existingDeploy.Spec.Template.ObjectMeta.Labels[common.ImageUpgradedKey] = time.Now().UTC().Format(common.TimeFormatMST)
			},
		},
		{&existingDeploy.Spec.Template.Spec, &desiredDeploy.Spec.Template.Spec, nil},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &deployChanged)
	}

	if !deployChanged {
		return nil
	}

	if err = workloads.UpdateDeployment(existingDeploy, rr.Client); err != nil {
		return errors.Wrapf(err, "reconcileDeployment: failed to update deployment %s", existingDeploy.Name)
	}

	rr.Logger.V(0).Info("deployment updated", "name", existingDeploy.Name, "namespace", existingDeploy.Namespace)
	return nil
}

func (rr *RedisReconciler) reconcileHADeployment() error {
	deployReq := rr.getHAProxyDeploymentRequest()

	desiredDeploy, err := workloads.RequestDeployment(deployReq)
	if err != nil {
		return errors.Wrapf(err, "reconcileHADeployment: failed to reconcile deployment %s", desiredDeploy.Name)
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desiredDeploy, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileHADeployment: failed to set owner reference for deployment", "name", desiredDeploy.Name, "namespace", desiredDeploy.Namespace)
	}

	existingDeploy, err := workloads.GetDeployment(desiredDeploy.Name, desiredDeploy.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileHADeployment: failed to retrieve deployment %s", desiredDeploy.Name)
		}

		if err = workloads.CreateDeployment(desiredDeploy, rr.Client); err != nil {
			return errors.Wrapf(err, "reconcileHADeployment: failed to create deployment %s in namespace %s", desiredDeploy.Name, desiredDeploy.Namespace)
		}
		rr.Logger.V(0).Info("reconcileHADeployment: deployment created", "name", desiredDeploy.Name, "namespace", desiredDeploy.Namespace)
		return nil
	}

	deployChanged := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{&existingDeploy.Spec.Template.Spec.Containers[0].Image, &desiredDeploy.Spec.Template.Spec.Containers[0].Image,
			func() {
				existingDeploy.Spec.Template.ObjectMeta.Labels[common.ImageUpgradedKey] = time.Now().UTC().Format(common.TimeFormatMST)
			},
		},
		{&existingDeploy.Spec.Template.Spec, &desiredDeploy.Spec.Template.Spec, nil},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &deployChanged)
	}

	if !deployChanged {
		return nil
	}

	if err = workloads.UpdateDeployment(existingDeploy, rr.Client); err != nil {
		return errors.Wrapf(err, "reconcileHADeployment: failed to update deployment %s", existingDeploy.Name)
	}

	rr.Logger.V(0).Info("deployment updated", "name", existingDeploy.Name, "namespace", existingDeploy.Namespace)
	return nil
}

func (rr *RedisReconciler) getDeploymentRequest() workloads.DeploymentRequest {
	depReq := workloads.DeploymentRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component),
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
	}

	return depReq
}

func (rr *RedisReconciler) getDeploymentContainers() []corev1.Container {
	containers := []corev1.Container{}

	redisContainer := corev1.Container{
		Args:            rr.getArgs(),
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
		ObjectMeta: argoutil.GetObjMeta(HAProxyResourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component),
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

// TriggerDeploymentRollout starts redis deployment rollout by updating the given key
func (rr *RedisReconciler) TriggerDeploymentRollout(name, namespace, key string) error {
	return argocdcommon.TriggerDeploymentRollout(name, namespace, key, rr.Client)
}

func (rr *RedisReconciler) deleteDeployment(name, namespace string) error {
	if err := workloads.DeleteDeployment(name, namespace, rr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteDeployment: failed to delete deployment %s", name)
	}
	rr.Logger.V(0).Info("deleteDeployment: deployment deleted", "name", name, "namespace", namespace)
	return nil
}
