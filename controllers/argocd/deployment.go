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
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// getArgoCDServerReplicas will return the size value for the argocd-server replica count if it
// has been set in argocd CR. Otherwise, nil is returned if the replicas is not set in the argocd CR or
// replicas value is < 0. If Autoscale is enabled, the value for replicas in the argocd CR will be ignored.
func getArgoCDServerReplicas(cr *argoproj.ArgoCD) *int32 {
	if !cr.Spec.Server.Autoscale.Enabled && cr.Spec.Server.Replicas != nil && *cr.Spec.Server.Replicas >= 0 {
		return cr.Spec.Server.Replicas
	}
	return nil
}

func (r *ReconcileArgoCD) getArgoCDExport(cr *argoproj.ArgoCD) (*argoprojv1alpha1.ArgoCDExport, error) {
	if cr.Spec.Import == nil {
		return nil, nil
	}

	namespace := cr.Namespace
	if cr.Spec.Import.Namespace != nil && len(*cr.Spec.Import.Namespace) > 0 {
		namespace = *cr.Spec.Import.Namespace
	}

	export := &argoprojv1alpha1.ArgoCDExport{}
	exists, err := argoutil.IsObjectFound(r.Client, namespace, cr.Spec.Import.Name, export)
	if err != nil {
		return nil, err
	}
	if exists {
		return export, nil
	}
	return nil, nil
}

func getArgoExportSecretName(export *argoprojv1alpha1.ArgoCDExport) string {
	name := argoutil.NameWithSuffix(export.ObjectMeta, "export")
	if export.Spec.Storage != nil && len(export.Spec.Storage.SecretName) > 0 {
		name = export.Spec.Storage.SecretName
	}
	return name
}

func getArgoImportBackend(client client.Client, cr *argoproj.ArgoCD) (string, error) {
	backend := common.ArgoCDExportStorageBackendLocal
	namespace := cr.Namespace
	if cr.Spec.Import != nil && cr.Spec.Import.Namespace != nil && len(*cr.Spec.Import.Namespace) > 0 {
		namespace = *cr.Spec.Import.Namespace
	}

	export := &argoprojv1alpha1.ArgoCDExport{}
	exists, err := argoutil.IsObjectFound(client, namespace, cr.Spec.Import.Name, export)
	if err != nil {
		return "", err
	}
	if exists {
		if export.Spec.Storage != nil && len(export.Spec.Storage.Backend) > 0 {
			backend = export.Spec.Storage.Backend
		}
	}
	return backend, nil
}

// getArgoImportCommand will return the command for the ArgoCD import process.
func getArgoImportCommand(client client.Client, cr *argoproj.ArgoCD) ([]string, error) {
	cmd := make([]string, 0)
	cmd = append(cmd, "uid_entrypoint.sh")
	cmd = append(cmd, "argocd-operator-util")
	cmd = append(cmd, "import")

	args, err := getArgoImportBackend(client, cr)
	if err != nil {
		return nil, err
	}

	cmd = append(cmd, args)
	return cmd, nil
}

func getArgoImportContainerEnv(cr *argoprojv1alpha1.ArgoCDExport) []corev1.EnvVar {
	env := make([]corev1.EnvVar, 0)

	switch cr.Spec.Storage.Backend {
	case common.ArgoCDExportStorageBackendAWS:
		env = append(env, corev1.EnvVar{
			Name: "AWS_ACCESS_KEY_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: argoutil.FetchStorageSecretName(cr),
					},
					Key: "aws.access.key.id",
				},
			},
		})

		env = append(env, corev1.EnvVar{
			Name: "AWS_SECRET_ACCESS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: argoutil.FetchStorageSecretName(cr),
					},
					Key: "aws.secret.access.key",
				},
			},
		})
	}

	return env
}

// getArgoImportContainerImage will return the container image for the Argo CD import process.
func getArgoImportContainerImage(cr *argoprojv1alpha1.ArgoCDExport) string {
	img := common.ArgoCDDefaultExportJobImage
	if len(cr.Spec.Image) > 0 {
		img = cr.Spec.Image
	}

	tag := common.ArgoCDDefaultExportJobVersion
	if len(cr.Spec.Version) > 0 {
		tag = cr.Spec.Version
	}

	return argoutil.CombineImageTag(img, tag)
}

