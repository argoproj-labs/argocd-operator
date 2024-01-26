package server

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"

	"fmt"
	"strings"
	"time"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/redis"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/reposerver"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/sso/dex"
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

	deployTpl := sr.getServerDeploymentTpl()

	deployReq := workloads.DeploymentRequest{
		ObjectMeta: deployTpl.ObjectMeta,
		Spec:       deployTpl.Spec,
		Client:     sr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	desiredDeploy, err := workloads.RequestDeployment(deployReq)
	if err != nil {
		return errors.Wrapf(err, "reconcileDeployment: failed to request deployment %s in namespace %s", desiredDeploy.Name, desiredDeploy.Namespace)
	}

	if err := controllerutil.SetControllerReference(sr.Instance, desiredDeploy, sr.Scheme); err != nil {
		sr.Logger.Error(err, "reconcileDeployment: failed to set owner reference for deployment", "name", desiredDeploy.Name, "namespace", desiredDeploy.Namespace)
	}

	// deployment doesn't exist in the namespace, create it
	existingDeploy, err := workloads.GetDeployment(desiredDeploy.Name, desiredDeploy.Namespace, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileDeployment: failed to retrieve deployment %s in namespace %s", desiredDeploy.Name, desiredDeploy.Namespace)
		}

		if err = workloads.CreateDeployment(desiredDeploy, sr.Client); err != nil {
			return errors.Wrapf(err, "reconcileDeployment: failed to create deployment %s in namespace %s", desiredDeploy.Name, desiredDeploy.Namespace)
		}

		sr.Logger.V(0).Info("deployment created", "name", desiredDeploy.Name, "namespace", desiredDeploy.Namespace)
		return nil
	}

	// difference in existing & desired deployment, update it
	changed := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{&existingDeploy.Spec.Template.Spec.Containers[0].Image, &desiredDeploy.Spec.Template.Spec.Containers[0].Image,
			func() {
				existingDeploy.Spec.Template.ObjectMeta.Labels[common.ImageUpgradedKey] = time.Now().UTC().Format(common.TimeFormatMST)
			},
		},
		{&existingDeploy.Spec.Template.Spec.Containers[0].Command, &desiredDeploy.Spec.Template.Spec.Containers[0].Command, nil},
		{&existingDeploy.Spec.Template.Spec.Containers[0].Env, &desiredDeploy.Spec.Template.Spec.Containers[0].Env, nil},
		{&existingDeploy.Spec.Template.Spec.Containers[0].Resources, &desiredDeploy.Spec.Template.Spec.Containers[0].Resources, nil},
		{&existingDeploy.Spec.Template.Spec.Containers[0].VolumeMounts, &desiredDeploy.Spec.Template.Spec.Containers[0].VolumeMounts, nil},
		{&existingDeploy.Spec.Template.Spec.Volumes, &desiredDeploy.Spec.Template.Spec.Volumes, nil},
		{&existingDeploy.Spec.Template.Spec.NodeSelector, &desiredDeploy.Spec.Template.Spec.NodeSelector, nil},
		{&existingDeploy.Spec.Template.Spec.Tolerations, &desiredDeploy.Spec.Template.Spec.Tolerations, nil},
		{&existingDeploy.Spec.Template.Spec.ServiceAccountName, &desiredDeploy.Spec.Template.Spec.ServiceAccountName, nil},
		{&existingDeploy.Spec.Template.Labels, &desiredDeploy.Spec.Template.Labels, nil},
		{&existingDeploy.Spec.Replicas, &desiredDeploy.Spec.Replicas, nil},
		{&existingDeploy.Spec.Selector, &desiredDeploy.Spec.Selector, nil},
		{&existingDeploy.Labels, &desiredDeploy.Labels, nil},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &changed)
	}

	// nothing changed, exit reconciliation
	if !changed {
		return nil
	}

	if err = workloads.UpdateDeployment(existingDeploy, sr.Client); err != nil {
		return errors.Wrapf(err, "reconcileDeployment: failed to update deployment %s in namespace %s", existingDeploy.Name, existingDeploy.Namespace)
	}
	
	sr.Logger.V(0).Info("deployment updated", "name", existingDeploy.Name, "namespace", existingDeploy.Namespace)
	return nil
}

