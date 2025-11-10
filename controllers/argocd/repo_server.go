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
	"crypto/sha256"
	"fmt"
	"reflect"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	argocdoperatorv1beta1 "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// getArgoCDRepoServerReplicas will return the size value for the argocd-repo-server replica count if it
// has been set in argocd CR. Otherwise, nil is returned if the replicas is not set in the argocd CR or
// replicas value is < 0.
func getArgoCDRepoServerReplicas(cr *argocdoperatorv1beta1.ArgoCD) *int32 {
	if cr.Spec.Repo.Replicas != nil && *cr.Spec.Repo.Replicas >= 0 {
		return cr.Spec.Repo.Replicas
	}

	return nil
}

// getArgoRepoCommand will return the command for the ArgoCD Repo component.
func getArgoRepoCommand(cr *argocdoperatorv1beta1.ArgoCD, useTLSForRedis bool) []string {
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
	cmd = appendUniqueArgs(cmd, extraArgs)

	return cmd
}

// getRepoServerAddress will return the Argo CD repo server address.
func getRepoServerAddress(cr *argocdoperatorv1beta1.ArgoCD) string {
	if cr.Spec.Repo.IsRemote() {
		return *cr.Spec.Repo.Remote
	}
	return fqdnServiceRef("repo-server", common.ArgoCDDefaultRepoServerPort, cr)
}