// getArgoImportVolumeMounts will return the VolumneMounts for the given ArgoCDExport.
func getArgoImportVolumeMounts() []corev1.VolumeMount {
	mounts := make([]corev1.VolumeMount, 0)

	mounts = append(mounts, corev1.VolumeMount{
		Name:      "backup-storage",
		MountPath: "/backups",
	})

	mounts = append(mounts, corev1.VolumeMount{
		Name:      "secret-storage",
		MountPath: "/secrets",
	})
	mounts = append(mounts, corev1.VolumeMount{
		Name:      "tmp",
		MountPath: "/tmp",
	})

	return mounts
}

// getArgoImportVolumes will return the Volumes for the given ArgoCDExport.
func getArgoImportVolumes(cr *argoprojv1alpha1.ArgoCDExport) []corev1.Volume {
	volumes := make([]corev1.Volume, 0)

	if cr.Spec.Storage != nil && cr.Spec.Storage.Backend == common.ArgoCDExportStorageBackendLocal {
		volumes = append(volumes, corev1.Volume{
			Name: "backup-storage",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: cr.Name,
				},
			},
		})
	} else {
		volumes = append(volumes, corev1.Volume{
			Name: "backup-storage",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	volumes = append(volumes, corev1.Volume{
		Name: "secret-storage",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: getArgoExportSecretName(cr),
			},
		},
	})

	volumes = append(volumes, corev1.Volume{
		Name: "tmp",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	return volumes
}

func getArgoRedisArgs(useTLS bool) []string {
	args := make([]string, 0)

	args = append(args, "--save", "")
	args = append(args, "--appendonly", "no")
	args = append(args, "--requirepass $(REDIS_PASSWORD)")

	if useTLS {
		args = append(args, "--tls-port", "6379")
		args = append(args, "--port", "0")

		args = append(args, "--tls-cert-file", "/app/config/redis/tls/tls.crt")
		args = append(args, "--tls-key-file", "/app/config/redis/tls/tls.key")
		args = append(args, "--tls-auth-clients", "no")
	}

	return args
}

// getArgoCmpServerInitCommand will return the command for the ArgoCD CMP Server init container
func getArgoCmpServerInitCommand() []string {
	cmd := make([]string, 0)
	cmd = append(cmd, "cp")
	cmd = append(cmd, "-n")
	cmd = append(cmd, "/usr/local/bin/argocd-cmp-server")
	cmd = append(cmd, "/var/run/argocd/argocd-cmp-server")
	return cmd
}

// getArgoServerCommand will return the command for the ArgoCD server component.
func getArgoServerCommand(cr *argoproj.ArgoCD, useTLSForRedis bool) []string {

	allowed := argoutil.IsNamespaceClusterConfigNamespace(cr.Namespace)

	cmd := make([]string, 0)
	cmd = append(cmd, "argocd-server")

	if getArgoServerInsecure(cr) {
		cmd = append(cmd, "--insecure")
	}

	if isRepoServerTLSVerificationRequested(cr) {
		cmd = append(cmd, "--repo-server-strict-tls")
	}

	cmd = append(cmd, "--staticassets", "/shared/app")

	cmd = append(cmd, "--dex-server", getDexServerAddress(cr))

	if cr.Spec.Repo.IsEnabled() {
		cmd = append(cmd, "--repo-server", getRepoServerAddress(cr))
	} else {
		log.Info("Repo Server is disabled. This would affect the functioning of ArgoCD Server.")
	}

	if cr.Spec.Redis.IsEnabled() {
		cmd = append(cmd, "--redis", getRedisServerAddress(cr))
	} else {
		log.Info("Redis is Disabled. Skipping adding Redis configuration to ArgoCD Server.")
	}

	if useTLSForRedis {
		cmd = append(cmd, "--redis-use-tls")
		if isRedisTLSVerificationDisabled(cr) {
			cmd = append(cmd, "--redis-insecure-skip-tls-verify")
		} else {
			cmd = append(cmd, "--redis-ca-certificate", "/app/config/server/tls/redis/tls.crt")
		}
	}

	cmd = append(cmd, "--loglevel", getLogLevel(cr.Spec.Server.LogLevel))
	cmd = append(cmd, "--logformat", getLogFormat(cr.Spec.Server.LogFormat))

	// Merge extraArgs while ignoring duplicates
	extraArgs := cr.Spec.Server.ExtraCommandArgs
	cmd = appendUniqueArgs(cmd, extraArgs)

	if len(cr.Spec.SourceNamespaces) > 0 && allowed {
		cmd = append(cmd, "--application-namespaces", fmt.Sprint(strings.Join(cr.Spec.SourceNamespaces, ",")))
	}

	return cmd
}

// isMergable returns error if any of the extraArgs is already part of the default command Arguments.
func isMergable(extraArgs []string, cmd []string) error {
	if len(extraArgs) > 0 {
		for _, arg := range extraArgs {
			if len(arg) > 2 && arg[:2] == "--" {
				if ok := contains(cmd, arg); ok {
					err := errors.New("duplicate argument error")
					log.Error(err, fmt.Sprintf("Arg %s is already part of the default command arguments", arg))
					return err
				}
			}
		}
	}
	return nil
}

// getDexServerAddress will return the Dex server address.
func getDexServerAddress(cr *argoproj.ArgoCD) string {
	return fmt.Sprintf("https://%s", fqdnServiceRef("dex-server", common.ArgoCDDefaultDexHTTPPort, cr))
}

// newDeployment returns a new Deployment instance for the given ArgoCD.
func newDeployment(cr *argoproj.ArgoCD) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
	}
}