// deleteDeployment will delete deployment with given name.
func (sr *ServerReconciler) deleteDeployment(name, namespace string) error {
	if err := workloads.DeleteDeployment(name, namespace, sr.Client); err != nil {
		return errors.Wrapf(err, "deleteDeployment: failed to delete deployment %s in namespace %s", name, namespace)
	}
	sr.Logger.V(0).Info("deployment deleted", "name", name, "namespace", namespace)
	return nil
}

// getServerDeploymentTmpl returns server deployment object
func (sr *ServerReconciler) getServerDeploymentTpl() *appsv1.Deployment {

	// deployment metadata
	objMeta := argoutil.GetObjMeta(resourceName, sr.Instance.Namespace, sr.Instance.Name, sr.Instance.Namespace, component)

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
			Command:         sr.getArgoServerCommand(),
			Image:           argocdcommon.GetArgoContainerImage(sr.Instance),
			ImagePullPolicy: corev1.PullAlways,
			Name:            ServerControllerComponent,
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

	deploymentSpec := appsv1.DeploymentSpec{
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

	deployment := &appsv1.Deployment{}
	deployment.ObjectMeta = objMeta
	deployment.Spec = deploymentSpec
	return deployment
}

// getArgoServerCommand will return the command for the ArgoCD server component.
func (sr *ServerReconciler) getArgoServerCommand() []string {
	cmd := make([]string, 0)
	cmd = append(cmd, "argocd-server")

	if sr.Instance.Spec.Server.Insecure {
		cmd = append(cmd, "--insecure")
	}

	cmd = append(cmd, "--staticassets")
	cmd = append(cmd, "/shared/app")

	cmd = append(cmd, "--dex-server")
	cmd = append(cmd, dex.GetDexServerAddress(sr.Instance.Name, sr.Instance.Namespace))

	// reposerver flags
	if reposerver.UseTLSForRepoServer(sr.Instance) {
		cmd = append(cmd, "--repo-server-strict-tls")
	}

	cmd = append(cmd, "--repo-server")
	cmd = append(cmd, reposerver.GetRepoServerAddress(sr.Instance.Name, sr.Instance.Namespace))

	// redis flags
	cmd = append(cmd, "--redis")
	cmd = append(cmd, redis.GetRedisServerAddress(sr.Instance))

	// TODO: add tls check for redis
	//if useTLSForRedis {
	if true {
		cmd = append(cmd, "--redis-use-tls")
		if redis.IsRedisTLSVerificationDisabled(sr.Instance) {
			cmd = append(cmd, "--redis-insecure-skip-tls-verify")
		} else {
			cmd = append(cmd, "--redis-ca-certificate", "/app/config/server/tls/redis/tls.crt")
		}
	}

	// set log level & format
	cmd = append(cmd, "--loglevel")
	cmd = append(cmd, argoutil.GetLogLevel(sr.Instance.Spec.Server.LogLevel))

	cmd = append(cmd, "--logformat")
	cmd = append(cmd, argoutil.GetLogLevel(sr.Instance.Spec.Server.LogFormat))

	// set source namespaces
	if sr.Instance.Spec.SourceNamespaces != nil && len(sr.Instance.Spec.SourceNamespaces) > 0 {
		cmd = append(cmd, "--application-namespaces", fmt.Sprint(strings.Join(sr.Instance.Spec.SourceNamespaces, ",")))
	}

	// extra args should always be added at the end
	extraArgs := sr.Instance.Spec.Server.ExtraCommandArgs
	err := util.IsMergable(extraArgs, cmd)
	if err != nil {
		return cmd
	}
	cmd = append(cmd, extraArgs...)

	return cmd
}
