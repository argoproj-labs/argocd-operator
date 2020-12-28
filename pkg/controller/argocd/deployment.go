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
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ReconcileArgoCD) getArgoCDExport(cr *argoprojv1a1.ArgoCD) *argoprojv1a1.ArgoCDExport {
	if cr.Spec.Import == nil {
		return nil
	}

	namespace := cr.ObjectMeta.Namespace
	if cr.Spec.Import.Namespace != nil && len(*cr.Spec.Import.Namespace) > 0 {
		namespace = *cr.Spec.Import.Namespace
	}

	export := &argoprojv1a1.ArgoCDExport{}
	if argoutil.IsObjectFound(r.client, namespace, cr.Spec.Import.Name, export) {
		return export
	}
	return nil
}

// getArgoApplicationControllerCommand will return the command for the ArgoCD Application Controller component.
func getArgoApplicationControllerCommand(cr *argoprojv1a1.ArgoCD) []string {
	cmd := []string{
		"argocd-application-controller",
		"--operation-processors", fmt.Sprint(getArgoServerOperationProcessors(cr)),
		"--redis", getRedisServerAddress(cr),
		"--repo-server", nameWithSuffix("repo-server:8081", cr),
		"--status-processors", fmt.Sprint(getArgoServerStatusProcessors(cr)),
	}
	if cr.Spec.Controller.AppSync != nil {
		cmd = append(cmd, "--app-resync", strconv.FormatInt(int64(cr.Spec.Controller.AppSync.Seconds()), 10))
	}
	return cmd
}

func getArgoExportSecretName(export *argoprojv1a1.ArgoCDExport) string {
	name := argoutil.NameWithSuffix(export.ObjectMeta, "export")
	if export.Spec.Storage != nil && len(export.Spec.Storage.SecretName) > 0 {
		name = export.Spec.Storage.SecretName
	}
	return name
}

func getArgoImportBackend(client client.Client, cr *argoprojv1a1.ArgoCD) string {
	backend := common.ArgoCDExportStorageBackendLocal
	namespace := cr.ObjectMeta.Namespace
	if cr.Spec.Import != nil && cr.Spec.Import.Namespace != nil && len(*cr.Spec.Import.Namespace) > 0 {
		namespace = *cr.Spec.Import.Namespace
	}

	export := &argoprojv1a1.ArgoCDExport{}
	if argoutil.IsObjectFound(client, namespace, cr.Spec.Import.Name, export) {
		if export.Spec.Storage != nil && len(export.Spec.Storage.Backend) > 0 {
			backend = export.Spec.Storage.Backend
		}
	}
	return backend
}

// getArgoImportCommand will return the command for the ArgoCD import process.
func getArgoImportCommand(client client.Client, cr *argoprojv1a1.ArgoCD) []string {
	cmd := make([]string, 0)
	cmd = append(cmd, "uid_entrypoint.sh")
	cmd = append(cmd, "argocd-operator-util")
	cmd = append(cmd, "import")
	cmd = append(cmd, getArgoImportBackend(client, cr))
	return cmd
}

