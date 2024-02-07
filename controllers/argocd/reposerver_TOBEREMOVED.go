package argocd

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"reflect"
	"time"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileStatusRepo will ensure that the Repo status is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusRepo(cr *argoproj.ArgoCD) error {
	status := "Unknown"

	deploy := newDeploymentWithSuffix("repo-server", "repo-server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		status = "Pending"

		if deploy.Spec.Replicas != nil {
			if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
				status = "Running"
			}
		}
	}

	if cr.Status.Repo != status {
		cr.Status.Repo = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}

// reconcileRepoServerTLSSecret checks whether the argocd-repo-server-tls secret
// has changed since our last reconciliation loop. It does so by comparing the
// checksum of tls.crt and tls.key in the status of the ArgoCD CR against the
// values calculated from the live state in the cluster.
func (r *ReconcileArgoCD) reconcileRepoServerTLSSecret(cr *argoproj.ArgoCD) error {
	var tlsSecretObj corev1.Secret
	var sha256sum string

	log.Info("reconciling repo-server TLS secret")

	tlsSecretName := types.NamespacedName{Namespace: cr.Namespace, Name: common.ArgoCDRepoServerTLSSecretName}
	err := r.Client.Get(context.TODO(), tlsSecretName, &tlsSecretObj)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	} else if tlsSecretObj.Type != corev1.SecretTypeTLS {
		// We only process secrets of type kubernetes.io/tls
		return nil
	} else {
		// We do the checksum over a concatenated byte stream of cert + key
		crt, crtOk := tlsSecretObj.Data[corev1.TLSCertKey]
		key, keyOk := tlsSecretObj.Data[corev1.TLSPrivateKeyKey]
		if crtOk && keyOk {
			var sumBytes []byte
			sumBytes = append(sumBytes, crt...)
			sumBytes = append(sumBytes, key...)
			sha256sum = fmt.Sprintf("%x", sha256.Sum256(sumBytes))
		}
	}

	// The content of the TLS secret has changed since we last looked if the
	// calculated checksum doesn't match the one stored in the status.
	if cr.Status.RepoTLSChecksum != sha256sum {
		// We store the value early to prevent a possible restart loop, for the
		// cost of a possibly missed restart when we cannot update the status
		// field of the resource.
		cr.Status.RepoTLSChecksum = sha256sum
		err = r.Client.Status().Update(context.TODO(), cr)
		if err != nil {
			return err
		}

		// Trigger rollout of API server
		apiDepl := newDeploymentWithSuffix("server", "server", cr)
		err = r.triggerRollout(apiDepl, "repo.tls.cert.changed")
		if err != nil {
			return err
		}

		// Trigger rollout of repository server
		repoDepl := newDeploymentWithSuffix("repo-server", "repo-server", cr)
		err = r.triggerRollout(repoDepl, "repo.tls.cert.changed")
		if err != nil {
			return err
		}

		// Trigger rollout of application controller
		controllerSts := newStatefulSetWithSuffix("application-controller", "application-controller", cr)
		err = r.triggerRollout(controllerSts, "repo.tls.cert.changed")
		if err != nil {
			return err
		}
	}

	return nil
}