// reconcileRepoDeployment will ensure the Deployment resource is present for the ArgoCD Repo component.
func (r *ReconcileArgoCD) reconcileRepoDeployment(cr *argocdoperatorv1beta1.ArgoCD, useTLSForRedis bool) error {
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
	repoEnv = append(repoEnv, corev1.EnvVar{
		Name: "REDIS_PASSWORD",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: fmt.Sprintf("%s-%s", cr.Name, "redis-initial-password"),
				},
				Key: "admin.password",
			},
		},
	})
	// Environment specified in the CR take precedence over everything else
	repoEnv = argoutil.EnvMerge(repoEnv, proxyEnvVars(), false)
	if cr.Spec.Repo.ExecTimeout != nil {
		repoEnv = argoutil.EnvMerge(repoEnv, []corev1.EnvVar{{Name: "ARGOCD_EXEC_TIMEOUT", Value: fmt.Sprintf("%ds", *cr.Spec.Repo.ExecTimeout)}}, true)
	}

	// if running in a FIPS enabled host, set the GODEBUG env wit appropriate value
	fipsConfigChecker := r.FipsConfigChecker
	if fipsConfigChecker != nil {
		fipsEnabled, err := fipsConfigChecker.IsFipsEnabled()
		if err != nil {
			return err
		}
		if fipsEnabled {
			repoEnv = append(repoEnv, corev1.EnvVar{
				Name:  "GODEBUG",
				Value: "fips140=on",
			})
		}
	}

	AddSeccompProfileForOpenShift(r.Client, &deploy.Spec.Template.Spec)

	deploy.Spec.Template.Spec.InitContainers = []corev1.Container{{
		Name:            "copyutil",
		Image:           getArgoContainerImage(cr),
		Command:         getArgoCmpServerInitCommand(),
		ImagePullPolicy: argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy),
		Resources:       getArgoRepoResources(cr),
		Env:             proxyEnvVars(),
		SecurityContext: argoutil.DefaultSecurityContext(),
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

	// If the user has specified a custom volume mount that overrides the existing /tmp mount, then we should use the user's custom mount, rather than the default.
	volumeMountOverridesTmpVolume := false
	for _, volumeMount := range cr.Spec.Repo.VolumeMounts {
		if volumeMount.MountPath == "/tmp" {
			volumeMountOverridesTmpVolume = true
			break
		}
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

	if !volumeMountOverridesTmpVolume {

		repoServerVolumeMounts = append(repoServerVolumeMounts, corev1.VolumeMount{
			Name:      "tmp",
			MountPath: "/tmp",
		})

	}

	if cr.Spec.Repo.VolumeMounts != nil {
		repoServerVolumeMounts = append(repoServerVolumeMounts, cr.Spec.Repo.VolumeMounts...)
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command:         getArgoRepoCommand(cr, useTLSForRedis),
		Image:           getRepoServerContainerImage(cr),
		ImagePullPolicy: argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy),
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
		Resources:       getArgoRepoResources(cr),
		SecurityContext: argoutil.DefaultSecurityContext(),
		VolumeMounts:    repoServerVolumeMounts,
	}}

	if cr.Spec.Repo.SidecarContainers != nil {
		// If no image is specified for a sidecar container, use the default
		// argo cd repo server image. Copy the containers to avoid changing the
		// original CR.
		containers := []corev1.Container{}
		containers = append(containers, cr.Spec.Repo.SidecarContainers...)
		image := getRepoServerContainerImage(cr)
		for i := range containers {
			if len(containers[i].Image) == 0 {
				containers[i].Image = image
				msg := fmt.Sprintf("no image specified for sidecar container \"%s\" in ArgoCD custom resource \"%s/%s\", using default image",
					containers[i].Name, cr.Namespace, cr.Name)
				log.Info(msg)
			}
		}
		deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers, containers...)
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

	// If the user is not used a custom /tmp mount, then just use the default
	if !volumeMountOverridesTmpVolume {
		repoServerVolumes = append(repoServerVolumes, corev1.Volume{
			Name: "tmp",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	if cr.Spec.Repo.Volumes != nil {
		repoServerVolumes = append(repoServerVolumes, cr.Spec.Repo.Volumes...)
	}
	deploy.Spec.Template.Spec.Volumes = repoServerVolumes

	r.injectCATrustToContainers(cr, deploy)

	if replicas := getArgoCDRepoServerReplicas(cr); replicas != nil {
		deploy.Spec.Replicas = replicas
	}

	if cr.Spec.Repo.Annotations != nil {
		for key, value := range cr.Spec.Repo.Annotations {
			deploy.Spec.Template.Annotations[key] = value
		}
	}

	if cr.Spec.Repo.Labels != nil {
		for key, value := range cr.Spec.Repo.Labels {
			deploy.Spec.Template.Labels[key] = value
		}
	}

	log.Info("Applying ArgoCD Repo Server reconciler hook")
	if err := applyReconcilerHook(cr, deploy, ""); err != nil {
		log.Error(err, "ArgoCD Repo Server reconciler hook failed")
		return err
	}

	existing := newDeploymentWithSuffix("repo-server", "repo-server", cr)
	deplExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing)
	if err != nil {
		return err
	}
	if deplExists {

		if !cr.Spec.Repo.IsEnabled() {
			// Delete existing deployment for ArgoCD Repo Server, if any ..
			argoutil.LogResourceDeletion(log, existing, "repo server is disabled")
			return r.Delete(context.TODO(), existing)
		} else if cr.Spec.Repo.IsRemote() {
			argoutil.LogResourceDeletion(log, deploy, "remote repo server is configured")
			return r.Delete(context.TODO(), deploy)
		}

		changed := false
		explanation := ""
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getRepoServerContainerImage(cr)
		actualImagePullPolicy := existing.Spec.Template.Spec.Containers[0].ImagePullPolicy
		desiredImagePullPolicy := argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			if existing.Spec.Template.Labels == nil {
				existing.Spec.Template.Labels = map[string]string{
					"image.upgraded": time.Now().UTC().Format("01022006-150406-MST"),
				}
			}
			existing.Spec.Template.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			explanation = "container image"
			changed = true
		}
		if actualImagePullPolicy != desiredImagePullPolicy {
			existing.Spec.Template.Spec.Containers[0].ImagePullPolicy = desiredImagePullPolicy
			if changed {
				explanation += ", "
			}
			explanation += "image pull policy"
			changed = true
		}
		updateNodePlacement(existing, deploy, &changed, &explanation)
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Volumes, existing.Spec.Template.Spec.Volumes) {
			existing.Spec.Template.Spec.Volumes = deploy.Spec.Template.Spec.Volumes
			if changed {
				explanation += ", "
			}
			explanation += "volumes"
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].VolumeMounts,
			existing.Spec.Template.Spec.Containers[0].VolumeMounts) {
			existing.Spec.Template.Spec.Containers[0].VolumeMounts = deploy.Spec.Template.Spec.Containers[0].VolumeMounts
			if changed {
				explanation += ", "
			}
			explanation += "container volume mounts"
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Env,
			existing.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			if changed {
				explanation += ", "
			}
			explanation += "container env"
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Resources, existing.Spec.Template.Spec.Containers[0].Resources) {
			existing.Spec.Template.Spec.Containers[0].Resources = deploy.Spec.Template.Spec.Containers[0].Resources
			if changed {
				explanation += ", "
			}
			explanation += "container resources"
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Command, existing.Spec.Template.Spec.Containers[0].Command) {
			existing.Spec.Template.Spec.Containers[0].Command = deploy.Spec.Template.Spec.Containers[0].Command
			if changed {
				explanation += ", "
			}
			explanation += "container command"
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].SecurityContext, existing.Spec.Template.Spec.Containers[0].SecurityContext) {
			existing.Spec.Template.Spec.Containers[0].SecurityContext = deploy.Spec.Template.Spec.Containers[0].SecurityContext
			if changed {
				explanation += ", "
			}
			explanation += "container security context"
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.SecurityContext, existing.Spec.Template.Spec.SecurityContext) {
			existing.Spec.Template.Spec.SecurityContext = deploy.Spec.Template.Spec.SecurityContext
			if changed {
				explanation += ", "
			}
			explanation += "pod security context"
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[1:],
			existing.Spec.Template.Spec.Containers[1:]) {
			existing.Spec.Template.Spec.Containers = append(existing.Spec.Template.Spec.Containers[0:1],
				deploy.Spec.Template.Spec.Containers[1:]...)
			if changed {
				explanation += ", "
			}
			explanation += "additional containers"
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.InitContainers, existing.Spec.Template.Spec.InitContainers) {
			existing.Spec.Template.Spec.InitContainers = deploy.Spec.Template.Spec.InitContainers
			if changed {
				explanation += ", "
			}
			explanation += "init containers"
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Replicas, existing.Spec.Replicas) {
			existing.Spec.Replicas = deploy.Spec.Replicas
			if changed {
				explanation += ", "
			}
			explanation += "replicas"
			changed = true
		}

		if deploy.Spec.Template.Spec.AutomountServiceAccountToken != existing.Spec.Template.Spec.AutomountServiceAccountToken {
			existing.Spec.Template.Spec.AutomountServiceAccountToken = deploy.Spec.Template.Spec.AutomountServiceAccountToken
			if changed {
				explanation += ", "
			}
			explanation += "auto-mount service account token"
			changed = true
		}

		if deploy.Spec.Template.Spec.ServiceAccountName != existing.Spec.Template.Spec.ServiceAccountName {
			existing.Spec.Template.Spec.ServiceAccountName = deploy.Spec.Template.Spec.ServiceAccountName
			existing.Spec.Template.Spec.DeprecatedServiceAccount = deploy.Spec.Template.Spec.ServiceAccountName
			if changed {
				explanation += ", "
			}
			explanation += "service account name"
			changed = true
		}

		// Add Kubernetes-specific labels/annotations from the live object in the source to preserve metadata.
		addKubernetesData(deploy.Spec.Template.Labels, existing.Spec.Template.Labels)
		addKubernetesData(deploy.Spec.Template.Annotations, existing.Spec.Template.Annotations)

		if !reflect.DeepEqual(deploy.Spec.Template.Annotations, existing.Spec.Template.Annotations) {
			existing.Spec.Template.Annotations = deploy.Spec.Template.Annotations
			if changed {
				explanation += ", "
			}
			explanation += "annotations"
			changed = true
		}

		if !reflect.DeepEqual(deploy.Spec.Template.Labels, existing.Spec.Template.Labels) {
			existing.Spec.Template.Labels = deploy.Spec.Template.Labels
			if changed {
				explanation += ", "
			}
			explanation += "labels"
			changed = true
		}

		if changed {
			argoutil.LogResourceUpdate(log, existing, "updating", explanation)
			return r.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if cr.Spec.Redis.IsEnabled() && cr.Spec.Repo.IsRemote() {
		log.Info("Custom Repo Endpoint. Skipping starting Repo Server.")
		return nil
	}

	if !cr.Spec.Repo.IsEnabled() {
		log.Info("ArgoCD Repo Server disabled. Skipping starting ArgoCD Repo Server.")
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, deploy)
	return r.Create(context.TODO(), deploy)
}

