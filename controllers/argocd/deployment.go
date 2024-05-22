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

// getArgoCDRepoServerReplicas will return the size value for the argocd-repo-server replica count if it
// has been set in argocd CR. Otherwise, nil is returned if the replicas is not set in the argocd CR or
// replicas value is < 0.
func getArgoCDRepoServerReplicas(cr *argoproj.ArgoCD) *int32 {
	if cr.Spec.Repo.Replicas != nil && *cr.Spec.Repo.Replicas >= 0 {
		return cr.Spec.Repo.Replicas
	}

	return nil
}

// getArgoCDServerReplicas will return the size value for the argocd-server replica count if it
// has been set in argocd CR. Otherwise, nil is returned if the replicas is not set in the argocd CR or
// replicas value is < 0. If Autoscale is enabled, the value for replicas in the argocd CR will be ignored.
func getArgoCDServerReplicas(cr *argoproj.ArgoCD) *int32 {
	if !cr.Spec.Server.Autoscale.Enabled && cr.Spec.Server.Replicas != nil && *cr.Spec.Server.Replicas >= 0 {
		return cr.Spec.Server.Replicas
	}
	return nil
}

func (r *ReconcileArgoCD) getArgoCDExport(cr *argoproj.ArgoCD) *argoprojv1alpha1.ArgoCDExport {
	if cr.Spec.Import == nil {
		return nil
	}

	namespace := cr.ObjectMeta.Namespace
	if cr.Spec.Import.Namespace != nil && len(*cr.Spec.Import.Namespace) > 0 {
		namespace = *cr.Spec.Import.Namespace
	}

	export := &argoprojv1alpha1.ArgoCDExport{}
	if argoutil.IsObjectFound(r.Client, namespace, cr.Spec.Import.Name, export) {
		return export
	}
	return nil
}

func getArgoExportSecretName(export *argoprojv1alpha1.ArgoCDExport) string {
	name := argoutil.NameWithSuffix(export.ObjectMeta, "export")
	if export.Spec.Storage != nil && len(export.Spec.Storage.SecretName) > 0 {
		name = export.Spec.Storage.SecretName
	}
	return name
}

func getArgoImportBackend(client client.Client, cr *argoproj.ArgoCD) string {
	backend := common.ArgoCDExportStorageBackendLocal
	namespace := cr.ObjectMeta.Namespace
	if cr.Spec.Import != nil && cr.Spec.Import.Namespace != nil && len(*cr.Spec.Import.Namespace) > 0 {
		namespace = *cr.Spec.Import.Namespace
	}

	export := &argoprojv1alpha1.ArgoCDExport{}
	if argoutil.IsObjectFound(client, namespace, cr.Spec.Import.Name, export) {
		if export.Spec.Storage != nil && len(export.Spec.Storage.Backend) > 0 {
			backend = export.Spec.Storage.Backend
		}
	}
	return backend
}

