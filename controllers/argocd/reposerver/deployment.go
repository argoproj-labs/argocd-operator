package reposerver

import (
	"fmt"
	"reflect"
	"time"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (rsr *RepoServerReconciler) reconcileDeployment() error {

	rsr.Logger.Info("reconciling deployment")

	useTLSForRedis, err := argocdcommon.ShouldUseTLS(rsr.Client, rsr.Instance.Namespace)
	if err != nil {
		rsr.Logger.Error(err, "reconcileDeployment: failed to determine if TLS should be used for Redis")
		return err
	}

	desiredDeployment := rsr.getDesiredDeployment(useTLSForRedis)
	deploymentRequest := rsr.getDeploymentRequest(*desiredDeployment)

	desiredDeployment, err = workloads.RequestDeployment(deploymentRequest)
	if err != nil {
		rsr.Logger.Error(err, "reconcileDeployment: failed to request deployment", "name", desiredDeployment.Name, "namespace", desiredDeployment.Namespace)
		return err
	}

	namespace, err := cluster.GetNamespace(rsr.Instance.Namespace, rsr.Client)
	if err != nil {
		rsr.Logger.Error(err, "reconcileDeployment: failed to retrieve namespace", "name", rsr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := rsr.deleteDeployment(desiredDeployment.Name, desiredDeployment.Namespace); err != nil {
			rsr.Logger.Error(err, "reconcileDeployment: failed to delete deployment", "name", desiredDeployment.Name, "namespace", desiredDeployment.Namespace)
		}
		return err
	}

	existingDeployment, err := workloads.GetDeployment(desiredDeployment.Name, desiredDeployment.Namespace, rsr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			rsr.Logger.Error(err, "reconcileDeployment: failed to retrieve deployment", "name", desiredDeployment.Name, "namespace", desiredDeployment.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(rsr.Instance, desiredDeployment, rsr.Scheme); err != nil {
			rsr.Logger.Error(err, "reconcileDeployment: failed to set owner reference for deployment", "name", desiredDeployment.Name, "namespace", desiredDeployment.Namespace)
		}

		if err = workloads.CreateDeployment(desiredDeployment, rsr.Client); err != nil {
			rsr.Logger.Error(err, "reconcileDeployment: failed to create deployment", "name", desiredDeployment.Name, "namespace", desiredDeployment.Namespace)
			return err
		}
		rsr.Logger.V(0).Info("reconcileDeployment: deployment created", "name", desiredDeployment.Name, "namespace", desiredDeployment.Namespace)
		return nil
	}
	deploymentChanged := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{&existingDeployment.Spec.Template.Spec.Containers[0].Image, &desiredDeployment.Spec.Template.Spec.Containers[0].Image,
			func() {
				if existingDeployment.Spec.Template.ObjectMeta.Labels == nil {
					existingDeployment.Spec.Template.ObjectMeta.Labels = map[string]string{}
				}
				existingDeployment.Spec.Template.ObjectMeta.Labels[common.ImageUpgradedKey] = time.Now().UTC().Format(common.TimeFormatMST)
			},
		},
		{&existingDeployment.Spec.Template.Spec.NodeSelector, &desiredDeployment.Spec.Template.Spec.NodeSelector, nil},
		{&existingDeployment.Spec.Template.Spec.Tolerations, &desiredDeployment.Spec.Template.Spec.Tolerations, nil},
		{&existingDeployment.Spec.Template.Spec.Volumes, &desiredDeployment.Spec.Template.Spec.Volumes, nil},
		{&existingDeployment.Spec.Template.Spec.Containers[0].Command, &desiredDeployment.Spec.Template.Spec.Containers[0].Command, nil},
		{&existingDeployment.Spec.Template.Spec.Containers[0].Env, &desiredDeployment.Spec.Template.Spec.Containers[0].Env, nil},
		{&existingDeployment.Spec.Template.Spec.Containers[0].Resources, &desiredDeployment.Spec.Template.Spec.Containers[0].Resources, nil},
		{&existingDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, &desiredDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, nil},
		{&existingDeployment.Spec.Template.Spec.InitContainers, &desiredDeployment.Spec.Template.Spec.InitContainers, nil},
		{&existingDeployment.Spec.Template.Spec.AutomountServiceAccountToken, &desiredDeployment.Spec.Template.Spec.AutomountServiceAccountToken, nil},
		{&existingDeployment.Spec.Template.Spec.ServiceAccountName, &desiredDeployment.Spec.Template.Spec.ServiceAccountName, nil},
		{&existingDeployment.Spec.Replicas, &desiredDeployment.Spec.Replicas, nil},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &deploymentChanged)
	}

	if !reflect.DeepEqual(desiredDeployment.Spec.Template.Spec.Containers[1:], existingDeployment.Spec.Template.Spec.Containers[1:]) {
		existingDeployment.Spec.Template.Spec.Containers = append(existingDeployment.Spec.Template.Spec.Containers[0:1],
			desiredDeployment.Spec.Template.Spec.Containers[1:]...)
		deploymentChanged = true
	}

	if deploymentChanged {

		if err = workloads.UpdateDeployment(existingDeployment, rsr.Client); err != nil {
			rsr.Logger.Error(err, "reconcileDeployment: failed to update deployment", "name", existingDeployment.Name, "namespace", existingDeployment.Namespace)
			return err
		}
	}

	rsr.Logger.V(0).Info("reconcileDeployment: deployment updated", "name", existingDeployment.Name, "namespace", existingDeployment.Namespace)
	return nil
}

func (rsr *RepoServerReconciler) deleteDeployment(name, namespace string) error {
	if err := workloads.DeleteDeployment(name, namespace, rsr.Client); err != nil {
		rsr.Logger.Error(err, "DeleteDeployment: failed to delete deployment", "name", name, "namespace", namespace)
		return err
	}
	rsr.Logger.V(0).Info("DeleteDeployment: deployment deleted", "name", name, "namespace", namespace)
	return nil
}

func (rsr *RepoServerReconciler) getDesiredDeployment(useTLSForRedis bool) *appsv1.Deployment {
	desiredDeployment := &appsv1.Deployment{}

	automountToken := false
	if rsr.Instance.Spec.Repo.MountSAToken {
		automountToken = rsr.Instance.Spec.Repo.MountSAToken
	}

	objMeta := metav1.ObjectMeta{
		Name:      resourceName,
		Namespace: rsr.Instance.Namespace,
		Labels:    resourceLabels,
	}
	podSpec := corev1.PodSpec{
		Volumes:        rsr.getRepoServerPodVolumes(),
		InitContainers: rsr.getRepoSeverInitContainers(),
		Containers:     rsr.getRepoServerContainers(useTLSForRedis),
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot: util.BoolPtr(true),
		},
		AutomountServiceAccountToken: &automountToken,
		NodeSelector:                 common.DefaultNodeSelector(),
	}

	if rsr.Instance.Spec.NodePlacement != nil {
		podSpec.NodeSelector = util.AppendStringMap(podSpec.NodeSelector, rsr.Instance.Spec.NodePlacement.NodeSelector)
		podSpec.Tolerations = rsr.Instance.Spec.NodePlacement.Tolerations
	}

	if rsr.Instance.Spec.Repo.ServiceAccount != "" {
		podSpec.ServiceAccountName = rsr.Instance.Spec.Repo.ServiceAccount
	}

	deploymentSpec := appsv1.DeploymentSpec{
		Template: corev1.PodTemplateSpec{
			Spec: podSpec,
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					common.AppK8sKeyName: resourceName,
				},
			},
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				common.AppK8sKeyName: resourceName,
			},
		},
		Replicas: rsr.GetRepoServerReplicas(),
	}

	desiredDeployment.ObjectMeta = objMeta
	desiredDeployment.Spec = deploymentSpec
	return desiredDeployment
}