// injectCATrustToContainers Creates the init container and volumes to trust CAs specified by `spec.repo.systemCATrust`.
//
// Take CAs from the `argocd-ca-trust-source` volume and mix it with the distro CAs into `argocd-ca-trust-target` volumes.
// Several ubuntu-specific problems exist:
// 1. /etc/ssl/certs/ cannot be updated by `update-ca-certificates` without root - desirable in the production container.
// 2. /etc/ssl/certs/ symlinkes to /usr/local/share/ca-certificates/, so mounting one without the other is futile.
//
// All source certs are projected into the `argocd-ca-trust-source` volume that is ultimately mounted in the prod container (addresses #2).
//
// To amend content of /etc/ssl/certs/ (ca-trust-target), an init container is used:
//   - it mounts `argocd-ca-trust-target` over `/etc/ssl/certs/` (addressing #1 by making it writable volume),
//     and `ca-trust-source` over `/usr/local/share/ca-certificates/`,
//     and amends it with user-added certs using `update-ca-certificates`.
//
// The production container is then mounted with `/etc/ssl/certs/` (`argocd-ca-trust-target`) and
// `/usr/local/share/ca-certificates/` (`argocd-ca-trust-source`) providing read-only CAs needed.
func (r *ReconcileArgoCD) injectCATrustToContainers(cr *argocdoperatorv1beta1.ArgoCD, deploy *appsv1.Deployment) {
	if cr.Spec.Repo.SystemCATrust == nil {
		return
	}

	sources, sourceNames := r.caTrustVolumes(cr)

	volumeSource := "argocd-ca-trust-source"
	volumeTarget := "argocd-ca-trust-target"

	repoServerVolumes := []corev1.Volume{
		{
			Name: volumeSource,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources:     sources,
					DefaultMode: ptr.To(int32(0o444)),
				},
			},
		}, {
			Name: volumeTarget,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	// The `/tmp` volume can be user-provided, so the actual mount name must be looked up
	volumeTmp := ""
	for _, vm := range deploy.Spec.Template.Spec.Containers[0].VolumeMounts {
		if vm.MountPath == "/tmp" {
			volumeTmp = vm.Name
			break
		}
	}

	argoImage := getArgoContainerImage(cr)
	deploy.Spec.Template.Spec.InitContainers = append(
		deploy.Spec.Template.Spec.InitContainers,
		caTrustInitContainer(cr, argoImage, volumeSource, volumeTarget, volumeTmp),
	)

	prodVolumeMounts := func() []corev1.VolumeMount {
		return []corev1.VolumeMount{
			{Name: volumeSource, ReadOnly: true, MountPath: "/usr/local/share/ca-certificates/"},
			{Name: volumeTarget, ReadOnly: true, MountPath: "/etc/ssl/certs/"},
		}
	}

	// Inject to prod container and sidecars (plugins)
	var containerNames []string
	for i, container := range deploy.Spec.Template.Spec.Containers {
		// This can only work with ubuntu or compatible, so do not inject to potentially incompatible containers
		if container.Image == argoImage {
			// Accessing by index because the container is a copy of the original struct
			deploy.Spec.Template.Spec.Containers[i].VolumeMounts = append(deploy.Spec.Template.Spec.Containers[i].VolumeMounts, prodVolumeMounts()...)
			containerNames = append(containerNames, container.Name)
		}
	}

	log.Info(fmt.Sprintf(
		"injecting system CA trust from %s to containers %s",
		strings.Join(sourceNames, ", "),
		strings.Join(containerNames, ", "),
	))

	deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, repoServerVolumes...)
}

