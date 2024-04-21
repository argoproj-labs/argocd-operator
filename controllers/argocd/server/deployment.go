package server

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

// TriggerDeploymentRollout starts server deployment rollout by updating the given key
func (sr *ServerReconciler) TriggerDeploymentRollout(name, namespace, key string) error {
	return argocdcommon.TriggerDeploymentRollout(name, namespace, key, sr.Client)
}

// reconcileDeployment will ensure all ArgoCD Server deployment is present
func (sr *ServerReconciler) reconcileDeployment() error {

	req := sr.getDeploymentReq()

	ignoreDrift := false
	updateFn := func(existing, desired *appsv1.Deployment, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Spec.Template.Spec.Containers[0].Image, Desired: &desired.Spec.Template.Spec.Containers[0].Image, ExtraAction: func() {
				existing.Spec.Template.ObjectMeta.Labels[common.ImageUpgradedKey] = time.Now().UTC().Format(common.TimeFormatMST)
			},
			},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Command, Desired: &desired.Spec.Template.Spec.Containers[0].Command, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Resources, Desired: &desired.Spec.Template.Spec.Containers[0].Resources, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].VolumeMounts, Desired: &desired.Spec.Template.Spec.Containers[0].VolumeMounts, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.NodeSelector, Desired: &desired.Spec.Template.Spec.NodeSelector, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Tolerations, Desired: &desired.Spec.Template.Spec.Tolerations, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.ServiceAccountName, Desired: &desired.Spec.Template.Spec.ServiceAccountName, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Labels, Desired: &desired.Spec.Template.Labels, ExtraAction: nil},
			{Existing: &existing.Spec.Selector, Desired: &desired.Spec.Selector, ExtraAction: nil},
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
		}
		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}

	return sr.reconDeployment(req, argocdcommon.UpdateFnDep(updateFn), ignoreDrift)
}

func (sr *ServerReconciler) reconDeployment(req workloads.DeploymentRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := workloads.RequestDeployment(req)
	if err != nil {
		sr.Logger.Debug("reconDeployment: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconDeployment: failed to request Deployment %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(sr.Instance, desired, sr.Scheme); err != nil {
		sr.Logger.Error(err, "reconDeployment: failed to set owner reference for Deployment", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := workloads.GetDeployment(desired.Name, desired.Namespace, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconDeployment: failed to retrieve Deployment %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = workloads.CreateDeployment(desired, sr.Client); err != nil {
			return errors.Wrapf(err, "reconDeployment: failed to create Deployment %s in namespace %s", desired.Name, desired.Namespace)
		}
		sr.Logger.Info("Deployment created", "name", desired.Name, "namespace", desired.Namespace)
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

	if err = workloads.UpdateDeployment(existing, sr.Client); err != nil {
		return errors.Wrapf(err, "reconDeployment: failed to update Deployment %s", existing.Name)
	}

	sr.Logger.Info("Deployment updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteDeployment will delete deployment with given name.
func (sr *ServerReconciler) deleteDeployment(name, namespace string) error {
	if err := workloads.DeleteDeployment(name, namespace, sr.Client); err != nil {
		// resource is already deleted, ignore error
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteDeployment: failed to delete deployment %s in namespace %s", name, namespace)
	}
	sr.Logger.Info("deployment deleted", "name", name, "namespace", namespace)
	return nil
}

func (sr *ServerReconciler) getDeploymentReq() workloads.DeploymentRequest {
	req := workloads.DeploymentRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, sr.Instance.Namespace, sr.Instance.Name, sr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Client:     sr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	// set deployment params
	env := sr.Instance.Spec.Server.Env
	env = util.EnvMerge(env, util.ProxyEnvVars(), false)

	resources := corev1.ResourceRequirements{}
	if sr.Instance.Spec.Server.Resources != nil {
		resources = *sr.Instance.Spec.Server.Resources
	}

	// nil if the replicas value is < 0 in argocd CR or autoscale is enabled
	var replicas *int32 = nil
	if !sr.Instance.Spec.Server.Autoscale.Enabled && sr.Instance.Spec.Server.Replicas != nil && *sr.Instance.Spec.Server.Replicas >= 0 {
		replicas = sr.Instance.Spec.Server.Replicas
	}

	podSpec := corev1.PodSpec{
		ServiceAccountName: resourceName,
		Volumes: []corev1.Volume{
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
				Name: common.ArgoCDRepoServerTLS,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: common.ArgoCDRepoServerTLSSecretName,
						Optional:   util.BoolPtr(true),
					},
				},
			},
			{
				Name: common.ArgoCDRedisServerTLS,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: common.ArgoCDRedisServerTLSSecretName,
						Optional:   util.BoolPtr(true),
					},
				},
			},
		},
		Containers: []corev1.Container{{
			Command:         sr.getCmd(),
			Image:           argocdcommon.GetArgoContainerImage(sr.Instance),
			ImagePullPolicy: corev1.PullAlways,
			Name:            common.ServerComponent,
			Env:             env,
			Resources:       resources,
			Ports: []corev1.ContainerPort{
				{
					ContainerPort: 8080,
				}, {
					ContainerPort: 8083,
				},
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/healthz",
						Port: intstr.FromInt(8080),
					},
				},
				InitialDelaySeconds: 3,
				PeriodSeconds:       30,
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/healthz",
						Port: intstr.FromInt(8080),
					},
				},
				InitialDelaySeconds: 3,
				PeriodSeconds:       30,
			},
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: util.BoolPtr(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{
						"ALL",
					},
				},
				RunAsNonRoot: util.BoolPtr(true),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      common.SSHKnownHosts,
					MountPath: common.VolumeMountPathSSH,
				}, {
					Name:      common.TLSCerts,
					MountPath: common.VolumeMountPathTLS,
				},
				{
					Name:      common.ArgoCDRepoServerTLS,
					MountPath: common.VolumeMountPathArgoCDServerTLS,
				},
				{
					Name:      common.ArgoCDRedisServerTLS,
					MountPath: common.VolumeMountPathRedisServerTLS,
				},
			},
		}},
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
		Replicas: replicas,
	}

	return req
}