// getArgoImportCommand will return the command for the ArgoCD import process.
func getArgoImportCommand(client client.Client, cr *argoproj.ArgoCD) []string {
	cmd := make([]string, 0)
	cmd = append(cmd, "uid_entrypoint.sh")
	cmd = append(cmd, "argocd-operator-util")
	cmd = append(cmd, "import")
	cmd = append(cmd, getArgoImportBackend(client, cr))
	return cmd
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

// getArgoRepoCommand will return the command for the ArgoCD Repo component.
func getArgoRepoCommand(cr *argoproj.ArgoCD, useTLSForRedis bool) []string {
	cmd := make([]string, 0)

	cmd = append(cmd, "uid_entrypoint.sh")
	cmd = append(cmd, "argocd-repo-server")

	cmd = append(cmd, "--redis")
	cmd = append(cmd, getRedisServerAddress(cr))

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

// getArgoCmpServerInitCommand will return the command for the ArgoCD CMP Server init container
func getArgoCmpServerInitCommand() []string {
	cmd := make([]string, 0)
	cmd = append(cmd, "cp")
	cmd = append(cmd, "-n")
	cmd = append(cmd, "/usr/local/bin/argocd")
	cmd = append(cmd, "/var/run/argocd/argocd-cmp-server")
	return cmd
}

// getArgoServerCommand will return the command for the ArgoCD server component.
func getArgoServerCommand(cr *argoproj.ArgoCD, useTLSForRedis bool) []string {
	cmd := make([]string, 0)
	cmd = append(cmd, "argocd-server")

	if getArgoServerInsecure(cr) {
		cmd = append(cmd, "--insecure")
	}

	if isRepoServerTLSVerificationRequested(cr) {
		cmd = append(cmd, "--repo-server-strict-tls")
	}

	cmd = append(cmd, "--staticassets")
	cmd = append(cmd, "/shared/app")

	cmd = append(cmd, "--dex-server")
	cmd = append(cmd, getDexServerAddress(cr))

	cmd = append(cmd, "--repo-server")
	cmd = append(cmd, getRepoServerAddress(cr))

	cmd = append(cmd, "--redis")
	cmd = append(cmd, getRedisServerAddress(cr))

	if useTLSForRedis {
		cmd = append(cmd, "--redis-use-tls")
		if isRedisTLSVerificationDisabled(cr) {
			cmd = append(cmd, "--redis-insecure-skip-tls-verify")
		} else {
			cmd = append(cmd, "--redis-ca-certificate", "/app/config/server/tls/redis/tls.crt")
		}
	}

	cmd = append(cmd, "--loglevel")
	cmd = append(cmd, getLogLevel(cr.Spec.Server.LogLevel))

	cmd = append(cmd, "--logformat")
	cmd = append(cmd, getLogFormat(cr.Spec.Server.LogFormat))

	extraArgs := cr.Spec.Server.ExtraCommandArgs
	err := isMergable(extraArgs, cmd)
	if err != nil {
		return cmd
	}
	if cr.Spec.SourceNamespaces != nil && len(cr.Spec.SourceNamespaces) > 0 {
		cmd = append(cmd, "--application-namespaces", fmt.Sprint(strings.Join(cr.Spec.SourceNamespaces, ",")))
	}

	cmd = append(cmd, extraArgs...)
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

// getRepoServerAddress will return the Argo CD repo server address.
func getRepoServerAddress(cr *argoproj.ArgoCD) string {
	return fqdnServiceRef("repo-server", common.ArgoCDDefaultRepoServerPort, cr)
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
	deploy.ObjectMeta.Name = name

	lbls := deploy.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	lbls[common.ArgoCDKeyComponent] = component
	deploy.ObjectMeta.Labels = lbls

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
	return newDeploymentWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), component, cr)
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
	deploy := newDeploymentWithSuffix("grafana", "grafana", cr)
	deploy.Spec.Replicas = getGrafanaReplicas(cr)
	AddSeccompProfileForOpenShift(r.Client, &deploy.Spec.Template.Spec)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Image:           getGrafanaContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "grafana",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 3000,
			},
		},
		Env:       proxyEnvVars(),
		Resources: getGrafanaResources(cr),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			RunAsNonRoot: boolPtr(true),
			RunAsUser:    int64Ptr(472),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "grafana-config",
				MountPath: "/etc/grafana",
			}, {
				Name:      "grafana-datasources-config",
				MountPath: "/etc/grafana/provisioning/datasources",
			}, {
				Name:      "grafana-dashboards-config",
				MountPath: "/etc/grafana/provisioning/dashboards",
			}, {
				Name:      "grafana-dashboard-templates",
				MountPath: "/var/lib/grafana/dashboards",
			},
		},
	}}

	deploy.Spec.Template.Spec.ServiceAccountName = fmt.Sprintf("%s-%s", cr.Name, "argocd-grafana")
	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "grafana-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: nameWithSuffix("grafana-config", cr),
					},
					Items: []corev1.KeyToPath{{
						Key:  "grafana.ini",
						Path: "grafana.ini",
					}},
				},
			},
		}, {
			Name: "grafana-datasources-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: nameWithSuffix("grafana-config", cr),
					},
					Items: []corev1.KeyToPath{{
						Key:  "datasource.yaml",
						Path: "datasource.yaml",
					}},
				},
			},
		}, {
			Name: "grafana-dashboards-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: nameWithSuffix("grafana-config", cr),
					},
					Items: []corev1.KeyToPath{{
						Key:  "provider.yaml",
						Path: "provider.yaml",
					}},
				},
			},
		}, {
			Name: "grafana-dashboard-templates",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: nameWithSuffix("grafana-dashboards", cr),
					},
				},
			},
		},
	}

	existing := newDeploymentWithSuffix("grafana", "grafana", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		if !cr.Spec.Grafana.Enabled {
			// Deployment exists but enabled flag has been set to false, delete the Deployment
			return r.Client.Delete(context.TODO(), existing)
		}
		changed := false
		if hasGrafanaSpecChanged(existing, cr) {
			existing.Spec.Replicas = cr.Spec.Grafana.Size
			changed = true
		}
		updateNodePlacement(existing, deploy, &changed)
		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			deploy.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Resources, existing.Spec.Template.Spec.Containers[0].Resources) {
			existing.Spec.Template.Spec.Containers[0].Resources = deploy.Spec.Template.Spec.Containers[0].Resources
			changed = true
		}
		if changed {
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found, do nothing
	}

	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}
	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileRedisDeployment will ensure the Deployment resource is present for the ArgoCD Redis component.