func (r *ReconcileArgoCD) caTrustVolumes(cr *argocdoperatorv1beta1.ArgoCD) (sources []corev1.VolumeProjection, sourceNames []string) {
	// The projected file needs to have the `.crt` suffix for the update-ca-certificates to work correctly. Add it if not present.
	ensureValidPath := func(path string) string {
		if strings.HasSuffix(path, ".crt") {
			return path
		}
		return path + ".crt"
	}

	trackSource := func(kind string, name string, optional *bool) {
		path := kind + ":" + name
		if optional != nil && *optional {
			path += "(optional)"
		}
		sourceNames = append(sourceNames, path)
	}

	for _, bundle := range cr.Spec.Repo.SystemCATrust.ClusterTrustBundles {
		bundle = *bundle.DeepCopy()
		// Using .Path, because .Name might not be specified
		trackSource("ClusterTrustBundle", bundle.Path, bundle.Optional)

		bundle.Path = ensureValidPath(bundle.Path)
		sources = append(sources, corev1.VolumeProjection{ClusterTrustBundle: &bundle})
	}
	for _, secret := range cr.Spec.Repo.SystemCATrust.Secrets {
		secret = *secret.DeepCopy()
		trackSource("Secret", secret.Name, secret.Optional)

		for i, item := range secret.Items {
			secret.Items[i].Path = ensureValidPath(item.Path)
		}
		sources = append(sources, corev1.VolumeProjection{Secret: &secret})
	}
	for _, cm := range cr.Spec.Repo.SystemCATrust.ConfigMaps {
		cm = *cm.DeepCopy()
		trackSource("ConfigMap", cm.Name, cm.Optional)

		for i, cmi := range cm.Items {
			cm.Items[i].Path = ensureValidPath(cmi.Path)
		}
		sources = append(sources, corev1.VolumeProjection{ConfigMap: &cm})
	}
	return sources, sourceNames
}