// newDeploymentWithName returns a new Deployment instance for the given ArgoCD using the given name.
func newDeploymentWithName(name string, component string, cr *argoproj.ArgoCD) *appsv1.Deployment {
	deploy := newDeployment(cr)

	// The name is already truncated by nameWithSuffix, so use it directly
	deploy.Name = name

	lbls := deploy.Labels
	lbls[common.ArgoCDKeyName] = name
	lbls[common.ArgoCDKeyComponent] = component
	deploy.Labels = lbls

	deploy.Spec = appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				common.ArgoCDKeyName: name,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					common.ArgoCDKeyName: name,
				},
				Annotations: make(map[string]string),
			},
			Spec: corev1.PodSpec{
				NodeSelector: common.DefaultNodeSelector(),
			},
		},
	}

	if cr.Spec.NodePlacement != nil {
		deploy.Spec.Template.Spec.NodeSelector = argoutil.AppendStringMap(deploy.Spec.Template.Spec.NodeSelector, cr.Spec.NodePlacement.NodeSelector)
		deploy.Spec.Template.Spec.Tolerations = cr.Spec.NodePlacement.Tolerations
	}
	return deploy
}

// newDeploymentWithSuffix returns a new Deployment instance for the given ArgoCD using the given suffix.
func newDeploymentWithSuffix(suffix string, component string, cr *argoproj.ArgoCD) *appsv1.Deployment {
	return newDeploymentWithName(nameWithSuffix(suffix, cr), component, cr)
}

// reconcileDeployments will ensure that all Deployment resources are present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileDeployments(cr *argoproj.ArgoCD, useTLSForRedis bool) error {

	if err := r.reconcileDexDeployment(cr); err != nil {
		log.Error(err, "error reconciling dex deployment")
	}

	err := r.reconcileRedisDeployment(cr, useTLSForRedis)
	if err != nil {
		return err
	}

	err = r.reconcileRedisHAProxyDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRepoDeployment(cr, useTLSForRedis)
	if err != nil {
		return err
	}

	err = r.reconcileServerDeployment(cr, useTLSForRedis)
	if err != nil {
		return err
	}

	err = r.reconcileGrafanaDeployment(cr)
	if err != nil {
		return err
	}

	return nil
}

