package reposerver

import (
	"fmt"
	"reflect"
	"time"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	redisTLSPath         = "/app/config/reposerver/tls/redis"
	cmpServerPluginsPath = "/home/argocd/cmp-server/plugins"
)

func (rsr *RepoServerReconciler) reconcileDeployment() error {

	req := rsr.getDeploymentRequest()

	desired, err := workloads.RequestDeployment(req)
	if err != nil {
		return errors.Wrapf(err, "reconcileDeployment: failed to reconcile deployment %s", desired.Name)
	}

	if err = controllerutil.SetControllerReference(rsr.Instance, desired, rsr.Scheme); err != nil {
		rsr.Logger.Error(err, "reconcileDeployment: failed to set owner reference for deployment", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := workloads.GetDeployment(desired.Name, desired.Namespace, rsr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileDeployment: failed to retrieve deployment %s", desired.Name)
		}

		if err = workloads.CreateDeployment(desired, rsr.Client); err != nil {
			return errors.Wrapf(err, "reconcileDeployment: failed to create deployment %s in namespace %s", desired.Name, desired.Namespace)
		}
		rsr.Logger.Info("deployment created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	changed := false

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
		{Existing: &existing.Spec.Template.Spec.Containers[0].Resources, Desired: &desired.Spec.Template.Spec.Containers[0].Resources, ExtraAction: nil},
		{Existing: &existing.Spec.Template.Spec.Containers[0].VolumeMounts, Desired: &desired.Spec.Template.Spec.Containers[0].VolumeMounts, ExtraAction: nil},
		{Existing: &existing.Spec.Template.Spec.AutomountServiceAccountToken, Desired: &desired.Spec.Template.Spec.AutomountServiceAccountToken, ExtraAction: nil},
		{Existing: &existing.Spec.Template.Spec.ServiceAccountName, Desired: &desired.Spec.Template.Spec.ServiceAccountName, ExtraAction: func() {
			existing.Spec.Template.Spec.DeprecatedServiceAccount = desired.Spec.Template.Spec.ServiceAccountName
		},
		},
		{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
	}

	argocdcommon.UpdateIfChanged(fieldsToCompare, &changed)

	if !reflect.DeepEqual(desired.Spec.Template.Spec.Containers[1:], existing.Spec.Template.Spec.Containers[1:]) {
		existing.Spec.Template.Spec.Containers = append(existing.Spec.Template.Spec.Containers[0:1],
			desired.Spec.Template.Spec.Containers[1:]...)
		changed = true
	}

	if !changed {
		return nil
	}

	if err = workloads.UpdateDeployment(existing, rsr.Client); err != nil {
		return errors.Wrapf(err, "reconcileDeployment: failed to update deployment %s", existing.Name)
	}

	rsr.Logger.Info("deployment updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

func (rsr *RepoServerReconciler) getDeploymentRequest() workloads.DeploymentRequest {
	req := workloads.DeploymentRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, rsr.Instance.Namespace, rsr.Instance.Name, rsr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					common.AppK8sKeyName: resourceName,
				},
			},
			Replicas: rsr.getReplicas(),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						common.AppK8sKeyName: resourceName,
					},
				},
				Spec: corev1.PodSpec{
					Volumes:        rsr.getPodVolumes(),
					InitContainers: rsr.getRepoSeverInitContainers(),
					Containers:     rsr.getContainers(),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: util.BoolPtr(true),
					},
					AutomountServiceAccountToken: &rsr.Instance.Spec.Repo.MountSAToken,
					NodeSelector:                 common.DefaultNodeSelector(),
					ServiceAccountName:           resourceName,
				},
			},
		},
		Instance:  rsr.Instance,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Client:    rsr.Client,
	}

	if rsr.Instance.Spec.Repo.ServiceAccount != "" {
		req.Spec.Template.Spec.ServiceAccountName = rsr.Instance.Spec.Repo.ServiceAccount
	}

	if rsr.Instance.Spec.NodePlacement != nil {
		req.Spec.Template.Spec.NodeSelector = util.MergeMaps(req.Spec.Template.Spec.NodeSelector, rsr.Instance.Spec.NodePlacement.NodeSelector)
		req.Spec.Template.Spec.Tolerations = rsr.Instance.Spec.NodePlacement.Tolerations
	}

	return req
}

func (rsr *RepoServerReconciler) getRepoSeverInitContainers() []corev1.Container {
	initContainers := []corev1.Container{{
		Name:            "copyutil",
		Image:           rsr.getContainerImage(),
		Command:         argocdcommon.GetArgoCmpServerInitCommand(),
		ImagePullPolicy: corev1.PullAlways,
		Resources:       rsr.getResources(),
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
				Name:      "var-files",
				MountPath: "var/run/argocd",
			},
		},
	}}
	if (rsr.Instance.Spec.Repo.InitContainers != nil) && len(rsr.Instance.Spec.Repo.InitContainers) > 0 {
		initContainers = append(initContainers, rsr.Instance.Spec.Repo.InitContainers...)
	}
	return initContainers
}

func (rsr *RepoServerReconciler) getContainers() []corev1.Container {
	// Global proxy env vars go first
	repoServerEnv := rsr.Instance.Spec.Repo.Env
	// Environment specified in the CR take precedence over everything else
	repoServerEnv = util.EnvMerge(repoServerEnv, util.ProxyEnvVars(), false)

	if rsr.Instance.Spec.Repo.ExecTimeout != nil {
		repoServerEnv = util.EnvMerge(repoServerEnv, []corev1.EnvVar{{Name: common.ArgoCDExecTimeoutEnvVar, Value: fmt.Sprintf("%ds", *rsr.Instance.Spec.Repo.ExecTimeout)}}, true)
	}

	volumeMounts := []corev1.VolumeMount{
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
			MountPath: redisTLSPath,
		},
		{
			Name:      "plugins",
			MountPath: cmpServerPluginsPath,
		},
	}

	containers := []corev1.Container{{
		Command:         rsr.getArgs(),
		Image:           argocdcommon.GetArgoContainerImage(rsr.Instance),
		ImagePullPolicy: corev1.PullAlways,
		VolumeMounts:    volumeMounts,
		Name:            common.ArgoCDRepoServerName,
		Env:             repoServerEnv,
		Resources:       rsr.getResources(),
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(common.DefaultRepoServerPort),
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
				ContainerPort: common.DefaultRepoServerPort,
				Name:          "server",
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
	}}

	if rsr.Instance.Spec.Repo.VolumeMounts != nil {
		containers[0].VolumeMounts = append(volumeMounts, rsr.Instance.Spec.Repo.VolumeMounts...)
	}

	if rsr.Instance.Spec.Repo.SidecarContainers != nil {
		containers = append(containers, rsr.Instance.Spec.Repo.SidecarContainers...)
	}

	return containers
}

func (rsr *RepoServerReconciler) getPodVolumes() []corev1.Volume {
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
			Name: "var-files",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "plugins",
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

func (rsr *RepoServerReconciler) deleteDeployment(name, namespace string) error {
	if err := workloads.DeleteDeployment(name, namespace, rsr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteDeployment: failed to delete deployment %s in namespace %s", name, namespace)
	}
	rsr.Logger.Info("deployment deleted", "name", name, "namespace", namespace)
	return nil
}

// TriggerDeploymentRollout starts repo-server deployment rollout by updating the given key
func (rsr *RepoServerReconciler) TriggerDeploymentRollout(name, namespace, key string) error {
	return argocdcommon.TriggerDeploymentRollout(name, namespace, key, rsr.Client)
}