func caTrustInitContainer(cr *argocdoperatorv1beta1.ArgoCD, argoImage string, volumeSource string, volumeTarget string, tmpVolume string) corev1.Container {
	// This is where the image keeps its vendored CAs, look elsewhere if DropImageCertificates
	imageCertPath := "/usr/share/ca-certificates"
	if cr.Spec.Repo.SystemCATrust.DropImageCertificates {
		imageCertPath = "/systemCATrust.dropImageCertificates"
	}

	return corev1.Container{
		Name:            "update-ca-certificates",
		Image:           argoImage,
		ImagePullPolicy: argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy),
		Env: proxyEnvVars(corev1.EnvVar{
			Name:  "IMAGE_CERT_PATH",
			Value: imageCertPath,
		}),
		Command: []string{"/bin/bash", "-c"},
		Args: []string{`
                set -eEuo pipefail
                trap 's=$?; echo >&2 "$0: Error on line "$LINENO": $BASH_COMMAND"; exit $s' ERR

                echo "User defined CA files:"
                ls -l /usr/local/share/ca-certificates/

                # Make sure the file exist even when the update-ca-certificates produces no pem blocks
                echo "" > /etc/ssl/certs/ca-certificates.crt

                update-ca-certificates --verbose --certsdir "$IMAGE_CERT_PATH"
                echo "Resulting /etc/ssl/certs/"
                ls -l /etc/ssl/certs/
                echo "Done!"
        `},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name: volumeSource,
				// Source path for user additional certificates - empty in the image, so not shadowing anything.
				MountPath: "/usr/local/share/ca-certificates/",
				ReadOnly:  true,
			}, {
				Name:      volumeTarget,
				MountPath: "/etc/ssl/certs/",
			}, {
				Name:      tmpVolume,
				MountPath: "/tmp",
			},
		},
		Resources:       getArgoRepoResources(cr),
		SecurityContext: argoutil.DefaultSecurityContext(),
	}
}

// getArgoRepoResources will return the ResourceRequirements for the Argo CD Repo server container.
func getArgoRepoResources(cr *argocdoperatorv1beta1.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Repo.Resources != nil {
		resources = *cr.Spec.Repo.Resources
	}

	return resources
}

// getRepoServerContainerImage will return the container image for the Repo server.
//
// There are four possible options for configuring the image, and this is the
// order of preference.
//
// 1. from the Spec, the spec.repo field has an image and version to use for
// generating an image reference.
// 2. from the Spec, the spec.version field has an image and version to use for
// generating an image reference
// 3. from the Environment, this looks for the `ARGOCD_IMAGE` field and uses
// that if the spec is not configured.
// 4. the default is configured in common.ArgoCDDefaultArgoVersion and
// common.ArgoCDDefaultArgoImage.
func getRepoServerContainerImage(cr *argocdoperatorv1beta1.ArgoCD) string {
	img, tag := GetImageAndTag(common.ArgoCDImageEnvName, cr.Spec.Repo.Image, cr.Spec.Repo.Version, cr.Spec.Image, cr.Spec.Version)
	return argoutil.CombineImageTag(img, tag)
}

