package applicationset

import (
	"time"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/reposerver"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (asr *ApplicationSetReconciler) getDesiredDeployment() *appsv1.Deployment {
	desiredDeployment := &appsv1.Deployment{}

	appSetEnv := asr.Instance.Spec.ApplicationSet.Env
	appSetEnv = util.EnvMerge(appSetEnv, util.ProxyEnvVars(), false)

	objMeta := metav1.ObjectMeta{
		Name:      resourceName,
		Namespace: asr.Instance.Namespace,
		Labels:    resourceLabels,
	}
	addSCMGitlabVolumeMount := false
	if scmRootCAConfigMapName := asr.getSCMRootCAConfigMapName(); scmRootCAConfigMapName != "" {
		cm, err := workloads.GetConfigMap(scmRootCAConfigMapName, asr.Instance.Namespace, asr.Client)
		if err != nil {
			asr.Logger.Error(err, "failed to get SCM Root CA ConfigMap")
		}
		if util.IsObjectFound(asr.Client, asr.Instance.Namespace, asr.Instance.Spec.ApplicationSet.SCMRootCAConfigMap, cm) {
			addSCMGitlabVolumeMount = true
		}
	}
	podSpec := corev1.PodSpec{
		ServiceAccountName: resourceName,
		Volumes:            asr.getApplicationSetPodVolumes(addSCMGitlabVolumeMount),
		Containers:         []corev1.Container{*asr.getApplicationSetContainer(addSCMGitlabVolumeMount)},
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot: util.BoolPtr(true),
		},
	}

	deploymentSpec := appsv1.DeploymentSpec{
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
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
	}

	desiredDeployment.ObjectMeta = objMeta
	desiredDeployment.Spec = deploymentSpec
	return desiredDeployment
}