// reconcileGrafanaDeployment will ensure the Deployment resource is present for the ArgoCD Grafana component.
func (r *ReconcileArgoCD) reconcileGrafanaDeployment(cr *argoproj.ArgoCD) error {

	//lint:ignore SA1019 known to be deprecated
	if !cr.Spec.Grafana.Enabled { //nolint:staticcheck // SA1019: We must test deprecated fields.
		return nil // Grafana not enabled, do nothing.
	}
	log.Info(grafanaDeprecatedWarning)
	return nil
}

// reconcileRedisDeployment will ensure the Deployment resource is present for the ArgoCD Redis component.
func (r *ReconcileArgoCD) reconcileRedisDeployment(cr *argoproj.ArgoCD, useTLS bool) error {
	deploy := newDeploymentWithSuffix("redis", "redis", cr)

	env := append(proxyEnvVars(), corev1.EnvVar{
		Name: "REDIS_PASSWORD",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: argoutil.GetSecretNameWithSuffix(cr, "redis-initial-password"),
				},
				Key: "admin.password",
			},
		},
	})

	AddSeccompProfileForOpenShift(r.Client, &deploy.Spec.Template.Spec)

	if !IsOpenShiftCluster() {
		deploy.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsUser: int64Ptr(1000),
		}
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Args:            getArgoRedisArgs(useTLS),
		Image:           getRedisContainerImage(cr),
		ImagePullPolicy: argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy),
		Name:            "redis",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultRedisPort,
			},
		},
		Resources:       getRedisResources(cr),
		Env:             env,
		SecurityContext: argoutil.DefaultSecurityContext(),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      common.ArgoCDRedisServerTLSSecretName,
				MountPath: "/app/config/redis/tls",
			},
		},
	}}

	deploy.Spec.Template.Spec.ServiceAccountName = fmt.Sprintf("%s-%s", cr.Name, "argocd-redis")
	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: common.ArgoCDRedisServerTLSSecretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRedisServerTLSSecretName,
					Optional:   boolPtr(true),
				},
			},
		},
	}

	if err := applyReconcilerHook(cr, deploy, ""); err != nil {
		return err
	}

	existing := newDeploymentWithSuffix("redis", "redis", cr)
	deplFound, err := argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing)
	if err != nil {
		return err
	}
	if deplFound {
		if !cr.Spec.Redis.IsEnabled() {
			// Deployment exists but component enabled flag has been set to false, delete the Deployment
			argoutil.LogResourceDeletion(log, deploy, "redis is disabled but deployment exists")
			return r.Delete(context.TODO(), deploy)
		} else if cr.Spec.Redis.IsRemote() {
			argoutil.LogResourceDeletion(log, deploy, "remote redis is configured")
			return r.Delete(context.TODO(), deploy)
		}
		if cr.Spec.HA.Enabled {
			// Deployment exists but HA enabled flag has been set to true, delete the Deployment
			argoutil.LogResourceDeletion(log, deploy, "redis ha is enabled but non-ha deployment exists")
			return r.Delete(context.TODO(), deploy)
		}
		changed := false
		explanation := ""
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getRedisContainerImage(cr)
		actualImagePullPolicy := existing.Spec.Template.Spec.Containers[0].ImagePullPolicy
		desiredImagePullPolicy := argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
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

		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Args, existing.Spec.Template.Spec.Containers[0].Args) {
			existing.Spec.Template.Spec.Containers[0].Args = deploy.Spec.Template.Spec.Containers[0].Args
			if changed {
				explanation += ", "
			}
			explanation += "container args"
			changed = true
		}

		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			deploy.Spec.Template.Spec.Containers[0].Env) {
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

		if !reflect.DeepEqual(deploy.Spec.Template.Spec.ServiceAccountName, existing.Spec.Template.Spec.ServiceAccountName) {
			existing.Spec.Template.Spec.ServiceAccountName = deploy.Spec.Template.Spec.ServiceAccountName
			if changed {
				explanation += ", "
			}
			explanation += "serviceAccountName"
			changed = true
		}

		if changed {
			argoutil.LogResourceUpdate(log, existing, "updating", explanation)
			return r.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if cr.Spec.Redis.IsEnabled() && cr.Spec.Redis.IsRemote() {
		log.Info("Custom Redis Endpoint. Skipping starting redis.")
		return nil
	}

	if !cr.Spec.Redis.IsEnabled() {
		log.Info("Redis disabled. Skipping starting redis.")
		return nil
	}

	if cr.Spec.HA.Enabled {
		return nil // HA enabled, do nothing.
	}
	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, deploy)
	return r.Create(context.TODO(), deploy)
}