func isRepoServerTLSVerificationRequested(cr *argocdoperatorv1beta1.ArgoCD) bool {
	return cr.Spec.Repo.VerifyTLS
}

// reconcileRepoService will ensure that the Service for the Argo CD repo server is present.
func (r *ReconcileArgoCD) reconcileRepoService(cr *argocdoperatorv1beta1.ArgoCD) error {
	svc := newServiceWithSuffix("repo-server", "repo-server", cr)

	svcFound, err := argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc)
	if err != nil {
		return err
	}
	if svcFound {
		if !cr.Spec.Repo.IsEnabled() {
			argoutil.LogResourceDeletion(log, svc, "repo server is disabled")
			return r.Delete(context.TODO(), svc)
		}
		update, err := ensureAutoTLSAnnotation(r.Client, svc, common.ArgoCDRepoServerTLSSecretName, cr.Spec.Repo.WantsAutoTLS())
		if err != nil {
			return err
		}
		if update {
			argoutil.LogResourceUpdate(log, svc, "updating auto tls annotation")
			return r.Update(context.TODO(), svc)
		}
		if cr.Spec.Repo.IsRemote() {
			argoutil.LogResourceDeletion(log, svc, "remote repo server is configured")
			return r.Delete(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	if !cr.Spec.Repo.IsEnabled() {
		return nil
	}

	// TODO: Existing and current service is not compared and updated
	svc.Spec.Type = corev1.ServiceTypeClusterIP

	_, err = ensureAutoTLSAnnotation(r.Client, svc, common.ArgoCDRepoServerTLSSecretName, cr.Spec.Repo.WantsAutoTLS())
	if err != nil {
		return fmt.Errorf("unable to ensure AutoTLS annotation: %w", err)
	}

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

	if cr.Spec.Repo.IsEnabled() && cr.Spec.Repo.IsRemote() {
		log.Info("skip creating repo server service, repo remote is enabled")
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, svc)
	return r.Create(context.TODO(), svc)
}

// reconcileStatusRepo will ensure that the Repo status is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusRepo(cr *argocdoperatorv1beta1.ArgoCD, argocdStatus *argocdoperatorv1beta1.ArgoCDStatus) error {
	status := "Unknown"

	deploy := newDeploymentWithSuffix("repo-server", "repo-server", cr)
	deplExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy)
	if err != nil {
		argocdStatus.Repo = "Failed"
		return err
	}
	if deplExists {
		status = "Pending"

		if deploy.Spec.Replicas != nil {
			if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
				status = "Running"
			} else if deploy.Status.Conditions != nil {
				for _, condition := range deploy.Status.Conditions {
					if condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue {
						// Deployment has failed
						status = "Failed"
						break
					}
				}
			}
		}
	}

	argocdStatus.Repo = status

	return nil
}