func getArgoImportContainerEnv(cr *argoprojv1a1.ArgoCDExport) []corev1.EnvVar {
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
func getArgoImportContainerImage(cr *argoprojv1a1.ArgoCDExport) string {
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
func getArgoImportVolumeMounts(cr *argoprojv1a1.ArgoCDExport) []corev1.VolumeMount {
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
func getArgoImportVolumes(cr *argoprojv1a1.ArgoCDExport) []corev1.Volume {
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

// getArgoRepoCommand will return the command for the ArgoCD Repo component.
func getArgoRepoCommand(cr *argoprojv1a1.ArgoCD) []string {
	cmd := make([]string, 0)

	cmd = append(cmd, "uid_entrypoint.sh")
	cmd = append(cmd, "argocd-repo-server")

	cmd = append(cmd, "--redis")
	cmd = append(cmd, getRedisServerAddress(cr))

	return cmd
}

// getArgoServerCommand will return the command for the ArgoCD server component.
func getArgoServerCommand(cr *argoprojv1a1.ArgoCD) []string {
	cmd := make([]string, 0)
	cmd = append(cmd, "argocd-server")

	if getArgoServerInsecure(cr) {
		cmd = append(cmd, "--insecure")
	}

	cmd = append(cmd, "--staticassets")
	cmd = append(cmd, "/shared/app")

	cmd = append(cmd, "--dex-server")
	cmd = append(cmd, getDexServerAddress(cr))

	cmd = append(cmd, "--repo-server")
	cmd = append(cmd, getRepoServerAddress(cr))

	cmd = append(cmd, "--redis")
	cmd = append(cmd, getRedisServerAddress(cr))

	return cmd
}

// getDexServerAddress will return the Dex server address.
func getDexServerAddress(cr *argoprojv1a1.ArgoCD) string {
	return fmt.Sprintf("http://%s:%d", nameWithSuffix("dex-server", cr), common.ArgoCDDefaultDexHTTPPort)
}

// getRepoServerAddress will return the Argo CD repo server address.
func getRepoServerAddress(cr *argoprojv1a1.ArgoCD) string {
	return fmt.Sprintf("%s:%d", nameWithSuffix("repo-server", cr), common.ArgoCDDefaultRepoServerPort)
}

// newDeployment returns a new Deployment instance for the given ArgoCD.
func newDeployment(cr *argoprojv1a1.ArgoCD) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}

// newDeploymentWithName returns a new Deployment instance for the given ArgoCD using the given name.
func newDeploymentWithName(name string, component string, cr *argoprojv1a1.ArgoCD) *appsv1.Deployment {
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
		},
	}

	return deploy
}

// newDeploymentWithSuffix returns a new Deployment instance for the given ArgoCD using the given suffix.
func newDeploymentWithSuffix(suffix string, component string, cr *argoprojv1a1.ArgoCD) *appsv1.Deployment {
	return newDeploymentWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), component, cr)
}

// reconcileApplicationControllerDeployment will ensure the Deployment resource is present for the ArgoCD Application Controller component.
func (r *ReconcileArgoCD) reconcileApplicationControllerDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := newDeploymentWithSuffix("application-controller", "application-controller", cr)
	podSpec := &deploy.Spec.Template.Spec
	podSpec.Containers = []corev1.Container{{
		Command:         getArgoApplicationControllerCommand(cr),
		Image:           getArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "argocd-application-controller",
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8082),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		Env: proxyEnvVars(),
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 8082,
			},
		},
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8082),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		Resources: getArgoApplicationControllerResources(cr),
	}}

	// Handle import/restore from ArgoCDExport
	export := r.getArgoCDExport(cr)
	if export == nil {
		log.Info("existing argocd export not found, skipping import")
	} else {
		podSpec.InitContainers = []corev1.Container{{
			Command:         getArgoImportCommand(r.client, cr),
			Env:             proxyEnvVars(getArgoImportContainerEnv(export)...),
			Image:           getArgoImportContainerImage(export),
			ImagePullPolicy: corev1.PullAlways,
			Name:            "argocd-import",
			VolumeMounts:    getArgoImportVolumeMounts(export),
		}}

		podSpec.Volumes = getArgoImportVolumes(export)
	}

	podSpec.ServiceAccountName = fmt.Sprintf("%s-%s", cr.Name, "argocd-application-controller")

	existing := newDeploymentWithSuffix("application-controller", "application-controller", cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, existing.Name, existing) {
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getArgoContainerImage(cr)
		changed := false
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}
		desiredCommand := getArgoApplicationControllerCommand(cr)
		if !reflect.DeepEqual(desiredCommand, existing.Spec.Template.Spec.Containers[0].Command) {
			existing.Spec.Template.Spec.Containers[0].Command = desiredCommand
			changed = true
		}

		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			deploy.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			changed = true
		}

		if changed {
			return r.client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), deploy)
}

// reconcileDeployments will ensure that all Deployment resources are present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileDeployments(cr *argoprojv1a1.ArgoCD) error {
	err := r.reconcileApplicationControllerDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileDexDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRedisDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRedisHAProxyDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRepoDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileServerDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileGrafanaDeployment(cr)
	if err != nil {
		return err
	}

	return nil
}