func (asr *ApplicationSetReconciler) getDeploymentRequest(dep appsv1.Deployment) workloads.DeploymentRequest {
	deploymentReq := workloads.DeploymentRequest{
		ObjectMeta: dep.ObjectMeta,
		Spec:       dep.Spec,
		Client:     asr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	return deploymentReq
}

func (asr *ApplicationSetReconciler) reconcileDeployment() error {

	asr.Logger.Info("reconciling deployment")

	desiredDeployment := asr.getDesiredDeployment()
	deploymentRequest := asr.getDeploymentRequest(*desiredDeployment)

	desiredDeployment, err := workloads.RequestDeployment(deploymentRequest)
	if err != nil {
		asr.Logger.Error(err, "reconcileDeployment: failed to request deployment", "name", desiredDeployment.Name, "namespace", desiredDeployment.Namespace)
		asr.Logger.V(1).Info("reconcileDeployment: one or more mutations could not be applied")
		return err
	}

	namespace, err := cluster.GetNamespace(asr.Instance.Namespace, asr.Client)
	if err != nil {
		asr.Logger.Error(err, "reconcileDeployment: failed to retrieve namespace", "name", asr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := asr.DeleteDeployment(desiredDeployment.Name, desiredDeployment.Namespace); err != nil {
			asr.Logger.Error(err, "reconcileDeployment: failed to delete deployment", "name", desiredDeployment.Name, "namespace", desiredDeployment.Namespace)
		}
		return err
	}

	existingDeployment, err := workloads.GetDeployment(desiredDeployment.Name, desiredDeployment.Namespace, asr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			asr.Logger.Error(err, "reconcileDeployment: failed to retrieve deployment", "name", existingDeployment.Name, "namespace", existingDeployment.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(asr.Instance, desiredDeployment, asr.Scheme); err != nil {
			asr.Logger.Error(err, "reconcileDeployment: failed to set owner reference for deployment", "name", desiredDeployment.Name, "namespace", desiredDeployment.Namespace)
		}

		if err = workloads.CreateDeployment(desiredDeployment, asr.Client); err != nil {
			asr.Logger.Error(err, "reconcileDeployment: failed to create deployment", "name", desiredDeployment.Name, "namespace", desiredDeployment.Namespace)
			return err
		}
		asr.Logger.V(0).Info("reconcileDeployment: deployment created", "name", desiredDeployment.Name, "namespace", desiredDeployment.Namespace)
		return nil
	}
	deploymentChanged := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{&existingDeployment.Spec.Template.Spec.Containers[0].Image, &desiredDeployment.Spec.Template.Spec.Containers[0].Image,
			func() {
				existingDeployment.Spec.Template.ObjectMeta.Labels[common.ImageUpgradedKey] = time.Now().UTC().Format(common.TimeFormatMST)
			},
		},
		{&existingDeployment.Spec.Template.Spec.Containers[0].Command, &desiredDeployment.Spec.Template.Spec.Containers[0].Command, nil},
		{&existingDeployment.Spec.Template.Spec.Containers[0].Env, &desiredDeployment.Spec.Template.Spec.Containers[0].Env, nil},
		{&existingDeployment.Spec.Template.Spec.Containers[0].Resources, &desiredDeployment.Spec.Template.Spec.Containers[0].Resources, nil},
		{&existingDeployment.Spec.Template.Spec.Volumes, &desiredDeployment.Spec.Template.Spec.Volumes, nil},
		{&existingDeployment.Spec.Template.Spec.NodeSelector, &desiredDeployment.Spec.Template.Spec.NodeSelector, nil},
		{&existingDeployment.Spec.Template.Spec.Tolerations, &desiredDeployment.Spec.Template.Spec.Tolerations, nil},
		{&existingDeployment.Spec.Template.Spec.ServiceAccountName, &desiredDeployment.Spec.Template.Spec.ServiceAccountName, nil},
		{&existingDeployment.Spec.Template.Labels, &desiredDeployment.Spec.Template.Labels, nil},
		{&existingDeployment.Spec.Replicas, &desiredDeployment.Spec.Replicas, nil},
		{&existingDeployment.Spec.Selector, &desiredDeployment.Spec.Selector, nil},
		{&existingDeployment.Labels, &desiredDeployment.Labels, nil},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &deploymentChanged)
	}

	if deploymentChanged {

		if err = workloads.UpdateDeployment(existingDeployment, asr.Client); err != nil {
			asr.Logger.Error(err, "reconcileDeployment: failed to update deployment", "name", existingDeployment.Name, "namespace", existingDeployment.Namespace)
			return err
		}
	}

	asr.Logger.V(0).Info("reconcileDeployment: deployment updated", "name", existingDeployment.Name, "namespace", existingDeployment.Namespace)
	return nil
}

func (asr *ApplicationSetReconciler) DeleteDeployment(name, namespace string) error {
	if err := workloads.DeleteDeployment(name, namespace, asr.Client); err != nil {
		asr.Logger.Error(err, "DeleteDeployment: failed to delete deployment", "name", name, "namespace", namespace)
		return err
	}
	asr.Logger.V(0).Info("DeleteDeployment: deployment deleted", "name", name, "namespace", namespace)
	return nil
}

func (asr *ApplicationSetReconciler) getArgoApplicationSetCommand() []string {
	cmd := make([]string, 0)

	cmd = append(cmd, common.EntryPointSh)
	cmd = append(cmd, ArgoCDApplicationSetController)

	cmd = append(cmd, common.ArgoCDRepoServer)
	cmd = append(cmd, reposerver.GetRepoServerAddress(resourceName, asr.Instance.Namespace))

	cmd = append(cmd, common.LogLevel)
	cmd = append(cmd, util.GetLogLevel(asr.Instance.Spec.ApplicationSet.LogLevel))

	// ApplicationSet command arguments provided by the user
	extraArgs := asr.Instance.Spec.ApplicationSet.ExtraCommandArgs
	err := isMergable(extraArgs, cmd)
	if err != nil {
		return cmd
	}

	cmd = append(cmd, extraArgs...)

	return cmd
}

func (asr *ApplicationSetReconciler) getApplicationSetContainer(addSCMGitlabVolumeMount bool) *corev1.Container {
	appSetEnv := asr.Instance.Spec.ApplicationSet.Env
	appSetEnv = util.EnvMerge(appSetEnv, util.ProxyEnvVars(), false)
	container := &corev1.Container{
		Command:         asr.getArgoApplicationSetCommand(),
		Image:           argocdcommon.GetArgoContainerImage(asr.Instance),
		ImagePullPolicy: corev1.PullAlways,
		Name:            ArgoCDApplicationSetController,
		Env:             appSetEnv,
		Resources:       asr.getApplicationSetResources(),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: util.BoolPtr(false),
			ReadOnlyRootFilesystem:   util.BoolPtr(true),
			RunAsNonRoot:             util.BoolPtr(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					common.CapabilityDropAll,
				},
			},
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
		},
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 7000,
				Name:          common.PortWebhook,
			},
			{
				ContainerPort: 8080,
				Name:          common.ArgoCDMetrics,
			},
		},
	}

	if addSCMGitlabVolumeMount {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      ApplicationSetGitlabSCMTlsCert,
			MountPath: ApplicationSetGitlabSCMTlsCertPath,
		})
	}

	return container
}

func (asr *ApplicationSetReconciler) getApplicationSetPodVolumes(addSCMGitlabVolumeMount bool) []corev1.Volume {
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
	}
	if addSCMGitlabVolumeMount {
		volumes = append(volumes, corev1.Volume{
			Name: ApplicationSetGitlabSCMTlsCert,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDAppSetGitlabSCMTLSCertsConfigMapName,
					},
				},
			},
		})
	}
	return volumes
}