func (rsr *RepoServerReconciler) getDeploymentRequest(dep appsv1.Deployment) workloads.DeploymentRequest {
	deploymentReq := workloads.DeploymentRequest{
		ObjectMeta: dep.ObjectMeta,
		Spec:       dep.Spec,
		Client:     rsr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	return deploymentReq
}

func (rsr *RepoServerReconciler) getRepoSeverInitContainers() []corev1.Container {
	initContainers := []corev1.Container{{
		Name:            common.CopyUtil,
		Image:           argocdcommon.GetArgoContainerImage(rsr.Instance),
		Command:         argocdcommon.GetArgoCmpServerInitCommand(),
		ImagePullPolicy: corev1.PullAlways,
		Resources:       rsr.GetRepoServerResources(),
		Env:             util.ProxyEnvVars(),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: util.BoolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					common.CapabilityDropAll,
				},
			},
			RunAsNonRoot: util.BoolPtr(true),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      common.VolumeVarFiles,
				MountPath: common.VolumeMountPathVarRunArgocd,
			},
		},
	}}
	if (rsr.Instance.Spec.Repo.InitContainers != nil) && len(rsr.Instance.Spec.Repo.InitContainers) > 0 {
		initContainers = append(initContainers, rsr.Instance.Spec.Repo.InitContainers...)
	}
	return initContainers
}