// reconcileDexDeployment will ensure the Deployment resource is present for the ArgoCD Dex component.
func (r *ReconcileArgoCD) reconcileDexDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := newDeploymentWithSuffix("dex-server", "dex-server", cr)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command: []string{
			"/shared/argocd-util",
			"rundex",
		},
		Image:           getDexContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "dex",
		Env:             proxyEnvVars(),
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultDexHTTPPort,
				Name:          "http",
			}, {
				ContainerPort: common.ArgoCDDefaultDexGRPCPort,
				Name:          "grpc",
			},
		},
		Resources: getDexResources(cr),
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "static-files",
			MountPath: "/shared",
		}},
	}}

	deploy.Spec.Template.Spec.InitContainers = []corev1.Container{{
		Command: []string{
			"cp",
			"/usr/local/bin/argocd-util",
			"/shared",
		},
		Env:             proxyEnvVars(),
		Image:           getArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "copyutil",
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "static-files",
			MountPath: "/shared",
		}},
	}}

	deploy.Spec.Template.Spec.ServiceAccountName = fmt.Sprintf("%s-%s", cr.Name, common.ArgoCDDefaultDexServiceAccountName)
	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{{
		Name: "static-files",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}}
	dexDisabled := isDexDisabled()
	if dexDisabled {
		log.Info("reconciling for dex, but dex is disabled")
	}

	existing := newDeploymentWithSuffix("dex-server", "dex-server", cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, existing.Name, existing) {
		if dexDisabled {
			log.Info("deleting the existing dex deployment because dex is disabled")
			// Deployment exists but enabled flag has been set to false, delete the Deployment
			return r.client.Delete(context.TODO(), existing)
		}
		changed := false

		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getDexContainerImage(cr)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}

		actualImage = existing.Spec.Template.Spec.InitContainers[0].Image
		desiredImage = getArgoContainerImage(cr)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.InitContainers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}

		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			deploy.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			changed = true
		}

		if !reflect.DeepEqual(existing.Spec.Template.Spec.InitContainers[0].Env,
			deploy.Spec.Template.Spec.InitContainers[0].Env) {
			existing.Spec.Template.Spec.InitContainers[0].Env = deploy.Spec.Template.Spec.InitContainers[0].Env
			changed = true
		}

		if changed {
			return r.client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if dexDisabled {
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), deploy)
}

// reconcileGrafanaDeployment will ensure the Deployment resource is present for the ArgoCD Grafana component.
func (r *ReconcileArgoCD) reconcileGrafanaDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := newDeploymentWithSuffix("grafana", "grafana", cr)
	deploy.Spec.Replicas = getGrafanaReplicas(cr)
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
	if argoutil.IsObjectFound(r.client, cr.Namespace, existing.Name, existing) {
		if !cr.Spec.Grafana.Enabled {
			// Deployment exists but enabled flag has been set to false, delete the Deployment
			return r.client.Delete(context.TODO(), existing)
		}
		changed := false
		if hasGrafanaSpecChanged(existing, cr) {
			existing.Spec.Replicas = cr.Spec.Grafana.Size
			changed = true
		}

		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			deploy.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			changed = true
		}
		if changed {
			return r.client.Update(context.TODO(), existing)
		}
		return nil // Deployment found, do nothing
	}

	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}
	if err := controllerutil.SetControllerReference(cr, deploy, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), deploy)
}

// reconcileRedisDeployment will ensure the Deployment resource is present for the ArgoCD Redis component.
func (r *ReconcileArgoCD) reconcileRedisDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := newDeploymentWithSuffix("redis", "redis", cr)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Args: []string{
			"--save",
			"",
			"--appendonly",
			"no",
		},
		Image:           getRedisContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "redis",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultRedisPort,
			},
		},
		Resources: getRedisResources(cr),
		Env:       proxyEnvVars(),
	}}

	existing := newDeploymentWithSuffix("redis", "redis", cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, existing.Name, existing) {
		if cr.Spec.HA.Enabled {
			// Deployment exists but HA enabled flag has been set to true, delete the Deployment
			return r.client.Delete(context.TODO(), deploy)
		}
		changed := false
		actualImage := deploy.Spec.Template.Spec.Containers[0].Image
		desiredImage := getRedisContainerImage(cr)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}

		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			deploy.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			changed = true
		}

		if changed {
			return r.client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if cr.Spec.HA.Enabled {
		return nil // HA enabled, do nothing.
	}
	if err := controllerutil.SetControllerReference(cr, deploy, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), deploy)
}