func (r *ReconcileArgoCD) reconcileRedisDeployment(cr *argoproj.ArgoCD, useTLS bool) error {
	deploy := newDeploymentWithSuffix("redis", "redis", cr)

	env := append(proxyEnvVars(), corev1.EnvVar{
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

	AddSeccompProfileForOpenShift(r.Client, &deploy.Spec.Template.Spec)

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Args:            getArgoRedisArgs(useTLS),
		Image:           getRedisContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "redis",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultRedisPort,
			},
		},
		Resources: getRedisResources(cr),
		Env:       env,
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			RunAsNonRoot: boolPtr(true),
			RunAsUser:    int64Ptr(999),
		},
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
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		if cr.Spec.HA.Enabled {
			// Deployment exists but HA enabled flag has been set to true, delete the Deployment
			return r.Client.Delete(context.TODO(), deploy)
		}
		changed := false
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getRedisContainerImage(cr)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}
		updateNodePlacement(existing, deploy, &changed)

		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Args, existing.Spec.Template.Spec.Containers[0].Args) {
			existing.Spec.Template.Spec.Containers[0].Args = deploy.Spec.Template.Spec.Containers[0].Args
			changed = true
		}

		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			deploy.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			changed = true
		}

		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Resources, existing.Spec.Template.Spec.Containers[0].Resources) {
			existing.Spec.Template.Spec.Containers[0].Resources = deploy.Spec.Template.Spec.Containers[0].Resources
			changed = true
		}

		if changed {
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if cr.Spec.HA.Enabled {
		return nil // HA enabled, do nothing.
	}
	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileRedisHAProxyDeployment will ensure the Deployment resource is present for the Redis HA Proxy component.
func (r *ReconcileArgoCD) reconcileRedisHAProxyDeployment(cr *argoproj.ArgoCD) error {
	deploy := newDeploymentWithSuffix("redis-ha-haproxy", "redis", cr)

	var redisEnv = append(proxyEnvVars(), corev1.EnvVar{
		Name: "AUTH",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: fmt.Sprintf("%s-%s", cr.Name, "redis-initial-password"),
				},
				Key: "admin.password",
			},
		},
	})

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
		ImagePullPolicy: corev1.PullIfNotPresent,
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
		Resources: getRedisHAResources(cr),
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
		ImagePullPolicy: corev1.PullIfNotPresent,
		Name:            "config-init",
		Env:             proxyEnvVars(),
		Resources:       getRedisHAResources(cr),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
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
					SecretName: fmt.Sprintf("%s-%s", cr.Name, "redis-initial-password"),
					Optional:   boolPtr(true),
				},
			},
		},
	}

	deploy.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
		RunAsNonRoot: boolPtr(true),
		RunAsUser:    int64Ptr(1000),
		FSGroup:      int64Ptr(1000),
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
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		if !cr.Spec.HA.Enabled {
			// Deployment exists but HA enabled flag has been set to false, delete the Deployment
			return r.Client.Delete(context.TODO(), existing)
		}
		changed := false
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getRedisHAProxyContainerImage(cr)

		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}
		updateNodePlacement(existing, deploy, &changed)

		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Resources, existing.Spec.Template.Spec.Containers[0].Resources) {
			existing.Spec.Template.Spec.Containers[0].Resources = deploy.Spec.Template.Spec.Containers[0].Resources
			changed = true
		}

		if !reflect.DeepEqual(deploy.Spec.Template.Spec.InitContainers[0].Resources, existing.Spec.Template.Spec.InitContainers[0].Resources) {
			existing.Spec.Template.Spec.InitContainers[0].Resources = deploy.Spec.Template.Spec.InitContainers[0].Resources
			changed = true
		}

		if changed {
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found, do nothing
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
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

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
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
					Name: fmt.Sprintf("%s-%s", cr.Name, "redis-initial-password"),
				},
				Key: "admin.password",
			},
		},
	})
	serverEnv = argoutil.EnvMerge(serverEnv, proxyEnvVars(), false)
	AddSeccompProfileForOpenShift(r.Client, &deploy.Spec.Template.Spec)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command:         getArgoServerCommand(cr, useTLSForRedis),
		Image:           getArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
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
		Resources: getArgoServerResources(cr),
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
		},
	}}
	deploy.Spec.Template.Spec.ServiceAccountName = fmt.Sprintf("%s-%s", cr.Name, "argocd-server")
	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
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
	}

	if replicas := getArgoCDServerReplicas(cr); replicas != nil {
		deploy.Spec.Replicas = replicas
	}

	existing := newDeploymentWithSuffix("server", "server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getArgoContainerImage(cr)
		changed := false
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}
		updateNodePlacement(existing, deploy, &changed)
		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			deploy.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			changed = true
		}
		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Command,
			deploy.Spec.Template.Spec.Containers[0].Command) {
			existing.Spec.Template.Spec.Containers[0].Command = deploy.Spec.Template.Spec.Containers[0].Command
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Volumes, existing.Spec.Template.Spec.Volumes) {
			existing.Spec.Template.Spec.Volumes = deploy.Spec.Template.Spec.Volumes
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].VolumeMounts,
			existing.Spec.Template.Spec.Containers[0].VolumeMounts) {
			existing.Spec.Template.Spec.Containers[0].VolumeMounts = deploy.Spec.Template.Spec.Containers[0].VolumeMounts
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Resources,
			existing.Spec.Template.Spec.Containers[0].Resources) {
			existing.Spec.Template.Spec.Containers[0].Resources = deploy.Spec.Template.Spec.Containers[0].Resources
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Replicas, existing.Spec.Replicas) {
			if !cr.Spec.Server.Autoscale.Enabled {
				existing.Spec.Replicas = deploy.Spec.Replicas
				changed = true
			}
		}
		if changed {
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
}

// triggerDeploymentRollout will update the label with the given key to trigger a new rollout of the Deployment.
func (r *ReconcileArgoCD) triggerDeploymentRollout(deployment *appsv1.Deployment, key string) error {
	if !argoutil.IsObjectFound(r.Client, deployment.Namespace, deployment.Name, deployment) {
		log.Info(fmt.Sprintf("unable to locate deployment with name: %s", deployment.Name))
		return nil
	}

	deployment.Spec.Template.ObjectMeta.Labels[key] = nowNano()
	return r.Client.Update(context.TODO(), deployment)
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
func updateNodePlacement(existing *appsv1.Deployment, deploy *appsv1.Deployment, changed *bool) {
	if !reflect.DeepEqual(existing.Spec.Template.Spec.NodeSelector, deploy.Spec.Template.Spec.NodeSelector) {
		existing.Spec.Template.Spec.NodeSelector = deploy.Spec.Template.Spec.NodeSelector
		*changed = true
	}
	if !reflect.DeepEqual(existing.Spec.Template.Spec.Tolerations, deploy.Spec.Template.Spec.Tolerations) {
		existing.Spec.Template.Spec.Tolerations = deploy.Spec.Template.Spec.Tolerations
		*changed = true
	}
}