func (rsr *RepoServerReconciler) getRepoServerContainers(useTLSForRedis bool) []corev1.Container {
	repoServerEnv := rsr.Instance.Spec.Repo.Env
	repoServerEnv = util.EnvMerge(repoServerEnv, util.ProxyEnvVars(), false)

	if rsr.Instance.Spec.Repo.ExecTimeout != nil {
		repoServerEnv = util.EnvMerge(repoServerEnv, []corev1.EnvVar{{Name: common.ArgoCDExecTimeoutEnvVar, Value: fmt.Sprintf("%ds", *rsr.Instance.Spec.Repo.ExecTimeout)}}, true)
	}

	containers := []corev1.Container{{
		Command:         rsr.GetRepoServerCommand(useTLSForRedis),
		Image:           argocdcommon.GetArgoContainerImage(rsr.Instance),
		ImagePullPolicy: corev1.PullAlways,
		Name:            common.RepoServerController,
		Env:             repoServerEnv,
		Resources:       rsr.GetRepoServerResources(),
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(common.ArgoCDDefaultRepoServerPort),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(common.ArgoCDDefaultRepoServerPort),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultRepoServerPort,
				Name:          common.Server,
			}, {
				ContainerPort: common.ArgoCDDefaultRepoMetricsPort,
				Name:          common.ArgoCDMetrics,
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: util.BoolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					common.CapabilityDropAll,
				},
			},
			RunAsNonRoot: util.BoolPtr(true),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      common.SSHKnownHosts,
				MountPath: common.VolumeMountPathSSH,
			},
			{
				Name:      common.TLSCerts,
				MountPath: common.VolumeMountPathTLS,
			},
			{
				Name:      common.GPGKeys,
				MountPath: common.VolumeMountPathGPG,
			},
			{
				Name:      common.GPGKeyRing,
				MountPath: common.VolumeMountPathGPGKeyring,
			},
			{
				Name:      common.VolumeTmp,
				MountPath: common.VolumeMountPathTmp,
			},
			{
				Name:      common.ArgoCDRepoServerTLSSecretName,
				MountPath: common.VolumeMountPathRepoServerTLS,
			},
			{
				Name:      common.ArgoCDRedisServerTLSSecretName,
				MountPath: common.VolumeMountPathRepoServerTLSRedis,
			},
			{
				Name:      common.VolumeMountPlugins,
				MountPath: common.VolumeMountPathPlugins,
			},
		},
	}}
	if rsr.Instance.Spec.Repo.VolumeMounts != nil {
		containers[0].VolumeMounts = append(containers[0].VolumeMounts, rsr.Instance.Spec.Repo.VolumeMounts...)
	}

	if rsr.Instance.Spec.Repo.SidecarContainers != nil {
		containers = append(containers, rsr.Instance.Spec.Repo.SidecarContainers...)
	}

	return containers
}

func (rsr *RepoServerReconciler) getRepoServerPodVolumes() []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: common.SSHKnownHosts,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDKnownHostsConfigMapName,
					},
				},
			},
		},
		{
			Name: common.TLSCerts,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDTLSCertsConfigMapName,
					},
				},
			},
		},
		{
			Name: common.GPGKeys,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDGPGKeysConfigMapName,
					},
				},
			},
		},
		{
			Name: common.GPGKeyRing,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: common.VolumeTmp,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: common.ArgoCDRepoServerTLS,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRepoServerTLSSecretName,
					Optional:   util.BoolPtr(true),
				},
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
		{
			Name: common.VolumeVarFiles,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: common.VolumeMountPlugins,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	if rsr.Instance.Spec.Repo.Volumes != nil && len(rsr.Instance.Spec.Repo.Volumes) > 0 {
		volumes = append(volumes, rsr.Instance.Spec.Repo.Volumes...)
	}
	return volumes
}