// reconcileRepoService will ensure that the Service for the Argo CD repo server is present.
func (r *ReconcileArgoCD) reconcileRepoService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("repo-server", "repo-server", cr)

	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		if !cr.Spec.Repo.IsEnabled() {
			return r.Client.Delete(context.TODO(), svc)
		}
		if ensureAutoTLSAnnotation(svc, common.ArgoCDRepoServerTLSSecretName, cr.Spec.Repo.WantsAutoTLS()) {
			return r.Client.Update(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	if !cr.Spec.Repo.IsEnabled() {
		return nil
	}

	ensureAutoTLSAnnotation(svc, common.ArgoCDRepoServerTLSSecretName, cr.Spec.Repo.WantsAutoTLS())

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("repo-server", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "server",
			Port:       common.ArgoCDDefaultRepoServerPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ArgoCDDefaultRepoServerPort),
		}, {
			Name:       "metrics",
			Port:       common.ArgoCDDefaultRepoMetricsPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ArgoCDDefaultRepoMetricsPort),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// getArgoRepoResources will return the ResourceRequirements for the Argo CD Repo server container.
func getArgoRepoResources(cr *argoproj.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Repo.Resources != nil {
		resources = *cr.Spec.Repo.Resources
	}

	return resources
}

func isRepoServerTLSVerificationRequested(cr *argoproj.ArgoCD) bool {
	return cr.Spec.Repo.VerifyTLS
}

// getRepoServerContainerImage will return the container image for the Repo server.
//
// There are three possible options for configuring the image, and this is the
// order of preference.
//
// 1. from the Spec, the spec.repo field has an image and version to use for
// generating an image reference.
// 2. from the Environment, this looks for the `ARGOCD_REPOSERVER_IMAGE` field and uses
// that if the spec is not configured.
// 3. the default is configured in common.ArgoCDDefaultRepoServerVersion and
// common.ArgoCDDefaultRepoServerImage.
func getRepoServerContainerImage(cr *argoproj.ArgoCD) string {
	defaultImg, defaultTag := false, false
	img := cr.Spec.Repo.Image
	if img == "" {
		img = common.ArgoCDDefaultArgoImage
		defaultImg = true
	}

	tag := cr.Spec.Repo.Version
	if tag == "" {
		tag = common.ArgoCDDefaultArgoVersion
		defaultTag = true
	}
	if e := os.Getenv(common.ArgoCDImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}
	return argoutil.CombineImageTag(img, tag)
}

// getRepoServerAddress will return the Argo CD repo server address.
func getRepoServerAddress(cr *argoproj.ArgoCD) string {
	if cr.Spec.Repo.Remote != nil && *cr.Spec.Repo.Remote != "" {
		return *cr.Spec.Repo.Remote
	}
	return fqdnServiceRef("repo-server", common.ArgoCDDefaultRepoServerPort, cr)
}

// getArgoCDRepoServerReplicas will return the size value for the argocd-repo-server replica count if it
// has been set in argocd CR. Otherwise, nil is returned if the replicas is not set in the argocd CR or
// replicas value is < 0.
func getArgoCDRepoServerReplicas(cr *argoproj.ArgoCD) *int32 {
	if cr.Spec.Repo.Replicas != nil && *cr.Spec.Repo.Replicas >= 0 {
		return cr.Spec.Repo.Replicas
	}

	return nil
}

// getArgoRepoCommand will return the command for the ArgoCD Repo component.
func getArgoRepoCommand(cr *argoproj.ArgoCD, useTLSForRedis bool) []string {
	cmd := make([]string, 0)

	cmd = append(cmd, "uid_entrypoint.sh")
	cmd = append(cmd, "argocd-repo-server")

	if cr.Spec.Redis.IsEnabled() {
		cmd = append(cmd, "--redis", getRedisServerAddress(cr))
	} else {
		log.Info("Redis is Disabled. Skipping adding Redis configuration to Repo Server.")
	}
	if useTLSForRedis {
		cmd = append(cmd, "--redis-use-tls")
		if isRedisTLSVerificationDisabled(cr) {
			cmd = append(cmd, "--redis-insecure-skip-tls-verify")
		} else {
			cmd = append(cmd, "--redis-ca-certificate", "/app/config/reposerver/tls/redis/tls.crt")
		}
	}

	cmd = append(cmd, "--loglevel")
	cmd = append(cmd, getLogLevel(cr.Spec.Repo.LogLevel))

	cmd = append(cmd, "--logformat")
	cmd = append(cmd, getLogFormat(cr.Spec.Repo.LogFormat))

	// *** NOTE ***
	// Do Not add any new default command line arguments below this.
	extraArgs := cr.Spec.Repo.ExtraRepoCommandArgs
	err := isMergable(extraArgs, cmd)
	if err != nil {
		return cmd
	}

	cmd = append(cmd, extraArgs...)
	return cmd
}

// reconcileRepoServerServiceMonitor will ensure that the ServiceMonitor is present for the Repo Server metrics Service.
func (r *ReconcileArgoCD) reconcileRepoServerServiceMonitor(cr *argoproj.ArgoCD) error {
	sm := newServiceMonitorWithSuffix("repo-server-metrics", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, sm.Name, sm) {
		if !cr.Spec.Prometheus.Enabled {
			// ServiceMonitor exists but enabled flag has been set to false, delete the ServiceMonitor
			return r.Client.Delete(context.TODO(), sm)
		}
		return nil // ServiceMonitor found, do nothing
	}

	if !cr.Spec.Prometheus.Enabled {
		return nil // Prometheus not enabled, do nothing.
	}

	sm.Spec.Selector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.ArgoCDKeyName: nameWithSuffix("repo-server", cr),
		},
	}
	sm.Spec.Endpoints = []monitoringv1.Endpoint{
		{
			Port: common.ArgoCDMetrics,
		},
	}

	if err := controllerutil.SetControllerReference(cr, sm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), sm)
}