// reconcileRepoServerTLSSecret checks whether the argocd-repo-server-tls secret
// has changed since our last reconciliation loop. It does so by comparing the
// checksum of tls.crt and tls.key in the status of the ArgoCD CR against the
// values calculated from the live state in the cluster.
func (r *ReconcileArgoCD) reconcileRepoServerTLSSecret(cr *argocdoperatorv1beta1.ArgoCD, argocdStatus *argocdoperatorv1beta1.ArgoCDStatus) error {
	var tlsSecretObj corev1.Secret
	var sha256sum string

	log.Info("reconciling repo-server TLS secret")

	tlsSecretName := types.NamespacedName{Namespace: cr.Namespace, Name: common.ArgoCDRepoServerTLSSecretName}
	err := r.Get(context.TODO(), tlsSecretName, &tlsSecretObj)
	if err != nil {
		if !errors.IsNotFound(err) {
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
		// We set the value on ArgoCD status field (where it will be set on cluster object later in the process).
		// This is set to prevent a possible restart loop.
		argocdStatus.RepoTLSChecksum = sha256sum

	}

	return nil
}

// systemCATrustMapper triggers reconciliation of repo-server Deployment if some of the tracked Secrets, ConfigMaps or ClusterTrustBundles have changed
func (r *ReconcileArgoCD) systemCATrustMapper(ctx context.Context, o client.Object) []reconcile.Request {
	// Track Argo CDs whose repo-servers need a rollout, and id of the resource that changed
	rolloutBecause := make(map[*argocdoperatorv1beta1.ArgoCD]string)

	// For cluster-wide resources, it is needed to consult all argos. For cluster-scoped ones, only the argos in the same NS.
	argoNamespace := client.InNamespace(o.GetNamespace())
	var argoCDs argocdoperatorv1beta1.ArgoCDList
	if err := r.List(ctx, &argoCDs, argoNamespace); err != nil {
		log.Error(err, "unable to list ArgoCD instances")
		return []reconcile.Request{}
	}

	for _, argocd := range argoCDs.Items {
		if argocd.Spec.Repo.SystemCATrust == nil {
			continue
		}

		switch obj := o.(type) {
		case *corev1.Secret:
			for _, trustSource := range argocd.Spec.Repo.SystemCATrust.Secrets {
				if trustSource.Name == obj.Name {
					rolloutBecause[&argocd] = fmt.Sprintf("Secret %s/%s", obj.Namespace, obj.Name)
					break
				}
			}
		case *corev1.ConfigMap:
			for _, trustSource := range argocd.Spec.Repo.SystemCATrust.ConfigMaps {
				if trustSource.Name == obj.Name {
					rolloutBecause[&argocd] = fmt.Sprintf("ConfigMap %s/%s", obj.Namespace, obj.Name)
					break
				}
			}
		case *v1beta1.ClusterTrustBundle:
			for _, trustSource := range argocd.Spec.Repo.SystemCATrust.ClusterTrustBundles {
				if isRelevantCtb(trustSource, obj) {
					rolloutBecause[&argocd] = fmt.Sprintf("ClusterTrustBundle %s", obj.Name)
					break
				}
			}
		default:
			panic(fmt.Errorf("systemCATrustMapper called for unknown type %t", o))
		}
	}

	for argocd, cause := range rolloutBecause {
		// Instead of triggering rollout, delete the pod to force trust recomputation
		pods := &corev1.PodList{}
		err := r.List(context.TODO(), pods,
			client.InNamespace(argocd.Namespace),
			client.MatchingLabelsSelector{Selector: labels.SelectorFromSet(map[string]string{
				"app.kubernetes.io/name": nameWithSuffix("repo-server", argocd),
			})},
		)
		if err != nil {
			log.Error(err, "unable to list repo-server pods for argocd", "ns", argocd.Namespace, "name", argocd.Name)
		}

		// In normal circumstances, there would be 1 pod. There can be multiple during ongoing rollout. None if not yet started, or recovering from an error.
		for _, pod := range pods.Items {
			log.Info(
				"restarting repo-server pod after SystemCATrust change in "+cause,
				"pod", pod.Name, "ns", pod.Namespace, "phase", pod.Status.Phase,
			)
			if err := r.Delete(context.TODO(), &pod); err != nil {
				log.Error(err, "unable to delete repo-server pod 1", "pod", pod.Name, "ns", pod.Namespace, "phase", pod.Status.Phase)
			}
		}
	}
	// No need to reconcile. The pods have been restarted
	return []reconcile.Request{}
}

func isRelevantCtb(proj corev1.ClusterTrustBundleProjection, actual *v1beta1.ClusterTrustBundle) bool {
	// ClusterTrustBundle uses either .Name or .SignerName plus eventual .LabelSelector to identify the source
	if proj.Name != nil && *proj.Name == actual.Name {
		return true
	}

	if proj.SignerName != nil && *proj.SignerName == actual.Spec.SignerName {
		// If unset, interpreted as "match nothing".  If set but empty, interpreted as "match everything".
		if proj.LabelSelector == nil {
			return false
		}
		if len(proj.LabelSelector.MatchLabels)+len(proj.LabelSelector.MatchExpressions) == 0 {
			return true
		}

		selector, err := metav1.LabelSelectorAsSelector(proj.LabelSelector)
		if err != nil {
			log.Error(err, "Failed evaluating label selector for System CA trust ClusterTrustBundle", "selector", proj.LabelSelector)
			return false
		}

		return selector.Matches(labels.Set(actual.Labels))
	}

	return false
}