// reconcileRedisHAProxyDeployment will ensure the Deployment resource is present for the Redis HA Proxy component.
func (r *ReconcileArgoCD) reconcileRedisHAProxyDeployment(cr *argoproj.ArgoCD) error {
	deploy := newDeploymentWithSuffix("redis-ha-haproxy", "redis", cr)
	deploy.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &appsv1.RollingUpdateDeployment{
			MaxSurge: &intstr.IntOrString{IntVal: 0},
		},
	}

	var redisEnv = proxyEnvVars()

	deploy.Spec.Replicas = getRedisHAReplicas()

	deploy.Spec.Template.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								common.ArgoCDKeyName: nameWithSuffix("redis-ha-haproxy", cr),
							},
						},
						TopologyKey: common.ArgoCDKeyFailureDomainZone,
					},
					Weight: int32(100),
				},
			},
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.ArgoCDKeyName: nameWithSuffix("redis-ha-haproxy", cr),
						},
					},
					TopologyKey: common.ArgoCDKeyHostname,
				},
			},
		},
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Image:           getRedisHAProxyContainerImage(cr),
		ImagePullPolicy: argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy),
		Name:            "haproxy",
		Env:             redisEnv,
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
				ContainerPort: common.ArgoCDDefaultRedisPort,
				Name:          "redis",
			},
		},
		Resources:       getRedisHAResources(cr),
		SecurityContext: argoutil.DefaultSecurityContext(),
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
				MountPath: "/app/config/redis/tls",
			},
			{
				Name:      "redis-initial-pass",
				MountPath: "/redis-initial-pass",
			},
		},
	}}

	deploy.Spec.Template.Spec.InitContainers = []corev1.Container{{
		Args: []string{
			"/readonly/haproxy_init.sh",
		},
		Command: []string{
			"sh",
		},
		Image:           getRedisHAProxyContainerImage(cr),
		ImagePullPolicy: argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy),
		Name:            "config-init",
		Env:             proxyEnvVars(),
		Resources:       getRedisHAResources(cr),
		SecurityContext: argoutil.DefaultSecurityContext(),
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
			{
				Name:      "redis-initial-pass",
				MountPath: "/redis-initial-pass",
			},
		},
	}}

	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
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
					Optional:   boolPtr(true),
				},
			},
		},
		{
			Name: "redis-initial-pass",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: argoutil.GetSecretNameWithSuffix(cr, "redis-initial-password"),
				},
			},
		},
	}

	if IsOpenShiftCluster() {
		deploy.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsNonRoot: boolPtr(true),
			SeccompProfile: &corev1.SeccompProfile{
				Type: "RuntimeDefault",
			},
		}
	} else {
		deploy.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsNonRoot: boolPtr(true),
			RunAsUser:    int64Ptr(1000),
			FSGroup:      int64Ptr(1000),
			SeccompProfile: &corev1.SeccompProfile{
				Type: "RuntimeDefault",
			},
		}
	}
	AddSeccompProfileForOpenShift(r.Client, &deploy.Spec.Template.Spec)

	deploy.Spec.Template.Spec.ServiceAccountName = fmt.Sprintf("%s-%s", cr.Name, "argocd-redis-ha")

	version, err := getClusterVersion(r.Client)
	if err != nil {
		log.Error(err, "error getting cluster version")
	}
	if err := applyReconcilerHook(cr, deploy, version); err != nil {
		return err
	}

	existing := newDeploymentWithSuffix("redis-ha-haproxy", "redis", cr)
	deplExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing)
	if err != nil {
		return err
	}
	if deplExists {
		if !cr.Spec.HA.Enabled {
			// Deployment exists but HA enabled flag has been set to false, delete the Deployment
			argoutil.LogResourceDeletion(log, existing, "redis ha is disabled")
			return r.Delete(context.TODO(), existing)
		}
		changed := false
		explanation := ""
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getRedisHAProxyContainerImage(cr)
		actualImagePullPolicy := existing.Spec.Template.Spec.Containers[0].ImagePullPolicy
		desiredImagePullPolicy := argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy)

		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
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
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.InitContainers, existing.Spec.Template.Spec.InitContainers) {
			existing.Spec.Template.Spec.InitContainers = deploy.Spec.Template.Spec.InitContainers
			if changed {
				explanation += ", "
			}
			explanation += "init containers"
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
		if !reflect.DeepEqual(deploy.Spec.Strategy, existing.Spec.Strategy) {
			existing.Spec.Strategy = deploy.Spec.Strategy
			if changed {
				explanation += ", "
			}
			explanation += "deployment strategy"
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
		if changed {
			argoutil.LogResourceUpdate(log, existing, "updating", explanation)
			return r.Update(context.TODO(), existing)
		}
		return nil // Deployment found, do nothing
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, deploy)
	return r.Create(context.TODO(), deploy)
}