// reconcileRedisHAProxyDeployment will ensure the Deployment resource is present for the Redis HA Proxy component.
func (r *ReconcileArgoCD) reconcileRedisHAProxyDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := newDeploymentWithSuffix("redis-ha-haproxy", "redis", cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, deploy.Name, deploy) {
		if !cr.Spec.HA.Enabled {
			// Deployment exists but HA enabled flag has been set to false, delete the Deployment
			return r.client.Delete(context.TODO(), deploy)
		}

		actualImage := deploy.Spec.Template.Spec.Containers[0].Image
		desiredImage := getRedisHAProxyContainerImage(cr)

		if actualImage != desiredImage {
			deploy.Spec.Template.Spec.Containers[0].Image = desiredImage
			deploy.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			return r.client.Update(context.TODO(), deploy)
		}
		return nil // Deployment found, do nothing
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

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
		Env:             proxyEnvVars(),
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
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
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "data",
				MountPath: "/usr/local/etc/haproxy",
			},
			{
				Name:      "shared-socket",
				MountPath: "/run/haproxy",
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
	}

	deploy.Spec.Template.Spec.ServiceAccountName = fmt.Sprintf("%s-%s", cr.Name, "argocd-redis-ha")

	if err := controllerutil.SetControllerReference(cr, deploy, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), deploy)
}

// reconcileRepoDeployment will ensure the Deployment resource is present for the ArgoCD Repo component.
func (r *ReconcileArgoCD) reconcileRepoDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := newDeploymentWithSuffix("repo-server", "repo-server", cr)
	automountToken := false
	if cr.Spec.Repo.MountSAToken {
		automountToken = cr.Spec.Repo.MountSAToken
	}

	deploy.Spec.Template.Spec.AutomountServiceAccountToken = &automountToken

	if cr.Spec.Repo.ServiceAccount != "" {
		deploy.Spec.Template.Spec.ServiceAccountName = cr.Spec.Repo.ServiceAccount
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command:         getArgoRepoCommand(cr),
		Image:           getArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(common.ArgoCDDefaultRepoServerPort),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		Env:  proxyEnvVars(),
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
			Handler: corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(common.ArgoCDDefaultRepoServerPort),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		Resources: getArgoRepoResources(cr),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "ssh-known-hosts",
				MountPath: "/app/config/ssh",
			},
			{
				Name:      "tls-certs",
				MountPath: "/app/config/tls",
			},
			{
				Name:      "gpg-keyring",
				MountPath: "/app/config/gpg/keys",
			},
		},
	}}

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
			Name: "gpg-keyring",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	existing := newDeploymentWithSuffix("repo-server", "repo-server", cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, existing.Name, existing) {
		changed := false
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getArgoContainerImage(cr)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
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
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Env,
			existing.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			changed = true
		}

		if changed {
			return r.client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), deploy)
}

// reconcileServerDeployment will ensure the Deployment resource is present for the ArgoCD Server component.
func (r *ReconcileArgoCD) reconcileServerDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := newDeploymentWithSuffix("server", "server", cr)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command:         getArgoServerCommand(cr),
		Image:           getArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Env:             proxyEnvVars(),
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
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
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8080),
				},
			},
			InitialDelaySeconds: 3,
			PeriodSeconds:       30,
		},
		Resources: getArgoServerResources(cr),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "ssh-known-hosts",
				MountPath: "/app/config/ssh",
			}, {
				Name:      "tls-certs",
				MountPath: "/app/config/tls",
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
		}, {
			Name: "tls-certs",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDTLSCertsConfigMapName,
					},
				},
			},
		},
	}

	existing := newDeploymentWithSuffix("server", "server", cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, existing.Name, existing) {
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getArgoContainerImage(cr)
		changed := false
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}
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
		if changed {
			return r.client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), deploy)
}

// triggerRollout will update the label with the given key to trigger a new rollout of the Deployment.
func (r *ReconcileArgoCD) triggerRollout(deployment *appsv1.Deployment, key string) error {
	if !argoutil.IsObjectFound(r.client, deployment.Namespace, deployment.Name, deployment) {
		log.Info(fmt.Sprintf("unable to locate deployment with name: %s", deployment.Name))
		return nil
	}

	deployment.Spec.Template.ObjectMeta.Labels[key] = nowDefault()
	return r.client.Update(context.TODO(), deployment)
}

func proxyEnvVars(vars ...corev1.EnvVar) []corev1.EnvVar {
	result := []corev1.EnvVar{}
	for _, v := range vars {
		result = append(result, v)
	}
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

func isDexDisabled() bool {
	if v := os.Getenv("DISABLE_DEX"); v != "" {
		return strings.ToLower(v) == "true"
	}
	return false
}
