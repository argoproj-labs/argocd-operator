package applicationset

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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (asr *ApplicationSetReconciler) reconcileDeployment() error {
	req := asr.getDeploymentRequest()

	ignoreDrift := false
	updateFn := func(existing, desired *appsv1.Deployment, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Spec.Template.Spec.Containers[0].Image, Desired: &desired.Spec.Template.Spec.Containers[0].Image,
				ExtraAction: func() {
					existing.Spec.Template.ObjectMeta.Labels[common.ImageUpgradedKey] = time.Now().UTC().Format(common.TimeFormatMST)
				},
			},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Command, Desired: &desired.Spec.Template.Spec.Containers[0].Command, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Volumes, Desired: &desired.Spec.Template.Spec.Volumes, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.NodeSelector, Desired: &desired.Spec.Template.Spec.NodeSelector, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Tolerations, Desired: &desired.Spec.Template.Spec.Tolerations, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.ServiceAccountName, Desired: &desired.Spec.Template.Spec.ServiceAccountName, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Labels, Desired: &desired.Spec.Template.Labels, ExtraAction: nil},
			{Existing: &existing.Spec.Replicas, Desired: &desired.Spec.Replicas, ExtraAction: nil},
			{Existing: &existing.Spec.Selector, Desired: &desired.Spec.Selector, ExtraAction: nil},
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
		}
		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}

	return asr.reconDeployment(req, argocdcommon.UpdateFnDep(updateFn), ignoreDrift)
}

func (asr *ApplicationSetReconciler) reconDeployment(req workloads.DeploymentRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := workloads.RequestDeployment(req)
	if err != nil {
		asr.Logger.Debug("reconDeployment: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconDeployment: failed to request Deployment %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(asr.Instance, desired, asr.Scheme); err != nil {
		asr.Logger.Error(err, "reconDeployment: failed to set owner reference for Deployment", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := workloads.GetDeployment(desired.Name, desired.Namespace, asr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconDeployment: failed to retrieve Deployment %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = workloads.CreateDeployment(desired, asr.Client); err != nil {
			return errors.Wrapf(err, "reconDeployment: failed to create Deployment %s in namespace %s", desired.Name, desired.Namespace)
		}
		asr.Logger.Info("Deployment created", "name", desired.Name, "namespace", desired.Namespace)
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

	if err = workloads.UpdateDeployment(existing, asr.Client); err != nil {
		return errors.Wrapf(err, "reconDeployment: failed to update Deployment %s", existing.Name)
	}

	asr.Logger.Info("Deployment updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteDeployment will delete deployment with given name.
func (asr *ApplicationSetReconciler) deleteDeployment(name, namespace string) error {
	if err := workloads.DeleteDeployment(name, namespace, asr.Client); err != nil {
		// resource is already deleted, ignore error
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteDeployment: failed to delete deployment %s in namespace %s", name, namespace)
	}
	asr.Logger.Info("deployment deleted", "name", name, "namespace", namespace)
	return nil
}

func (asr *ApplicationSetReconciler) getdesired() *appsv1.Deployment {
	desired := &appsv1.Deployment{}

	objMeta := metav1.ObjectMeta{
		Name:      resourceName,
		Namespace: asr.Instance.Namespace,
	}
	addSCMGitlabVolumeMount := false
	if scmRootCAConfigMapName := asr.getSCMRootCAConfigMapName(); scmRootCAConfigMapName != "" {
		cm, err := workloads.GetConfigMap(scmRootCAConfigMapName, asr.Instance.Namespace, asr.Client)
		if err != nil {
			asr.Logger.Error(err, "failed to get SCM Root CA ConfigMap")
		}
		if argoutil.IsObjectFound(asr.Client, asr.Instance.Namespace, asr.Instance.Spec.ApplicationSet.SCMRootCAConfigMap, cm) {
			addSCMGitlabVolumeMount = true
		}
	}
	podSpec := corev1.PodSpec{
		ServiceAccountName: resourceName,
		Volumes:            asr.getPodVolumes(addSCMGitlabVolumeMount),
		Containers:         []corev1.Container{*asr.getContainer(addSCMGitlabVolumeMount)},
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

	desired.ObjectMeta = objMeta
	desired.Spec = deploymentSpec
	return desired
}

func (asr *ApplicationSetReconciler) getDeploymentRequest() workloads.DeploymentRequest {
	req := workloads.DeploymentRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, asr.Instance.Namespace, asr.Instance.Name, asr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Client:     asr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	podSpec := corev1.PodSpec{
		ServiceAccountName: resourceName,
		Volumes: []corev1.Volume{
			{
				Name: "ssh-known-hosts",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: common.ArgoCDKnownHostsConfigMapName,
						},
					},
				},
			},
			{
				Name: "tls-certs",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: common.ArgoCDTLSCertsConfigMapName,
						},
					},
				},
			},
			{
				Name: "gpg-keys",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: common.ArgoCDGPGKeysConfigMapName,
						},
					},
				},
			},
			{
				Name: "gpg-keyring",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			{
				Name: "tmp",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
	}

	addSCMGitlabVolumeMount := false
	if scmRootCAConfigMapName := asr.getSCMRootCAConfigMapName(); scmRootCAConfigMapName != "" {
		cm, err := workloads.GetConfigMap(scmRootCAConfigMapName, asr.Instance.Namespace, asr.Client)
		if err != nil {
			asr.Logger.Error(err, "failed to get SCM Root CA ConfigMap")
		}
		if argoutil.IsObjectFound(asr.Client, asr.Instance.Namespace, asr.Instance.Spec.ApplicationSet.SCMRootCAConfigMap, cm) {
			addSCMGitlabVolumeMount = true
			podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
				Name: "appset-gitlab-scm-tls-cert",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: common.ArgoCDAppSetGitlabSCMTLSCertsConfigMapName,
						},
					},
				},
			})
		}
	}

	podSpec.Containers = []corev1.Container{
		*asr.getContainer(addSCMGitlabVolumeMount),
	}

	req.Spec = appsv1.DeploymentSpec{
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
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

	return req
}

func (asr *ApplicationSetReconciler) getContainer(addSCMGitlabVolumeMount bool) *corev1.Container {
	// Global proxy env vars go first
	appSetEnv := []corev1.EnvVar{{
		Name: "NAMESPACE",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
	}}

	// Merge ApplicationSet env vars provided by the user
	// User should be able to override the default NAMESPACE environmental variable
	appSetEnv = util.EnvMerge(asr.Instance.Spec.ApplicationSet.Env, appSetEnv, true)
	appSetEnv = util.EnvMerge(appSetEnv, util.ProxyEnvVars(), false)
	container := &corev1.Container{
		Command:         asr.getCmd(),
		Image:           argocdcommon.GetArgoContainerImage(asr.Instance),
		ImagePullPolicy: corev1.PullAlways,
		Name:            AppSetController,
		Env:             appSetEnv,
		Resources:       asr.getResources(),
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
				ContainerPort: common.AppSetWebhookPort,
				Name:          common.Webhook,
			},
			{
				ContainerPort: common.AppSetMetricsPort,
				Name:          common.ArgoCDMetrics,
			},
		},
	}

	if addSCMGitlabVolumeMount {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      AppSetGitlabSCMTlsCert,
			MountPath: AppSetGitlabSCMTlsCertPath,
		})
	}

	return container
}

func (asr *ApplicationSetReconciler) getPodVolumes(addSCMGitlabVolumeMount bool) []corev1.Volume {
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
			Name: AppSetGitlabSCMTlsCert,
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