// reconcileServerDeployment will ensure the Deployment resource is present for the ArgoCD Server component.
func (r *ReconcileArgoCD) reconcileServerDeployment(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	deploy := newDeploymentWithSuffix("server", "server", cr)
	serverEnv := cr.Spec.Server.Env
	serverEnv = append(serverEnv, corev1.EnvVar{
		Name: "REDIS_PASSWORD",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: argoutil.GetSecretNameWithSuffix(cr, "redis-initial-password"),
				},
				Key: "admin.password",
			},
		},
	})
	serverEnv = argoutil.EnvMerge(serverEnv, proxyEnvVars(), false)
	AddSeccompProfileForOpenShift(r.Client, &deploy.Spec.Template.Spec)

	if cr.Spec.Server.InitContainers != nil {
		deploy.Spec.Template.Spec.InitContainers = append(deploy.Spec.Template.Spec.InitContainers, cr.Spec.Server.InitContainers...)
	}

	serverVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "ssh-known-hosts",
			MountPath: "/app/config/ssh",
		}, {
			Name:      "tls-certs",
			MountPath: "/app/config/tls",
		},
		{
			Name:      "argocd-repo-server-tls",
			MountPath: "/app/config/server/tls",
		},
		{
			Name:      common.ArgoCDRedisServerTLSSecretName,
			MountPath: "/app/config/server/tls/redis",
		},
		{
			Name:      "plugins-home",
			MountPath: "/home/argocd",
		},
		{
			Name:      "argocd-cmd-params-cm",
			MountPath: "/home/argocd/params",
		},
		{
			Name:      "tmp",
			MountPath: "/tmp",
		},
	}

	if cr.Spec.Server.VolumeMounts != nil {
		serverVolumeMounts = append(serverVolumeMounts, cr.Spec.Server.VolumeMounts...)
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command:         getArgoServerCommand(cr, useTLSForRedis),
		Image:           getArgoContainerImage(cr),
		ImagePullPolicy: argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy),
		Env:             serverEnv,
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
		Name: "argocd-server",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 8080,
			}, {
				ContainerPort: 8083,
			},
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
		Resources:       getArgoServerResources(cr),
		SecurityContext: argoutil.DefaultSecurityContext(),
		VolumeMounts:    serverVolumeMounts,
	}}
	deploy.Spec.Template.Spec.ServiceAccountName = fmt.Sprintf("%s-%s", cr.Name, "argocd-server")

	serverVolumes := []corev1.Volume{
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
			Name: "plugins-home",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "argocd-cmd-params-cm",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "argocd-cmd-params-cm",
					},
					Optional: boolPtr(true),
					Items: []corev1.KeyToPath{
						{
							Key:  "server.profile.enabled",
							Path: "profiler.enabled",
						},
					},
				},
			},
		},
		{
			Name: "tmp",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	if cr.Spec.Server.Volumes != nil {
		serverVolumes = append(serverVolumes, cr.Spec.Server.Volumes...)
	}

	deploy.Spec.Template.Spec.Volumes = serverVolumes

	const rolloutsVolumeName = "rollout-extensions"
	if cr.Spec.Server.EnableRolloutsUI {
		deploy.Spec.Template.Spec.InitContainers = append(deploy.Spec.Template.Spec.InitContainers, getRolloutInitContainer()...)

		deploy.Spec.Template.Spec.Containers[0].VolumeMounts = append(deploy.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      rolloutsVolumeName,
			MountPath: "/tmp/extensions/",
		})

		deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: rolloutsVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	} else if !cr.Spec.Server.EnableRolloutsUI {
		deploy.Spec.Template.Spec.InitContainers = removeInitContainer(deploy.Spec.Template.Spec.InitContainers, rolloutsVolumeName)
		deploy.Spec.Template.Spec.Volumes = removeVolume(deploy.Spec.Template.Spec.Volumes, rolloutsVolumeName)
		deploy.Spec.Template.Spec.Containers[0].VolumeMounts = removeVolumeMount(deploy.Spec.Template.Spec.Containers[0].VolumeMounts, rolloutsVolumeName)
	}

	if replicas := getArgoCDServerReplicas(cr); replicas != nil {
		deploy.Spec.Replicas = replicas

		// Add ARGOCD_API_SERVER_REPLICAS env var to the argocd-server container
		deploy.Spec.Template.Spec.Containers[0].Env = append(
			deploy.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name:  "ARGOCD_API_SERVER_REPLICAS",
				Value: fmt.Sprintf("%d", *replicas),
			},
		)
	}

	if cr.Spec.Server.SidecarContainers != nil {
		deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers, cr.Spec.Server.SidecarContainers...)
	}

	if cr.Spec.Server.Annotations != nil {
		for key, value := range cr.Spec.Server.Annotations {
			deploy.Spec.Template.Annotations[key] = value
		}
	}

	if cr.Spec.Server.Labels != nil {
		for key, value := range cr.Spec.Server.Labels {
			deploy.Spec.Template.Labels[key] = value
		}
	}
	if err := applyReconcilerHook(cr, deploy, ""); err != nil {
		return err
	}

	existing := newDeploymentWithSuffix("server", "server", cr)
	deplExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing)
	if err != nil {
		return err
	}
	if deplExists {
		if !cr.Spec.Server.IsEnabled() {
			// Delete existing deployment for ArgoCD Server, if any ..
			argoutil.LogResourceDeletion(log, existing, "argocd server is disabled")
			return r.Delete(context.TODO(), existing)
		}
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getArgoContainerImage(cr)
		actualImagePullPolicy := existing.Spec.Template.Spec.Containers[0].ImagePullPolicy
		desiredImagePullPolicy := argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy)
		changed := false
		explanation := ""
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
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
		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			deploy.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			if changed {
				explanation += ", "
			}
			explanation += "container env"
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
		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Command,
			deploy.Spec.Template.Spec.Containers[0].Command) {
			existing.Spec.Template.Spec.Containers[0].Command = deploy.Spec.Template.Spec.Containers[0].Command
			if changed {
				explanation += ", "
			}
			explanation += "container command"
			changed = true
		}
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
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Resources,
			existing.Spec.Template.Spec.Containers[0].Resources) {
			existing.Spec.Template.Spec.Containers[0].Resources = deploy.Spec.Template.Spec.Containers[0].Resources
			if changed {
				explanation += ", "
			}
			explanation += "container resources"
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].SecurityContext,
			existing.Spec.Template.Spec.Containers[0].SecurityContext) {
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
		if !reflect.DeepEqual(deploy.Spec.Replicas, existing.Spec.Replicas) {
			if !cr.Spec.Server.Autoscale.Enabled {
				existing.Spec.Replicas = deploy.Spec.Replicas
				if changed {
					explanation += ", "
				}
				explanation += "replicas"
				changed = true
			}
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

	if !cr.Spec.Server.IsEnabled() {
		log.Info("ArgoCD Server disabled. Skipping starting argocd server.")
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, deploy)
	return r.Create(context.TODO(), deploy)
}

// triggerDeploymentRollout will update the label with the given key to trigger a new rollout of the Deployment.
func (r *ReconcileArgoCD) triggerDeploymentRollout(deployment *appsv1.Deployment, key string) error {

	deplExists, err := argoutil.IsObjectFound(r.Client, deployment.Namespace, deployment.Name, deployment)
	if err != nil {
		return err
	}
	if !deplExists {
		log.Info(fmt.Sprintf("unable to locate deployment with name: %s", deployment.Name))
		return nil
	}

	deployment.Spec.Template.Labels[key] = nowNano()
	argoutil.LogResourceUpdate(log, deployment, "to trigger rollout")
	return r.Update(context.TODO(), deployment)
}

func proxyEnvVars(vars ...corev1.EnvVar) []corev1.EnvVar {
	result := []corev1.EnvVar{}
	result = append(result, vars...)
	proxyKeys := []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"}
	for _, p := range proxyKeys {
		if k, v := caseInsensitiveGetenv(p); k != "" {
			result = append(result, corev1.EnvVar{Name: k, Value: v})
		}
	}
	return result
}

func caseInsensitiveGetenv(s string) (string, string) {
	if v := os.Getenv(s); v != "" {
		return s, v
	}
	ls := strings.ToLower(s)
	if v := os.Getenv(ls); v != "" {
		return ls, v
	}
	return "", ""
}

func isRemoveManagedByLabelOnArgoCDDeletion() bool {
	if v := os.Getenv("REMOVE_MANAGED_BY_LABEL_ON_ARGOCD_DELETION"); v != "" {
		return strings.ToLower(v) == "true"
	}
	return false
}

// to update nodeSelector and tolerations in reconciler
func updateNodePlacement(existing *appsv1.Deployment, deploy *appsv1.Deployment, changed *bool, explanation *string) {
	if !reflect.DeepEqual(existing.Spec.Template.Spec.NodeSelector, deploy.Spec.Template.Spec.NodeSelector) {
		existing.Spec.Template.Spec.NodeSelector = deploy.Spec.Template.Spec.NodeSelector
		if *changed {
			*explanation += ", "
		}
		*explanation += "node selector"
		*changed = true
	}
	if !reflect.DeepEqual(existing.Spec.Template.Spec.Tolerations, deploy.Spec.Template.Spec.Tolerations) {
		existing.Spec.Template.Spec.Tolerations = deploy.Spec.Template.Spec.Tolerations
		if *changed {
			*explanation += ", "
		}
		*explanation += "tolerations"
		*changed = true
	}
}

func getRolloutInitContainer() []corev1.Container {
	containers := []corev1.Container{
		{
			Name: "rollout-extension",
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "rollout-extensions",
					MountPath: "/tmp/extensions/",
				},
				{
					Name:      "tmp",
					MountPath: "/tmp",
				},
			},
			SecurityContext: argoutil.DefaultSecurityContext(),
		},
	}

	if value, exists := os.LookupEnv(common.ArgoCDExtensionImageEnvName); exists {
		containers[0].Image = value
	} else {
		containers[0].Image = common.ArgoCDExtensionInstallerImage
		containers[0].Env = []corev1.EnvVar{
			{
				Name:  "EXTENSION_URL",
				Value: common.ArgoRolloutsExtensionURL,
			}}
	}
	return containers
}

func removeInitContainer(initContainers []corev1.Container, name string) []corev1.Container {
	for i, container := range initContainers {
		if container.Name == name {
			// Remove the init container by slicing it out
			return append(initContainers[:i], initContainers[i+1:]...)
		}
	}
	// If the init container is not found, return the original list
	return initContainers
}

func removeVolume(volumes []corev1.Volume, name string) []corev1.Volume {
	for i, volume := range volumes {
		if volume.Name == name {
			return append(volumes[:i], volumes[i+1:]...)
		}
	}
	return volumes
}

func removeVolumeMount(volumeMounts []corev1.VolumeMount, name string) []corev1.VolumeMount {
	for i, volumeMount := range volumeMounts {
		if volumeMount.Name == name {
			return append(volumeMounts[:i], volumeMounts[i+1:]...)
		}
	}
	return volumeMounts
}