// getArgoCmpServerInitCommand will return the command for the ArgoCD CMP Server init container
func getArgoCmpServerInitCommand() []string {
	cmd := make([]string, 0)
	cmd = append(cmd, "cp")
	cmd = append(cmd, "-n")
	cmd = append(cmd, "/usr/local/bin/argocd")
	cmd = append(cmd, "/var/run/argocd/argocd-cmp-server")
	return cmd
}

// reconcileRepoDeployment will ensure the Deployment resource is present for the ArgoCD Repo component.
func (r *ReconcileArgoCD) reconcileRepoDeployment(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	deploy := newDeploymentWithSuffix("repo-server", "repo-server", cr)
	automountToken := false
	if cr.Spec.Repo.MountSAToken {
		automountToken = cr.Spec.Repo.MountSAToken
	}

	deploy.Spec.Template.Spec.AutomountServiceAccountToken = &automountToken

	if cr.Spec.Repo.ServiceAccount != "" {
		deploy.Spec.Template.Spec.ServiceAccountName = cr.Spec.Repo.ServiceAccount
	}

	// Global proxy env vars go first
	repoEnv := cr.Spec.Repo.Env
	// Environment specified in the CR take precedence over everything else
	repoEnv = argoutil.EnvMerge(repoEnv, proxyEnvVars(), false)
	if cr.Spec.Repo.ExecTimeout != nil {
		repoEnv = argoutil.EnvMerge(repoEnv, []corev1.EnvVar{{Name: "ARGOCD_EXEC_TIMEOUT", Value: fmt.Sprintf("%ds", *cr.Spec.Repo.ExecTimeout)}}, true)
	}

	AddSeccompProfileForOpenShift(r.Client, &deploy.Spec.Template.Spec)

	deploy.Spec.Template.Spec.InitContainers = []corev1.Container{{
		Name:            "copyutil",
		Image:           getArgoContainerImage(cr),
		Command:         getArgoCmpServerInitCommand(),
		ImagePullPolicy: corev1.PullAlways,
		Resources:       getArgoRepoResources(cr),
		Env:             proxyEnvVars(),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			RunAsNonRoot: boolPtr(true),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "var-files",
				MountPath: "/var/run/argocd",
			},
		},
	}}

	if cr.Spec.Repo.InitContainers != nil {
		deploy.Spec.Template.Spec.InitContainers = append(deploy.Spec.Template.Spec.InitContainers, cr.Spec.Repo.InitContainers...)
	}

	repoServerVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "ssh-known-hosts",
			MountPath: "/app/config/ssh",
		},
		{
			Name:      "tls-certs",
			MountPath: "/app/config/tls",
		},
		{
			Name:      "gpg-keys",
			MountPath: "/app/config/gpg/source",
		},
		{
			Name:      "gpg-keyring",
			MountPath: "/app/config/gpg/keys",
		},
		{
			Name:      "tmp",
			MountPath: "/tmp",
		},
		{
			Name:      "argocd-repo-server-tls",
			MountPath: "/app/config/reposerver/tls",
		},
		{
			Name:      common.ArgoCDRedisServerTLSSecretName,
			MountPath: "/app/config/reposerver/tls/redis",
		},
		{
			Name:      "plugins",
			MountPath: "/home/argocd/cmp-server/plugins",
		},
	}

	if cr.Spec.Repo.VolumeMounts != nil {
		repoServerVolumeMounts = append(repoServerVolumeMounts, cr.Spec.Repo.VolumeMounts...)
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command:         getArgoRepoCommand(cr, useTLSForRedis),
		Image:           getRepoServerContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(common.ArgoCDDefaultRepoServerPort),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		Env:  repoEnv,
		Name: "argocd-repo-server",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultRepoServerPort,
				Name:          "server",
			}, {
				ContainerPort: common.ArgoCDDefaultRepoMetricsPort,
				Name:          "metrics",
			},
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
		Resources: getArgoRepoResources(cr),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			RunAsNonRoot: boolPtr(true),
		},
		VolumeMounts: repoServerVolumeMounts,
	}}

	if cr.Spec.Repo.SidecarContainers != nil {
		deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers, cr.Spec.Repo.SidecarContainers...)
	}

	repoServerVolumes := []corev1.Volume{
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
		{
			Name: "argocd-repo-server-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRepoServerTLSSecretName,
					Optional:   boolPtr(true),
				},
			},
		},
		{
			Name: common.ArgoCDRedisServerTLSSecretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRedisServerTLSSecretName,
					Optional:   boolPtr(true),
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

	if cr.Spec.Repo.Volumes != nil {
		repoServerVolumes = append(repoServerVolumes, cr.Spec.Repo.Volumes...)
	}

	deploy.Spec.Template.Spec.Volumes = repoServerVolumes

	if replicas := getArgoCDRepoServerReplicas(cr); replicas != nil {
		deploy.Spec.Replicas = replicas
	}

	existing := newDeploymentWithSuffix("repo-server", "repo-server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {

		if !cr.Spec.Repo.IsEnabled() {
			log.Info("Existing ArgoCD Repo Server found but should be disabled. Deleting Repo Server")
			// Delete existing deployment for ArgoCD Repo Server, if any ..
			return r.Client.Delete(context.TODO(), existing)
		}

		changed := false
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getRepoServerContainerImage(cr)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			if existing.Spec.Template.ObjectMeta.Labels == nil {
				existing.Spec.Template.ObjectMeta.Labels = map[string]string{
					"image.upgraded": time.Now().UTC().Format("01022006-150406-MST"),
				}
			}
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}
		updateNodePlacement(existing, deploy, &changed)
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Volumes, existing.Spec.Template.Spec.Volumes) {
			existing.Spec.Template.Spec.Volumes = deploy.Spec.Template.Spec.Volumes
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].VolumeMounts,
			existing.Spec.Template.Spec.Containers[0].VolumeMounts) {
			existing.Spec.Template.Spec.Containers[0].VolumeMounts = deploy.Spec.Template.Spec.Containers[0].VolumeMounts
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Env,
			existing.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Resources, existing.Spec.Template.Spec.Containers[0].Resources) {
			existing.Spec.Template.Spec.Containers[0].Resources = deploy.Spec.Template.Spec.Containers[0].Resources
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Command, existing.Spec.Template.Spec.Containers[0].Command) {
			existing.Spec.Template.Spec.Containers[0].Command = deploy.Spec.Template.Spec.Containers[0].Command
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[1:],
			existing.Spec.Template.Spec.Containers[1:]) {
			existing.Spec.Template.Spec.Containers = append(existing.Spec.Template.Spec.Containers[0:1],
				deploy.Spec.Template.Spec.Containers[1:]...)
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.InitContainers, existing.Spec.Template.Spec.InitContainers) {
			existing.Spec.Template.Spec.InitContainers = deploy.Spec.Template.Spec.InitContainers
			changed = true
		}

		if !reflect.DeepEqual(deploy.Spec.Replicas, existing.Spec.Replicas) {
			existing.Spec.Replicas = deploy.Spec.Replicas
			changed = true
		}

		if deploy.Spec.Template.Spec.AutomountServiceAccountToken != existing.Spec.Template.Spec.AutomountServiceAccountToken {
			existing.Spec.Template.Spec.AutomountServiceAccountToken = deploy.Spec.Template.Spec.AutomountServiceAccountToken
			changed = true
		}

		if deploy.Spec.Template.Spec.ServiceAccountName != existing.Spec.Template.Spec.ServiceAccountName {
			existing.Spec.Template.Spec.ServiceAccountName = deploy.Spec.Template.Spec.ServiceAccountName
			changed = true
		}

		if changed {
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if !cr.Spec.Repo.IsEnabled() {
		log.Info("ArgoCD Repo Server disabled. Skipping starting ArgoCD Repo Server.")
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
}
