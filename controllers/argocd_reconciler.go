// Copyright 2021 ArgoCD Operator Developers
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

package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	argopass "github.com/argoproj/argo-cd/util/password"
	"helm.sh/helm/log"

	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	crierrors "k8s.io/cri-api/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ArgoCDReconciler) deleteClusterResources(cr *argoprojv1a1.ArgoCD) error {
	selector, err := argocd.ArgocdInstanceSelector(cr.Name)
	if err != nil {
		return err
	}

	clusterRoleList := &v1.ClusterRoleList{}
	if err := argocd.FilterObjectsBySelector(r.Client, clusterRoleList, selector); err != nil {
		return fmt.Errorf("failed to filter ClusterRoles for %s: %w", cr.Name, err)
	}

	if err := argocd.DeleteClusterRoles(r.Client, clusterRoleList); err != nil {
		return err
	}

	clusterBindingsList := &v1.ClusterRoleBindingList{}
	if err := argocd.FilterObjectsBySelector(r.Client, clusterBindingsList, selector); err != nil {
		return fmt.Errorf("failed to filter ClusterRoleBindings for %s: %w", cr.Name, err)
	}

	if err := argocd.DeleteClusterRoleBindings(r.Client, clusterBindingsList); err != nil {
		return err
	}

	return nil
}

func (r *ArgoCDReconciler) removeDeletionFinalizer(cr *argoprojv1a1.ArgoCD) error {
	cr.Finalizers = argocd.RemoveString(cr.GetFinalizers(), common.ArgoCDDeletionFinalizer)
	if err := r.Client.Update(context.TODO(), cr); err != nil {
		return fmt.Errorf("failed to remove deletion finalizer from %s: %w", cr.Name, err)
	}
	return nil
}

func (r *ArgoCDReconciler) addDeletionFinalizer(argocd *argoprojv1a1.ArgoCD) error {
	argocd.Finalizers = append(argocd.Finalizers, common.ArgoCDDeletionFinalizer)
	if err := r.Client.Update(context.TODO(), argocd); err != nil {
		return fmt.Errorf("failed to add deletion finalizer for %s: %w", argocd.Name, err)
	}
	return nil
}

// reconcileResources will reconcile common ArgoCD resources.
func (r *ArgoCDReconciler) reconcileResources(cr *argoprojv1a1.ArgoCD) error {
	logr.Info("reconciling status")
	if err := r.reconcileStatus(cr); err != nil {
		return err
	}

	logr.Info("reconciling roles")
	if _, err := r.reconcileRoles(cr); err != nil {
		return err
	}

	logr.Info("reconciling rolebindings")
	if err := r.reconcileRoleBindings(cr); err != nil {
		return err
	}

	logr.Info("reconciling service accounts")
	if err := r.reconcileServiceAccounts(cr); err != nil {
		return err
	}

	logr.Info("reconciling certificate authority")
	if err := r.reconcileCertificateAuthority(cr); err != nil {
		return err
	}

	logr.Info("reconciling secrets")
	if err := r.reconcileSecrets(cr); err != nil {
		return err
	}

	logr.Info("reconciling config maps")
	if err := r.reconcileConfigMaps(cr); err != nil {
		return err
	}

	logr.Info("reconciling services")
	if err := r.reconcileServices(cr); err != nil {
		return err
	}

	logr.Info("reconciling deployments")
	if err := r.reconcileDeployments(cr); err != nil {
		return err
	}

	logr.Info("reconciling statefulsets")
	if err := r.reconcileStatefulSets(cr); err != nil {
		return err
	}

	logr.Info("reconciling autoscalers")
	if err := r.reconcileAutoscalers(cr); err != nil {
		return err
	}

	logr.Info("reconciling ingresses")
	if err := r.reconcileIngresses(cr); err != nil {
		return err
	}

	if argocd.IsRouteAPIAvailable() {
		logr.Info("reconciling routes")
		if err := r.reconcileRoutes(cr); err != nil {
			return err
		}
	}

	if argocd.IsPrometheusAPIAvailable() {
		logr.Info("reconciling prometheus")
		if err := r.reconcilePrometheus(cr); err != nil {
			return err
		}

		if err := r.reconcileMetricsServiceMonitor(cr); err != nil {
			return err
		}

		if err := r.reconcileRepoServerServiceMonitor(cr); err != nil {
			return err
		}

		if err := r.reconcileServerMetricsServiceMonitor(cr); err != nil {
			return err
		}
	}

	if cr.Spec.ApplicationSet != nil {
		logr.Info("reconciling ApplicationSet controller")
		if err := r.reconcileApplicationSetController(cr); err != nil {
			return err
		}
	}

	if err := r.reconcileRepoServerTLSSecret(cr); err != nil {
		return err
	}

	if cr.Spec.SSO != nil {
		logr.Info("reconciling SSO")
		if err := r.reconcileSSO(cr); err != nil {
			return err
		}
	}

	return nil
}

// reconcileDeployments will ensure that all Deployment resources are present for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileDeployments(cr *argoprojv1a1.ArgoCD) error {
	err := r.reconcileDexDeployment(cr)
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

// reconcileGrafanaDeployment will ensure the Deployment resource is present for the ArgoCD Grafana component.
func (r *ArgoCDReconciler) reconcileGrafanaDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := argocd.NewDeploymentWithSuffix("grafana", "grafana", cr)
	deploy.Spec.Replicas = argocd.GetGrafanaReplicas(cr)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Image:           argocd.GetGrafanaContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "grafana",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 3000,
			},
		},
		Env:       argocd.ProxyEnvVars(),
		Resources: argocd.GetGrafanaResources(cr),
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
						Name: argocd.NameWithSuffix("grafana-config", cr),
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
						Name: argocd.NameWithSuffix("grafana-config", cr),
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
						Name: argocd.NameWithSuffix("grafana-config", cr),
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
						Name: argocd.NameWithSuffix("grafana-dashboards", cr),
					},
				},
			},
		},
	}

	existing := argocd.NewDeploymentWithSuffix("grafana", "grafana", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		if !cr.Spec.Grafana.Enabled {
			// Deployment exists but enabled flag has been set to false, delete the Deployment
			return r.Client.Delete(context.TODO(), existing)
		}
		changed := false
		if argocd.HasGrafanaSpecChanged(existing, cr) {
			existing.Spec.Replicas = cr.Spec.Grafana.Size
			changed = true
		}

		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			deploy.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
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

// reconcileServerDeployment will ensure the Deployment resource is present for the ArgoCD Server component.
func (r *ArgoCDReconciler) reconcileServerDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := argocd.NewDeploymentWithSuffix("server", "server", cr)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command:         argocd.GetArgoServerCommand(cr),
		Image:           argocd.GetArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Env:             argocd.ProxyEnvVars(),
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
		Resources: argocd.GetArgoServerResources(cr),
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
		}, {
			Name: "argocd-repo-server-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRepoServerTLSSecretName,
					Optional:   argocd.BoolPtr(true),
				},
			},
		},
	}

	existing := argocd.NewDeploymentWithSuffix("server", "server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := argocd.GetArgoContainerImage(cr)
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
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Volumes, existing.Spec.Template.Spec.Volumes) {
			existing.Spec.Template.Spec.Volumes = deploy.Spec.Template.Spec.Volumes
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].VolumeMounts,
			existing.Spec.Template.Spec.Containers[0].VolumeMounts) {
			existing.Spec.Template.Spec.Containers[0].VolumeMounts = deploy.Spec.Template.Spec.Containers[0].VolumeMounts
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

// reconcileRepoDeployment will ensure the Deployment resource is present for the ArgoCD Repo component.
func (r *ArgoCDReconciler) reconcileRepoDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := argocd.NewDeploymentWithSuffix("repo-server", "repo-server", cr)
	automountToken := false
	if cr.Spec.Repo.MountSAToken {
		automountToken = cr.Spec.Repo.MountSAToken
	}

	deploy.Spec.Template.Spec.AutomountServiceAccountToken = &automountToken

	if cr.Spec.Repo.ServiceAccount != "" {
		deploy.Spec.Template.Spec.ServiceAccountName = cr.Spec.Repo.ServiceAccount
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command:         argocd.GetArgoRepoCommand(cr),
		Image:           argocd.GetArgoContainerImage(cr),
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
		Env:  argocd.ProxyEnvVars(),
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
		Resources: argocd.GetArgoRepoResources(cr),
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
					Optional:   argocd.BoolPtr(true),
				},
			},
		},
	}

	existing := argocd.NewDeploymentWithSuffix("repo-server", "repo-server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		changed := false
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := argocd.GetArgoContainerImage(cr)
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
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileRedisHAProxyDeployment will ensure the Deployment resource is present for the Redis HA Proxy component.
func (r *ArgoCDReconciler) reconcileRedisHAProxyDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := argocd.NewDeploymentWithSuffix("redis-ha-haproxy", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		if !cr.Spec.HA.Enabled {
			// Deployment exists but HA enabled flag has been set to false, delete the Deployment
			return r.Client.Delete(context.TODO(), deploy)
		}

		actualImage := deploy.Spec.Template.Spec.Containers[0].Image
		desiredImage := argocd.GetRedisHAProxyContainerImage(cr)

		if actualImage != desiredImage {
			deploy.Spec.Template.Spec.Containers[0].Image = desiredImage
			deploy.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			return r.Client.Update(context.TODO(), deploy)
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
								common.ArgoCDKeyName: argocd.NameWithSuffix("redis-ha-haproxy", cr),
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
							common.ArgoCDKeyName: argocd.NameWithSuffix("redis-ha-haproxy", cr),
						},
					},
					TopologyKey: common.ArgoCDKeyHostname,
				},
			},
		},
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Image:           argocd.GetRedisHAProxyContainerImage(cr),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Name:            "haproxy",
		Env:             argocd.ProxyEnvVars(),
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
		Resources: argocd.GetRedisHAProxyResources(cr),
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
		Image:           argocd.GetRedisHAProxyContainerImage(cr),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Name:            "config-init",
		Env:             argocd.ProxyEnvVars(),
		Resources:       argocd.GetRedisHAProxyResources(cr),
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

	if err := argocd.ApplyReconcilerHook(cr, deploy, ""); err != nil {
		return err
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileRedisDeployment will ensure the Deployment resource is present for the ArgoCD Redis component.
func (r *ArgoCDReconciler) reconcileRedisDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := argocd.NewDeploymentWithSuffix("redis", "redis", cr)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Args: []string{
			"--save",
			"",
			"--appendonly",
			"no",
		},
		Image:           argocd.GetRedisContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "redis",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultRedisPort,
			},
		},
		Resources: argocd.GetRedisResources(cr),
		Env:       argocd.ProxyEnvVars(),
	}}

	if err := argocd.ApplyReconcilerHook(cr, deploy, ""); err != nil {
		return err
	}

	existing := argocd.NewDeploymentWithSuffix("redis", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		if cr.Spec.HA.Enabled {
			// Deployment exists but HA enabled flag has been set to true, delete the Deployment
			return r.Client.Delete(context.TODO(), deploy)
		}
		changed := false
		actualImage := deploy.Spec.Template.Spec.Containers[0].Image
		desiredImage := argocd.GetRedisContainerImage(cr)
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

// reconcileDexDeployment will ensure the Deployment resource is present for the ArgoCD Dex component.
func (r *ArgoCDReconciler) reconcileDexDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := argocd.NewDeploymentWithSuffix("dex-server", "dex-server", cr)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command: []string{
			"/shared/argocd-dex",
			"rundex",
		},
		Image:           argocd.GetDexContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "dex",
		Env:             argocd.ProxyEnvVars(),
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultDexHTTPPort,
				Name:          "http",
			}, {
				ContainerPort: common.ArgoCDDefaultDexGRPCPort,
				Name:          "grpc",
			},
		},
		Resources: argocd.GetDexResources(cr),
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "static-files",
			MountPath: "/shared",
		}},
	}}

	deploy.Spec.Template.Spec.InitContainers = []corev1.Container{{
		Command: []string{
			"cp",
			"-n",
			"/usr/local/bin/argocd",
			"/shared/argocd-dex",
		},
		Env:             argocd.ProxyEnvVars(),
		Image:           argocd.GetArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "copyutil",
		Resources:       argocd.GetDexResources(cr),
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
	dexDisabled := argocd.IsDexDisabled()
	if dexDisabled {
		log.Info("reconciling for dex, but dex is disabled")
	}

	existing := argocd.NewDeploymentWithSuffix("dex-server", "dex-server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		if dexDisabled {
			log.Info("deleting the existing dex deployment because dex is disabled")
			// Deployment exists but enabled flag has been set to false, delete the Deployment
			return r.Client.Delete(context.TODO(), existing)
		}
		changed := false

		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := argocd.GetDexContainerImage(cr)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}

		actualImage = existing.Spec.Template.Spec.InitContainers[0].Image
		desiredImage = argocd.GetArgoContainerImage(cr)
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
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if dexDisabled {
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileServices will ensure that all Services are present for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileServices(cr *argoprojv1a1.ArgoCD) error {
	err := r.reconcileDexService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileGrafanaService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileMetricsService(cr)
	if err != nil {
		return err
	}

	if cr.Spec.HA.Enabled {
		err = r.reconcileRedisHAServices(cr)
		if err != nil {
			return err
		}
	} else {
		err = r.reconcileRedisService(cr)
		if err != nil {
			return err
		}
	}

	err = r.reconcileRepoService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileServerMetricsService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileServerService(cr)
	if err != nil {
		return err
	}
	return nil
}

// reconcileServerService will ensure that the Service is present for the Argo CD server component.
func (r *ArgoCDReconciler) reconcileServerService(cr *argoprojv1a1.ArgoCD) error {
	svc := argocd.NewServiceWithSuffix("server", "server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http",
			Port:       80,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8080),
		}, {
			Name:       "https",
			Port:       443,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8080),
		},
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: argocd.NameWithSuffix("server", cr),
	}

	svc.Spec.Type = argocd.GetArgoServerServiceType(cr)

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileServerMetricsService will ensure that the Service for the Argo CD server metrics is present.
func (r *ArgoCDReconciler) reconcileServerMetricsService(cr *argoprojv1a1.ArgoCD) error {
	svc := argocd.NewServiceWithSuffix("server-metrics", "server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: argocd.NameWithSuffix("server", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       8083,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8083),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileServerMetricsServiceMonitor will ensure that the ServiceMonitor is present for the ArgoCD Server metrics Service.
func (r *ArgoCDReconciler) reconcileServerMetricsServiceMonitor(cr *argoprojv1a1.ArgoCD) error {
	sm := argocd.NewServiceMonitorWithSuffix("server-metrics", cr)
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
			common.ArgoCDKeyName: argocd.NameWithSuffix("server-metrics", cr),
		},
	}
	sm.Spec.Endpoints = []monitoringv1.Endpoint{
		{
			Port: common.ArgoCDKeyMetrics,
		},
	}

	if err := controllerutil.SetControllerReference(cr, sm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), sm)
}

// reconcileRepoService will ensure that the Service for the Argo CD repo server is present.
func (r *ArgoCDReconciler) reconcileRepoService(cr *argoprojv1a1.ArgoCD) error {
	svc := argocd.NewServiceWithSuffix("repo-server", "repo-server", cr)

	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		if argocd.EnsureAutoTLSAnnotation(cr, svc) {
			return r.Client.Update(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	argocd.EnsureAutoTLSAnnotation(cr, svc)

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: argocd.NameWithSuffix("repo-server", cr),
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

// reconcileRedisService will ensure that the Service for Redis is present.
func (r *ArgoCDReconciler) reconcileRedisService(cr *argoprojv1a1.ArgoCD) error {
	svc := argocd.NewServiceWithSuffix("redis", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: argocd.NameWithSuffix("redis", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "tcp-redis",
			Port:       common.ArgoCDDefaultRedisPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ArgoCDDefaultRedisPort),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileRedisHAServices will ensure that all required Services are present for Redis when running in HA mode.
func (r *ArgoCDReconciler) reconcileRedisHAServices(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileRedisHAAnnounceServices(cr); err != nil {
		return err
	}

	if err := r.reconcileRedisHAMasterService(cr); err != nil {
		return err
	}

	if err := r.reconcileRedisHAProxyService(cr); err != nil {
		return err
	}
	return nil
}

func (r *ArgoCDReconciler) reconcileRedisHAProxyService(cr *argoprojv1a1.ArgoCD) error {
	svc := argocd.NewServiceWithSuffix("redis-ha-haproxy", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: argocd.NameWithSuffix("redis-ha-haproxy", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "haproxy",
			Port:       common.ArgoCDDefaultRedisPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString("redis"),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileRedisHAMasterService will ensure that the "master" Service is present for Redis when running in HA mode.
func (r *ArgoCDReconciler) reconcileRedisHAMasterService(cr *argoprojv1a1.ArgoCD) error {
	svc := argocd.NewServiceWithSuffix("redis-ha", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: argocd.NameWithSuffix("redis-ha", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "server",
			Port:       common.ArgoCDDefaultRedisPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString("redis"),
		}, {
			Name:       "sentinel",
			Port:       common.ArgoCDDefaultRedisSentinelPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString("sentinel"),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileRedisHAAnnounceServices will ensure that the announce Services are present for Redis when running in HA mode.
func (r *ArgoCDReconciler) reconcileRedisHAAnnounceServices(cr *argoprojv1a1.ArgoCD) error {
	for i := int32(0); i < common.ArgoCDDefaultRedisHAReplicas; i++ {
		svc := argocd.NewServiceWithSuffix(fmt.Sprintf("redis-ha-announce-%d", i), "redis", cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
			return nil // Service found, do nothing
		}

		svc.ObjectMeta.Annotations = map[string]string{
			common.ArgoCDKeyTolerateUnreadyEndpounts: "true",
		}

		svc.Spec.PublishNotReadyAddresses = true

		svc.Spec.Selector = map[string]string{
			common.ArgoCDKeyName:               argocd.NameWithSuffix("redis-ha", cr),
			common.ArgoCDKeyStatefulSetPodName: argocd.NameWithSuffix(fmt.Sprintf("redis-ha-server-%d", i), cr),
		}

		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "server",
				Port:       common.ArgoCDDefaultRedisPort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromString("redis"),
			}, {
				Name:       "sentinel",
				Port:       common.ArgoCDDefaultRedisSentinelPort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromString("sentinel"),
			},
		}

		if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
			return err
		}

		if err := r.Client.Create(context.TODO(), svc); err != nil {
			return err
		}
	}
	return nil
}

// reconcileMetricsService will ensure that the Service for the Argo CD application controller metrics is present.
func (r *ArgoCDReconciler) reconcileMetricsService(cr *argoprojv1a1.ArgoCD) error {
	svc := argocd.NewServiceWithSuffix("metrics", "metrics", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		// Service found, do nothing
		return nil
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: argocd.NameWithSuffix("application-controller", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       8082,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8082),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileGrafanaService will ensure that the Service for Grafana is present.
func (r *ArgoCDReconciler) reconcileGrafanaService(cr *argoprojv1a1.ArgoCD) error {
	svc := argocd.NewServiceWithSuffix("grafana", "grafana", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		if !cr.Spec.Grafana.Enabled {
			// Service exists but enabled flag has been set to false, delete the Service
			return r.Client.Delete(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: argocd.NameWithSuffix("grafana", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http",
			Port:       80,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(3000),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileDexService will ensure that the Service for Dex is present.
func (r *ArgoCDReconciler) reconcileDexService(cr *argoprojv1a1.ArgoCD) error {
	svc := argocd.NewServiceWithSuffix("dex-server", "dex-server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		if argocd.IsDexDisabled() {
			// Service exists but enabled flag has been set to false, delete the Service
			return r.Client.Delete(context.TODO(), svc)
		}
		return nil
	}

	if argocd.IsDexDisabled() {
		return nil // Dex is disabled, do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: argocd.NameWithSuffix("dex-server", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http",
			Port:       common.ArgoCDDefaultDexHTTPPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ArgoCDDefaultDexHTTPPort),
		}, {
			Name:       "grpc",
			Port:       common.ArgoCDDefaultDexGRPCPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ArgoCDDefaultDexGRPCPort),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileConfigMaps will ensure that all ArgoCD ConfigMaps are present.
func (r *ArgoCDReconciler) reconcileConfigMaps(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileArgoConfigMap(cr); err != nil {
		return err
	}

	if err := r.reconcileRedisConfiguration(cr); err != nil {
		return err
	}

	if err := r.reconcileRBAC(cr); err != nil {
		return err
	}

	if err := r.reconcileSSHKnownHosts(cr); err != nil {
		return err
	}

	if err := r.reconcileTLSCerts(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaConfiguration(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaDashboards(cr); err != nil {
		return err
	}

	return r.reconcileGPGKeysConfigMap(cr)
}

// reconcileGPGKeysConfigMap creates a gpg-keys config map
func (r *ArgoCDReconciler) reconcileGPGKeysConfigMap(cr *argoprojv1a1.ArgoCD) error {
	cm := argocd.NewConfigMapWithName(common.ArgoCDGPGKeysConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil
	}
	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileGrafanaDashboards will ensure that the Grafana dashboards ConfigMap is present.
func (r *ArgoCDReconciler) reconcileGrafanaDashboards(cr *argoprojv1a1.ArgoCD) error {
	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	cm := argocd.NewConfigMapWithSuffix(common.ArgoCDGrafanaDashboardConfigMapSuffix, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	pattern := filepath.Join(argocd.GetGrafanaConfigPath(), "dashboards/*.json")
	dashboards, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	data := make(map[string]string)
	for _, f := range dashboards {
		dashboard, err := ioutil.ReadFile(f)
		if err != nil {
			return err
		}

		parts := strings.Split(f, "/")
		filename := parts[len(parts)-1]
		data[filename] = string(dashboard)
	}
	cm.Data = data

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileGrafanaConfiguration will ensure that the Grafana configuration ConfigMap is present.
func (r *ArgoCDReconciler) reconcileGrafanaConfiguration(cr *argoprojv1a1.ArgoCD) error {
	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	cm := argocd.NewConfigMapWithSuffix(common.ArgoCDGrafanaConfigMapSuffix, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	secret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "grafana")
	secret, err := argoutil.FetchSecret(r.Client, cr.ObjectMeta, secret.Name)
	if err != nil {
		return err
	}

	grafanaConfig := argocd.GrafanaConfig{
		Security: argocd.GrafanaSecurityConfig{
			AdminUser:     string(secret.Data[common.ArgoCDKeyGrafanaAdminUsername]),
			AdminPassword: string(secret.Data[common.ArgoCDKeyGrafanaAdminPassword]),
			SecretKey:     string(secret.Data[common.ArgoCDKeyGrafanaSecretKey]),
		},
	}

	data, err := argocd.LoadGrafanaConfigs()
	if err != nil {
		return err
	}

	tmpls, err := argocd.LoadGrafanaTemplates(&grafanaConfig)
	if err != nil {
		return err
	}

	for key, val := range tmpls {
		data[key] = val
	}
	cm.Data = data

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileTLSCerts will ensure that the ArgoCD TLS Certs ConfigMap is present.
func (r *ArgoCDReconciler) reconcileTLSCerts(cr *argoprojv1a1.ArgoCD) error {
	cm := argocd.NewConfigMapWithName(common.ArgoCDTLSCertsConfigMapName, cr)
	cm.Data = argocd.GetInitialTLSCerts(cr)
	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}

	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return r.Client.Update(context.TODO(), cm)
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileSSHKnownHosts will ensure that the ArgoCD SSH Known Hosts ConfigMap is present.
func (r *ArgoCDReconciler) reconcileSSHKnownHosts(cr *argoprojv1a1.ArgoCD) error {
	cm := argocd.NewConfigMapWithName(common.ArgoCDKnownHostsConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, move along...
	}

	cm.Data = map[string]string{
		common.ArgoCDKeySSHKnownHosts: argocd.GetInitialSSHKnownHosts(cr),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileRBAC will ensure that the ArgoCD RBAC ConfigMap is present.
func (r *ArgoCDReconciler) reconcileRBAC(cr *argoprojv1a1.ArgoCD) error {
	cm := argocd.NewConfigMapWithName(common.ArgoCDRBACConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return r.reconcileRBACConfigMap(cm, cr)
	}
	return r.createRBACConfigMap(cm, cr)
}

// createRBACConfigMap will create the Argo CD RBAC ConfigMap resource.
func (r *ArgoCDReconciler) createRBACConfigMap(cm *corev1.ConfigMap, cr *argoprojv1a1.ArgoCD) error {
	data := make(map[string]string)
	data[common.ArgoCDKeyRBACPolicyCSV] = argocd.GetRBACPolicy(cr)
	data[common.ArgoCDKeyRBACPolicyDefault] = argocd.GetRBACDefaultPolicy(cr)
	data[common.ArgoCDKeyRBACScopes] = argocd.GetRBACScopes(cr)
	cm.Data = data

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileRBACConfigMap will ensure that the RBAC ConfigMap is syncronized with the given ArgoCD.
func (r *ArgoCDReconciler) reconcileRBACConfigMap(cm *corev1.ConfigMap, cr *argoprojv1a1.ArgoCD) error {
	changed := false
	// Policy CSV
	if cr.Spec.RBAC.Policy != nil && cm.Data[common.ArgoCDKeyRBACPolicyCSV] != *cr.Spec.RBAC.Policy {
		cm.Data[common.ArgoCDKeyRBACPolicyCSV] = *cr.Spec.RBAC.Policy
		changed = true
	}

	// Default Policy
	if cr.Spec.RBAC.DefaultPolicy != nil && cm.Data[common.ArgoCDKeyRBACPolicyDefault] != *cr.Spec.RBAC.DefaultPolicy {
		cm.Data[common.ArgoCDKeyRBACPolicyDefault] = *cr.Spec.RBAC.DefaultPolicy
		changed = true
	}

	// Scopes
	if cr.Spec.RBAC.Scopes != nil && cm.Data[common.ArgoCDKeyRBACScopes] != *cr.Spec.RBAC.Scopes {
		cm.Data[common.ArgoCDKeyRBACScopes] = *cr.Spec.RBAC.Scopes
		changed = true
	}

	if changed {
		// TODO: Reload server (and dex?) if RBAC settings change?
		return r.Client.Update(context.TODO(), cm)
	}
	return nil // ConfigMap exists and nothing to do, move along...
}

// reconcileRedisConfiguration will ensure that all of the Redis ConfigMaps are present for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileRedisConfiguration(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileRedisHAConfigMap(cr); err != nil {
		return err
	}
	return nil
}

// reconcileRedisHAConfigMap will ensure that the Redis HA ConfigMap is present for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileRedisHAConfigMap(cr *argoprojv1a1.ArgoCD) error {
	cm := argocd.NewConfigMapWithName(common.ArgoCDRedisHAConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if !cr.Spec.HA.Enabled {
			// ConfigMap exists but HA enabled flag has been set to false, delete the ConfigMap
			return r.Client.Delete(context.TODO(), cm)
		}
		return nil // ConfigMap found with nothing changed, move along...
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

	cm.Data = map[string]string{
		"haproxy.cfg":     argocd.GetRedisHAProxyConfig(cr),
		"haproxy_init.sh": argocd.GetRedisHAProxyScript(cr),
		"init.sh":         argocd.GetRedisInitScript(cr),
		"redis.conf":      argocd.GetRedisConf(cr),
		"sentinel.conf":   argocd.GetRedisSentinelConf(cr),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileConfiguration will ensure that the main ConfigMap for ArgoCD is present.
func (r *ArgoCDReconciler) reconcileArgoConfigMap(cr *argoprojv1a1.ArgoCD) error {
	cm := argocd.NewConfigMapWithName(common.ArgoCDConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if err := r.reconcileDexConfiguration(cm, cr); err != nil {
			return err
		}
		return r.reconcileExistingArgoConfigMap(cm, cr)
	}

	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}

	cm.Data[common.ArgoCDKeyApplicationInstanceLabelKey] = argocd.GetApplicationInstanceLabelKey(cr)
	cm.Data[common.ArgoCDKeyConfigManagementPlugins] = argocd.GetConfigManagementPlugins(cr)
	cm.Data[common.ArgoCDKeyAdminEnabled] = fmt.Sprintf("%t", !cr.Spec.DisableAdmin)
	cm.Data[common.ArgoCDKeyGATrackingID] = argocd.GetGATrackingID(cr)
	cm.Data[common.ArgoCDKeyGAAnonymizeUsers] = fmt.Sprint(cr.Spec.GAAnonymizeUsers)
	cm.Data[common.ArgoCDKeyHelpChatURL] = argocd.GetHelpChatURL(cr)
	cm.Data[common.ArgoCDKeyHelpChatText] = argocd.GetHelpChatText(cr)
	cm.Data[common.ArgoCDKeyKustomizeBuildOptions] = argocd.GetKustomizeBuildOptions(cr)
	cm.Data[common.ArgoCDKeyOIDCConfig] = argocd.GetOIDCConfig(cr)
	if c := argocd.GetResourceCustomizations(cr); c != "" {
		cm.Data[common.ArgoCDKeyResourceCustomizations] = c
	}
	cm.Data[common.ArgoCDKeyResourceExclusions] = argocd.GetResourceExclusions(cr)
	cm.Data[common.ArgoCDKeyResourceInclusions] = argocd.GetResourceInclusions(cr)
	cm.Data[common.ArgoCDKeyRepositories] = argocd.GetInitialRepositories(cr)
	cm.Data[common.ArgoCDKeyRepositoryCredentials] = argocd.GetRepositoryCredentials(cr)
	cm.Data[common.ArgoCDKeyStatusBadgeEnabled] = fmt.Sprint(cr.Spec.StatusBadgeEnabled)
	cm.Data[common.ArgoCDKeyServerURL] = r.getArgoServerURI(cr)
	cm.Data[common.ArgoCDKeyUsersAnonymousEnabled] = fmt.Sprint(cr.Spec.UsersAnonymousEnabled)

	if !argocd.IsDexDisabled() {
		dexConfig := argocd.GetDexConfig(cr)
		if dexConfig == "" && cr.Spec.Dex.OpenShiftOAuth {
			cfg, err := r.getOpenShiftDexConfig(cr)
			if err != nil {
				return err
			}
			dexConfig = cfg
		}
		cm.Data[common.ArgoCDKeyDexConfig] = dexConfig
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

func (r *ArgoCDReconciler) reconcileExistingArgoConfigMap(cm *corev1.ConfigMap, cr *argoprojv1a1.ArgoCD) error {
	changed := false

	if cm.Data[common.ArgoCDKeyAdminEnabled] == fmt.Sprintf("%t", cr.Spec.DisableAdmin) {
		cm.Data[common.ArgoCDKeyAdminEnabled] = fmt.Sprintf("%t", !cr.Spec.DisableAdmin)
		changed = true
	}

	if cm.Data[common.ArgoCDKeyApplicationInstanceLabelKey] != cr.Spec.ApplicationInstanceLabelKey {
		cm.Data[common.ArgoCDKeyApplicationInstanceLabelKey] = cr.Spec.ApplicationInstanceLabelKey
		changed = true
	}

	if cm.Data[common.ArgoCDKeyConfigManagementPlugins] != cr.Spec.ConfigManagementPlugins {
		cm.Data[common.ArgoCDKeyConfigManagementPlugins] = cr.Spec.ConfigManagementPlugins
		changed = true
	}

	if cm.Data[common.ArgoCDKeyGATrackingID] != cr.Spec.GATrackingID {
		cm.Data[common.ArgoCDKeyGATrackingID] = cr.Spec.GATrackingID
		changed = true
	}

	if cm.Data[common.ArgoCDKeyGAAnonymizeUsers] != fmt.Sprint(cr.Spec.GAAnonymizeUsers) {
		cm.Data[common.ArgoCDKeyGAAnonymizeUsers] = fmt.Sprint(cr.Spec.GAAnonymizeUsers)
		changed = true
	}

	if cm.Data[common.ArgoCDKeyHelpChatURL] != cr.Spec.HelpChatURL {
		cm.Data[common.ArgoCDKeyHelpChatURL] = cr.Spec.HelpChatURL
		changed = true
	}

	if cm.Data[common.ArgoCDKeyHelpChatText] != cr.Spec.HelpChatText {
		cm.Data[common.ArgoCDKeyHelpChatText] = cr.Spec.HelpChatText
		changed = true
	}

	if cm.Data[common.ArgoCDKeyKustomizeBuildOptions] != cr.Spec.KustomizeBuildOptions {
		cm.Data[common.ArgoCDKeyKustomizeBuildOptions] = cr.Spec.KustomizeBuildOptions
		changed = true
	}

	if cr.Spec.SSO == nil {
		if cm.Data[common.ArgoCDKeyOIDCConfig] != cr.Spec.OIDCConfig {
			cm.Data[common.ArgoCDKeyOIDCConfig] = cr.Spec.OIDCConfig
			changed = true
		}
	}

	if cm.Data[common.ArgoCDKeyResourceCustomizations] != cr.Spec.ResourceCustomizations {
		cm.Data[common.ArgoCDKeyResourceCustomizations] = cr.Spec.ResourceCustomizations
		changed = true
	}

	if cm.Data[common.ArgoCDKeyResourceExclusions] != cr.Spec.ResourceExclusions {
		cm.Data[common.ArgoCDKeyResourceExclusions] = cr.Spec.ResourceExclusions
		changed = true
	}

	uri := r.getArgoServerURI(cr)
	if cm.Data[common.ArgoCDKeyServerURL] != uri {
		cm.Data[common.ArgoCDKeyServerURL] = uri
		changed = true
	}

	if cm.Data[common.ArgoCDKeyStatusBadgeEnabled] != fmt.Sprint(cr.Spec.StatusBadgeEnabled) {
		cm.Data[common.ArgoCDKeyStatusBadgeEnabled] = fmt.Sprint(cr.Spec.StatusBadgeEnabled)
		changed = true
	}

	if cm.Data[common.ArgoCDKeyUsersAnonymousEnabled] != fmt.Sprint(cr.Spec.UsersAnonymousEnabled) {
		cm.Data[common.ArgoCDKeyUsersAnonymousEnabled] = fmt.Sprint(cr.Spec.UsersAnonymousEnabled)
		changed = true
	}

	if cm.Data[common.ArgoCDKeyRepositoryCredentials] != cr.Spec.RepositoryCredentials {
		cm.Data[common.ArgoCDKeyRepositoryCredentials] = cr.Spec.RepositoryCredentials
		changed = true
	}

	if changed {
		return r.Client.Update(context.TODO(), cm) // TODO: Reload Argo CD server after ConfigMap change (which properties)?
	}

	return nil // Nothing changed, no update needed...
}

// reconcileDexConfiguration will ensure that Dex is configured properly.
func (r *ArgoCDReconciler) reconcileDexConfiguration(cm *corev1.ConfigMap, cr *argoprojv1a1.ArgoCD) error {
	actual := cm.Data[common.ArgoCDKeyDexConfig]
	desired := argocd.GetDexConfig(cr)
	if len(desired) <= 0 && cr.Spec.Dex.OpenShiftOAuth {
		cfg, err := r.getOpenShiftDexConfig(cr)
		if err != nil {
			return err
		}
		desired = cfg
	}

	if actual != desired {
		// Update ConfigMap with desired configuration.
		cm.Data[common.ArgoCDKeyDexConfig] = desired
		if err := r.Client.Update(context.TODO(), cm); err != nil {
			return err
		}

		// Trigger rollout of Dex Deployment to pick up changes.
		deploy := argocd.NewDeploymentWithSuffix("dex-server", "dex-server", cr)
		if !argoutil.IsObjectFound(r.Client, deploy.Namespace, deploy.Name, deploy) {
			logr.Info("unable to locate dex deployment")
			return nil
		}

		deploy.Spec.Template.ObjectMeta.Labels["dex.config.changed"] = time.Now().UTC().Format("01022006-150406-MST")
		return r.Client.Update(context.TODO(), deploy)
	}
	return nil
}

// getOpenShiftDexConfig will return the configuration for the Dex server running on OpenShift.
func (r *ArgoCDReconciler) getOpenShiftDexConfig(cr *argoprojv1a1.ArgoCD) (string, error) {
	clientSecret, err := r.getDexOAuthClientSecret(cr)
	if err != nil {
		return "", err
	}

	connector := argocd.DexConnector{
		Type: "openshift",
		ID:   "openshift",
		Name: "OpenShift",
		Config: map[string]interface{}{
			"issuer":       "https://kubernetes.default.svc", // TODO: Should this be hard-coded?
			"clientID":     argocd.GetDexOAuthClientID(cr),
			"clientSecret": *clientSecret,
			"redirectURI":  r.getDexOAuthRedirectURI(cr),
			"insecureCA":   true, // TODO: Configure for openshift CA
		},
	}

	connectors := make([]argocd.DexConnector, 0)
	connectors = append(connectors, connector)

	dex := make(map[string]interface{})
	dex["connectors"] = connectors

	bytes, err := yaml.Marshal(dex)
	return string(bytes), err
}

// getDexOAuthClientSecret will return the OAuth client secret for the given ArgoCD.
func (r *ArgoCDReconciler) getDexOAuthClientSecret(cr *argoprojv1a1.ArgoCD) (*string, error) {
	sa := argocd.NewServiceAccountWithName(common.ArgoCDDefaultDexServiceAccountName, cr)
	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		return nil, err
	}

	// Find the token secret
	var tokenSecret *corev1.ObjectReference
	for _, saSecret := range sa.Secrets {
		if strings.Contains(saSecret.Name, "token") {
			tokenSecret = &saSecret
			break
		}
	}

	if tokenSecret == nil {
		return nil, errors.New("unable to locate ServiceAccount token for OAuth client secret")
	}

	// Fetch the secret to obtain the token
	secret := argoutil.NewSecretWithName(cr.ObjectMeta, tokenSecret.Name)
	if err := argoutil.FetchObject(r.Client, cr.Namespace, secret.Name, secret); err != nil {
		return nil, err
	}

	token := string(secret.Data["token"])
	return &token, nil
}

func (r *ArgoCDReconciler) reconcileSecrets(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileClusterSecrets(cr); err != nil {
		return err
	}

	if err := r.reconcileArgoSecret(cr); err != nil {
		return err
	}

	return nil
}

// reconcileArgoSecret will ensure that the Argo CD Secret is present.
func (r *ArgoCDReconciler) reconcileArgoSecret(cr *argoprojv1a1.ArgoCD) error {
	clusterSecret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "cluster")
	secret := argoutil.NewSecretWithName(cr.ObjectMeta, common.ArgoCDSecretName)

	if !argoutil.IsObjectFound(r.Client, cr.Namespace, clusterSecret.Name, clusterSecret) {
		logr.Info(fmt.Sprintf("cluster secret [%s] not found, waiting to reconcile argo secret [%s]", clusterSecret.Name, secret.Name))
		return nil
	}

	tlsSecret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "tls")
	if !argoutil.IsObjectFound(r.Client, cr.Namespace, tlsSecret.Name, tlsSecret) {
		logr.Info(fmt.Sprintf("tls secret [%s] not found, waiting to reconcile argo secret [%s]", tlsSecret.Name, secret.Name))
		return nil
	}

	if argoutil.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return r.reconcileExistingArgoSecret(cr, secret, clusterSecret, tlsSecret)
	}

	// Secret not found, create it...
	hashedPassword, err := argopass.HashPassword(string(clusterSecret.Data[common.ArgoCDKeyAdminPassword]))
	if err != nil {
		return err
	}

	sessionKey, err := argocd.GenerateArgoServerSessionKey()
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		common.ArgoCDKeyAdminPassword:      []byte(hashedPassword),
		common.ArgoCDKeyAdminPasswordMTime: argocd.NowBytes(),
		common.ArgoCDKeyServerSecretKey:    sessionKey,
		common.ArgoCDKeyTLSCert:            tlsSecret.Data[common.ArgoCDKeyTLSCert],
		common.ArgoCDKeyTLSPrivateKey:      tlsSecret.Data[common.ArgoCDKeyTLSPrivateKey],
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), secret)
}

// reconcileExistingArgoSecret will ensure that the Argo CD Secret is up to date.
func (r *ArgoCDReconciler) reconcileExistingArgoSecret(cr *argoprojv1a1.ArgoCD, secret *corev1.Secret, clusterSecret *corev1.Secret, tlsSecret *corev1.Secret) error {
	changed := false

	if argocd.HasArgoAdminPasswordChanged(secret, clusterSecret) {
		hashedPassword, err := argopass.HashPassword(string(clusterSecret.Data[common.ArgoCDKeyAdminPassword]))
		if err != nil {
			return err
		}

		secret.Data[common.ArgoCDKeyAdminPassword] = []byte(hashedPassword)
		secret.Data[common.ArgoCDKeyAdminPasswordMTime] = argocd.NowBytes()
		changed = true
	}

	if argocd.HasArgoTLSChanged(secret, tlsSecret) {
		secret.Data[common.ArgoCDKeyTLSCert] = tlsSecret.Data[common.ArgoCDKeyTLSCert]
		secret.Data[common.ArgoCDKeyTLSPrivateKey] = tlsSecret.Data[common.ArgoCDKeyTLSPrivateKey]
		changed = true
	}

	if changed {
		logr.Info("updating argo secret")
		if err := r.Client.Update(context.TODO(), secret); err != nil {
			return err
		}

		// Trigger rollout of Argo Server Deployment
		deploy := argocd.NewDeploymentWithSuffix("server", "server", cr)
		return r.triggerRollout(deploy, "secret.changed")
	}

	return nil
}

// reconcileClusterSecrets will reconcile all Secret resources for the ArgoCD cluster.
func (r *ArgoCDReconciler) reconcileClusterSecrets(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileClusterMainSecret(cr); err != nil {
		return err
	}

	if err := r.reconcileClusterCASecret(cr); err != nil {
		return err
	}

	if err := r.reconcileClusterTLSSecret(cr); err != nil {
		return err
	}

	if err := r.reconcileClusterPermissionsSecret(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaSecret(cr); err != nil {
		return err
	}

	return nil
}

// reconcileGrafanaSecret will ensure that the Grafana Secret is present.
func (r *ArgoCDReconciler) reconcileGrafanaSecret(cr *argoprojv1a1.ArgoCD) error {
	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	clusterSecret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "cluster")
	secret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "grafana")

	if !argoutil.IsObjectFound(r.Client, cr.Namespace, clusterSecret.Name, clusterSecret) {
		logr.Info(fmt.Sprintf("cluster secret [%s] not found, waiting to reconcile grafana secret [%s]", clusterSecret.Name, secret.Name))
		return nil
	}

	if argoutil.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		actual := string(secret.Data[common.ArgoCDKeyGrafanaAdminPassword])
		expected := string(clusterSecret.Data[common.ArgoCDKeyAdminPassword])

		if actual != expected {
			logr.Info("cluster secret changed, updating and reloading grafana")
			secret.Data[common.ArgoCDKeyGrafanaAdminPassword] = clusterSecret.Data[common.ArgoCDKeyAdminPassword]
			if err := r.Client.Update(context.TODO(), secret); err != nil {
				return err
			}

			// Regenerate the Grafana configuration
			cm := argocd.NewConfigMapWithSuffix("grafana-config", cr)
			if !argoutil.IsObjectFound(r.Client, cm.Namespace, cm.Name, cm) {
				logr.Info("unable to locate grafana-config")
				return nil
			}

			if err := r.Client.Delete(context.TODO(), cm); err != nil {
				return err
			}

			// Trigger rollout of Grafana Deployment
			deploy := argocd.NewDeploymentWithSuffix("grafana", "grafana", cr)
			return r.triggerRollout(deploy, "admin.password.changed")
		}
		return nil // Nothing has changed, move along...
	}

	// Secret not found, create it...

	secretKey, err := argocd.GenerateGrafanaSecretKey()
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		common.ArgoCDKeyGrafanaAdminUsername: []byte(common.ArgoCDDefaultGrafanaAdminUsername),
		common.ArgoCDKeyGrafanaAdminPassword: clusterSecret.Data[common.ArgoCDKeyAdminPassword],
		common.ArgoCDKeyGrafanaSecretKey:     secretKey,
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), secret)
}

// triggerRollout will trigger a rollout of a Kubernetes resource specified as
// obj. It currently supports Deployment and StatefulSet resources.
func (r *ArgoCDReconciler) triggerRollout(obj interface{}, key string) error {
	switch res := obj.(type) {
	case *appsv1.Deployment:
		return r.triggerDeploymentRollout(res, key)
	case *appsv1.StatefulSet:
		return r.triggerStatefulSetRollout(res, key)
	default:
		return fmt.Errorf("resource of unknown type %T, cannot trigger rollout", res)
	}
}

// triggerStatefulSetRollout will update the label with the given key to trigger a new rollout of the StatefulSet.
func (r *ArgoCDReconciler) triggerStatefulSetRollout(sts *appsv1.StatefulSet, key string) error {
	if !argoutil.IsObjectFound(r.Client, sts.Namespace, sts.Name, sts) {
		logr.Info(fmt.Sprintf("unable to locate deployment with name: %s", sts.Name))
		return nil
	}

	sts.Spec.Template.ObjectMeta.Labels[key] = argocd.NowNano()
	return r.Client.Update(context.TODO(), sts)
}

// triggerDeploymentRollout will update the label with the given key to trigger a new rollout of the Deployment.
func (r *ArgoCDReconciler) triggerDeploymentRollout(deployment *appsv1.Deployment, key string) error {
	if !argoutil.IsObjectFound(r.Client, deployment.Namespace, deployment.Name, deployment) {
		logr.Info(fmt.Sprintf("unable to locate deployment with name: %s", deployment.Name))
		return nil
	}

	deployment.Spec.Template.ObjectMeta.Labels[key] = argocd.NowNano()
	return r.Client.Update(context.TODO(), deployment)
}

// reconcileClusterPermissionsSecret ensures ArgoCD instance is namespace-scoped
func (r *ArgoCDReconciler) reconcileClusterPermissionsSecret(cr *argoprojv1a1.ArgoCD) error {
	var clusterConfigInstance bool
	secret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "default-cluster-config")
	secret.Labels[common.ArgoCDSecretTypeLabel] = "cluster"
	dataBytes, _ := json.Marshal(map[string]interface{}{
		"tlsClientConfig": map[string]interface{}{
			"insecure": false,
		},
	})

	secret.Data = map[string][]byte{
		"config":     dataBytes,
		"name":       []byte("in-cluster"),
		"server":     []byte(common.ArgoCDDefaultServer),
		"namespaces": []byte(cr.Namespace),
	}

	if argocd.AllowedNamespace(cr.Namespace, os.Getenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")) {
		clusterConfigInstance = true
	}

	clusterSecrets := &corev1.SecretList{}
	opts := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			common.ArgoCDSecretTypeLabel: "cluster",
		}),
		Namespace: cr.Namespace,
	}

	if err := r.Client.List(context.TODO(), clusterSecrets, opts); err != nil {
		return err
	}
	for _, s := range clusterSecrets.Items {
		// check if cluster secret with default server address exists
		// do nothing if exists.
		if string(s.Data["server"]) == common.ArgoCDDefaultServer {
			if clusterConfigInstance {
				r.Client.Delete(context.TODO(), &s)
			} else {
				return nil
			}
		}
	}

	if clusterConfigInstance {
		// do nothing
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), secret)
}

// reconcileClusterTLSSecret ensures the TLS Secret is created for the ArgoCD cluster.
func (r *ArgoCDReconciler) reconcileClusterTLSSecret(cr *argoprojv1a1.ArgoCD) error {
	secret := argoutil.NewTLSSecret(cr.ObjectMeta, "tls")
	if argoutil.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return nil // Secret found, do nothing
	}

	caSecret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "ca")
	caSecret, err := argoutil.FetchSecret(r.Client, cr.ObjectMeta, caSecret.Name)
	if err != nil {
		return err
	}

	caCert, err := argoutil.ParsePEMEncodedCert(caSecret.Data[corev1.TLSCertKey])
	if err != nil {
		return err
	}

	caKey, err := argoutil.ParsePEMEncodedPrivateKey(caSecret.Data[corev1.TLSPrivateKeyKey])
	if err != nil {
		return err
	}

	secret, err = argocd.NewCertificateSecret("tls", caCert, caKey, cr)
	if err != nil {
		return err
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}

	return r.Client.Create(context.TODO(), secret)
}

// reconcileClusterMainSecret will ensure that the main Secret is present for the Argo CD cluster.
func (r *ArgoCDReconciler) reconcileClusterMainSecret(cr *argoprojv1a1.ArgoCD) error {
	secret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "cluster")
	if argoutil.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return nil // Secret found, do nothing
	}

	adminPassword, err := argocd.GenerateArgoAdminPassword()
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		common.ArgoCDKeyAdminPassword: adminPassword,
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), secret)
}

// reconcileCertificateAuthority will reconcile all Certificate Authority resources.
func (r *ArgoCDReconciler) reconcileCertificateAuthority(cr *argoprojv1a1.ArgoCD) error {
	logr.Info("reconciling CA secret")
	if err := r.reconcileClusterCASecret(cr); err != nil {
		return err
	}

	logr.Info("reconciling CA config map")
	if err := r.reconcileCAConfigMap(cr); err != nil {
		return err
	}
	return nil
}

// reconcileCAConfigMap will ensure that the Certificate Authority ConfigMap is present.
// This ConfigMap holds the CA Certificate data for client use.
func (r *ArgoCDReconciler) reconcileCAConfigMap(cr *argoprojv1a1.ArgoCD) error {
	cm := argocd.NewConfigMapWithName(argocd.GetCAConfigMapName(cr), cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	caSecret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, common.ArgoCDCASuffix)
	if !argoutil.IsObjectFound(r.Client, cr.Namespace, caSecret.Name, caSecret) {
		logr.Info(fmt.Sprintf("ca secret [%s] not found, waiting to reconcile ca configmap [%s]", caSecret.Name, cm.Name))
		return nil
	}

	cm.Data = map[string]string{
		common.ArgoCDKeyTLSCert: string(caSecret.Data[common.ArgoCDKeyTLSCert]),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileClusterCASecret ensures the CA Secret is created for the ArgoCD cluster.
func (r *ArgoCDReconciler) reconcileClusterCASecret(cr *argoprojv1a1.ArgoCD) error {
	secret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "ca")
	if argoutil.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return nil // Secret found, do nothing
	}

	secret, err := argocd.NewCASecret(cr)
	if err != nil {
		return err
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), secret)
}

// reconcileServiceAccounts will ensure that all ArgoCD Service Accounts are configured.
func (r *ArgoCDReconciler) reconcileServiceAccounts(cr *argoprojv1a1.ArgoCD) error {

	if err := r.reconcileServiceAccountPermissions(common.ArgoCDServerComponent, argocd.PolicyRuleForServer(), cr); err != nil {
		return err
	}

	if err := r.reconcileServiceAccountPermissions(common.ArgoCDDexServerComponent, argocd.PolicyRuleForDexServer(), cr); err != nil {
		return err
	}

	if err := r.reconcileServiceAccountPermissions(common.ArgoCDApplicationControllerComponent, argocd.PolicyRuleForApplicationController(), cr); err != nil {
		return err
	}

	if err := r.reconcileServiceAccountPermissions(common.ArgoCDRedisHAComponent, argocd.PolicyRuleForRedisHa(cr), cr); err != nil {
		return err
	}

	if err := r.reconcileServiceAccountClusterPermissions(common.ArgoCDServerComponent, argocd.PolicyRuleForServerClusterRole(), cr); err != nil {
		return err
	}

	if err := r.reconcileServiceAccountClusterPermissions(common.ArgoCDApplicationControllerComponent, argocd.PolicyRuleForApplicationController(), cr); err != nil {
		return err
	}

	// specialized handling for dex

	if err := r.reconcileDexServiceAccount(cr); err != nil {
		return err
	}

	return nil
}

func (r *ArgoCDReconciler) reconcileServiceAccountClusterPermissions(name string, rules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) error {
	var role *v1.ClusterRole
	var sa *corev1.ServiceAccount
	var err error

	sa, err = r.reconcileServiceAccount(name, cr)
	if err != nil {
		return err
	}

	if role, err = r.reconcileClusterRole(name, rules, cr); err != nil {
		return err
	}

	return r.reconcileClusterRoleBinding(name, role, sa, cr)
}

func (r *ArgoCDReconciler) reconcileClusterRoleBinding(name string, role *v1.ClusterRole, sa *corev1.ServiceAccount, cr *argoprojv1a1.ArgoCD) error {

	// get expected name
	roleBinding := argocd.NewClusterRoleBindingWithname(name, cr)
	// fetch existing rolebinding by name
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name}, roleBinding)
	roleBindingExists := true
	if err != nil {
		if !crierrors.IsNotFound(err) {
			return err
		}
		roleBindingExists = false
		roleBinding = argocd.NewClusterRoleBindingWithname(name, cr)
	}

	if roleBindingExists && role == nil {
		return r.Client.Delete(context.TODO(), roleBinding)
	}

	if !roleBindingExists && role == nil {
		// DO Nothing
		return nil
	}

	roleBinding.Subjects = []v1.Subject{
		{
			Kind:      v1.ServiceAccountKind,
			Name:      argocd.GenerateResourceName(name, cr),
			Namespace: cr.Namespace,
		},
	}
	roleBinding.RoleRef = v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     "ClusterRole",
		Name:     argocd.GenerateUniqueResourceName(name, cr),
	}

	controllerutil.SetControllerReference(cr, roleBinding, r.Scheme)
	if roleBindingExists {
		return r.Client.Update(context.TODO(), roleBinding)
	}
	return r.Client.Create(context.TODO(), roleBinding)
}

func (r *ArgoCDReconciler) reconcileServiceAccountPermissions(name string, rules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) error {
	return r.reconcileRoleBinding(name, rules, cr)
}

// reconcileRoleBindings will ensure that all ArgoCD RoleBindings are configured.
func (r *ArgoCDReconciler) reconcileRoleBindings(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileRoleBinding(applicationController, argocd.PolicyRuleForApplicationController(), cr); err != nil {
		return fmt.Errorf("error reconciling roleBinding for %q: %w", applicationController, err)
	}
	if err := r.reconcileRoleBinding(dexServer, argocd.PolicyRuleForDexServer(), cr); err != nil {
		return fmt.Errorf("error reconciling roleBinding for %q: %w", dexServer, err)
	}

	if err := r.reconcileRoleBinding(redisHa, argocd.PolicyRuleForRedisHa(cr), cr); err != nil {
		return fmt.Errorf("error reconciling roleBinding for %q: %w", redisHa, err)
	}

	if err := r.reconcileRoleBinding(server, argocd.PolicyRuleForServer(), cr); err != nil {
		return fmt.Errorf("error reconciling roleBinding for %q: %w", server, err)
	}
	return nil
}

func (r *ArgoCDReconciler) reconcileRoleBinding(name string, rules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) error {
	var role *v1.Role
	var sa *corev1.ServiceAccount
	var error error

	if role, error = r.reconcileRole(name, rules, cr); error != nil {
		return error
	}

	if sa, error = r.reconcileServiceAccount(name, cr); error != nil {
		return error
	}

	// get expected name
	roleBinding := argocd.NewRoleBindingWithname(name, cr)

	// fetch existing rolebinding by name
	existingRoleBinding := &v1.RoleBinding{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: cr.Namespace}, existingRoleBinding)
	roleBindingExists := true
	if err != nil {
		if !crierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s : %s", name, err)
		}
		if name == dexServer && argocd.IsDexDisabled() {
			return nil // Dex is disabled, do nothing
		}
		roleBindingExists = false
		roleBinding = argocd.NewRoleBindingWithname(name, cr)
	}

	roleBinding.Subjects = []v1.Subject{
		{
			Kind:      v1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
	}
	roleBinding.RoleRef = v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     "Role",
		Name:     role.Name,
	}

	if roleBindingExists {
		if name == dexServer && argocd.IsDexDisabled() {
			// Delete any existing RoleBinding created for Dex
			return r.Client.Delete(context.TODO(), roleBinding)
		}

		// if the RoleRef changes, delete the existing role binding and create a new one
		if !reflect.DeepEqual(roleBinding.RoleRef, existingRoleBinding.RoleRef) {
			if err = r.Client.Delete(context.TODO(), existingRoleBinding); err != nil {
				return err
			}
		} else {
			existingRoleBinding.Subjects = roleBinding.Subjects
			return r.Client.Update(context.TODO(), existingRoleBinding)
		}
	}

	controllerutil.SetControllerReference(cr, roleBinding, r.Scheme)
	return r.Client.Create(context.TODO(), roleBinding)
}

func (r *ArgoCDReconciler) reconcileServiceAccount(name string, cr *argoprojv1a1.ArgoCD) (*corev1.ServiceAccount, error) {
	sa := argocd.NewServiceAccountWithName(name, cr)

	exists := true
	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		if !crierrors.IsNotFound(err) {
			return nil, err
		}
		if name == dexServer && argocd.IsDexDisabled() {
			return sa, nil // Dex is disabled, do nothing
		}
		exists = false
	}
	if exists {
		if name == dexServer && argocd.IsDexDisabled() {
			// Delete any existing Service Account created for Dex
			return sa, r.Client.Delete(context.TODO(), sa)
		}
		return sa, nil
	}

	if err := controllerutil.SetControllerReference(cr, sa, r.Scheme); err != nil {
		return nil, err
	}

	err := r.Client.Create(context.TODO(), sa)
	if err != nil {
		return nil, err
	}

	return sa, nil
}

// reconcileDexServiceAccount will ensure that the Dex ServiceAccount is configured properly for OpenShift OAuth.
func (r *ArgoCDReconciler) reconcileDexServiceAccount(cr *argoprojv1a1.ArgoCD) error {
	if !cr.Spec.Dex.OpenShiftOAuth {
		return nil // OpenShift OAuth not enabled, move along...
	}

	logr.Info("oauth enabled, configuring dex service account")
	sa := argocd.NewServiceAccountWithName(common.ArgoCDDefaultDexServiceAccountName, cr)
	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		return err
	}

	// Get the OAuth redirect URI that should be used.
	uri := r.getDexOAuthRedirectURI(cr)
	logr.Info(fmt.Sprintf("URI: %s", uri))

	// Get the current redirect URI
	ann := sa.ObjectMeta.Annotations
	currentURI, found := ann[common.ArgoCDKeyDexOAuthRedirectURI]
	if found && currentURI == uri {
		return nil // Redirect URI annotation found and correct, move along...
	}

	logr.Info(fmt.Sprintf("current URI: %s is not correct, should be: %s", currentURI, uri))
	if len(ann) <= 0 {
		ann = make(map[string]string)
	}

	ann[common.ArgoCDKeyDexOAuthRedirectURI] = uri
	sa.ObjectMeta.Annotations = ann

	return r.Client.Update(context.TODO(), sa)
}

// getDexOAuthRedirectURI will return the OAuth redirect URI for the Dex server.
func (r *ArgoCDReconciler) getDexOAuthRedirectURI(cr *argoprojv1a1.ArgoCD) string {
	uri := r.getArgoServerURI(cr)
	return uri + common.ArgoCDDefaultDexOAuthRedirectPath
}

// getArgoServerURI will return the URI for the ArgoCD server.
// The hostname for argocd-server is from the route, ingress, an external hostname or service name in that order.
func (r *ArgoCDReconciler) getArgoServerURI(cr *argoprojv1a1.ArgoCD) string {
	host := argocd.NameWithSuffix("server", cr) // Default to service name

	// Use the external hostname provided by the user
	if cr.Spec.Server.Host != "" {
		host = cr.Spec.Server.Host
	}

	// Use Ingress host if enabled
	if cr.Spec.Server.Ingress.Enabled {
		ing := argocd.NewIngressWithSuffix("server", cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, ing.Name, ing) {
			host = ing.Spec.Rules[0].Host
		}
	}

	// Use Route host if available, override Ingress if both exist
	if argocd.IsRouteAPIAvailable() {
		route := argocd.NewRouteWithSuffix("server", cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, route.Name, route) {
			host = route.Spec.Host
		}
	}

	return fmt.Sprintf("https://%s", host) // TODO: Safe to assume HTTPS here?
}

// reconcileRoles will ensure that all ArgoCD Service Accounts are configured.
func (r *ArgoCDReconciler) reconcileRoles(cr *argoprojv1a1.ArgoCD) (role *v1.Role, err error) {
	if role, err := r.reconcileRole(applicationController, argocd.PolicyRuleForApplicationController(), cr); err != nil {
		return role, err
	}

	if role, err := r.reconcileRole(dexServer, argocd.PolicyRuleForDexServer(), cr); err != nil {
		return role, err
	}

	if role, err := r.reconcileRole(server, argocd.PolicyRuleForServer(), cr); err != nil {
		return role, err
	}

	if role, err := r.reconcileRole(redisHa, argocd.PolicyRuleForRedisHa(cr), cr); err != nil {
		return role, err
	}

	if _, err := r.reconcileClusterRole(applicationController, argocd.PolicyRuleForApplicationController(), cr); err != nil {
		return nil, err
	}

	if _, err := r.reconcileClusterRole(server, argocd.PolicyRuleForServerClusterRole(), cr); err != nil {
		return nil, err
	}

	return nil, nil
}

func (r *ArgoCDReconciler) reconcileClusterRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) (*v1.ClusterRole, error) {
	allowed := false
	if argocd.AllowedNamespace(cr.Namespace, os.Getenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")) {
		allowed = true
	}
	clusterRole := argocd.NewClusterRole(name, policyRules, cr)
	if err := argocd.ApplyReconcilerHook(cr, clusterRole, ""); err != nil {
		return nil, err
	}

	existingClusterRole := &v1.ClusterRole{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRole.Name}, existingClusterRole)
	if err != nil {
		if !crierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the cluster role for the service account associated with %s : %s", name, err)
		}
		if !allowed {
			// Do Nothing
			return nil, nil
		}
		controllerutil.SetControllerReference(cr, clusterRole, r.Scheme)
		return clusterRole, r.Client.Create(context.TODO(), clusterRole)
	}

	if !allowed {
		return nil, r.Client.Delete(context.TODO(), existingClusterRole)
	}

	existingClusterRole.Rules = clusterRole.Rules
	return existingClusterRole, r.Client.Update(context.TODO(), existingClusterRole)
}

// reconcileRole
func (r *ArgoCDReconciler) reconcileRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) (*v1.Role, error) {
	role := argocd.NewRole(name, policyRules, cr)
	if err := argocd.ApplyReconcilerHook(cr, role, ""); err != nil {
		return nil, err
	}
	existingRole := v1.Role{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: cr.Namespace}, &existingRole)
	if err != nil {
		if !crierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the role for the service account associated with %s : %s", name, err)
		}
		if name == dexServer && argocd.IsDexDisabled() {
			return role, nil // Dex is disabled, do nothing
		}
		controllerutil.SetControllerReference(cr, role, r.Scheme)
		return role, r.Client.Create(context.TODO(), role)
	}

	if name == dexServer && argocd.IsDexDisabled() {
		// Delete any existing Role created for Dex
		return role, r.Client.Delete(context.TODO(), role)
	}
	existingRole.Rules = role.Rules
	return &existingRole, r.Client.Update(context.TODO(), &existingRole)
}

// reconcileStatus will ensure that all of the Status properties are updated for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileStatus(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileStatusApplicationController(cr); err != nil {
		return err
	}

	if err := r.reconcileStatusDex(cr); err != nil {
		return err
	}

	if err := r.reconcileStatusPhase(cr); err != nil {
		return err
	}

	if err := r.reconcileStatusRedis(cr); err != nil {
		return err
	}

	if err := r.reconcileStatusRepo(cr); err != nil {
		return err
	}

	if err := r.reconcileStatusServer(cr); err != nil {
		return err
	}
	return nil
}

// reconcileStatusServer will ensure that the Server status is updated for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileStatusServer(cr *argoprojv1a1.ArgoCD) error {
	status := "Unknown"

	deploy := argocd.NewDeploymentWithSuffix("server", "server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		status = "Pending"

		// TODO: Refactor these checks.
		if deploy.Spec.Replicas != nil {
			if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
				status = "Running"
			}
		}
	}

	if cr.Status.Server != status {
		cr.Status.Server = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}

// reconcileStatusRepo will ensure that the Repo status is updated for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileStatusRepo(cr *argoprojv1a1.ArgoCD) error {
	status := "Unknown"

	deploy := argocd.NewDeploymentWithSuffix("repo-server", "repo-server", cr)
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

// reconcileStatusRedis will ensure that the Redis status is updated for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileStatusRedis(cr *argoprojv1a1.ArgoCD) error {
	status := "Unknown"

	if !cr.Spec.HA.Enabled {
		deploy := argocd.NewDeploymentWithSuffix("redis", "redis", cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
			status = "Pending"

			if deploy.Spec.Replicas != nil {
				if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
					status = "Running"
				}
			}
		}
	} else {
		ss := argocd.NewStatefulSetWithSuffix("redis-ha-server", "redis-ha-server", cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, ss.Name, ss) {
			status = "Pending"

			if ss.Status.ReadyReplicas == *ss.Spec.Replicas {
				status = "Running"
			}
		}
		// TODO: Add check for HA proxy deployment here as well?
	}

	if cr.Status.Redis != status {
		cr.Status.Redis = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}

// reconcileStatusPhase will ensure that the Status Phase is updated for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileStatusPhase(cr *argoprojv1a1.ArgoCD) error {
	var phase string

	if cr.Status.ApplicationController == "Running" && cr.Status.Redis == "Running" && cr.Status.Repo == "Running" && cr.Status.Server == "Running" {
		phase = "Available"
	} else {
		phase = "Pending"
	}

	if cr.Status.Phase != phase {
		cr.Status.Phase = phase
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}

// reconcileStatusApplicationController will ensure that the ApplicationController Status is updated for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileStatusApplicationController(cr *argoprojv1a1.ArgoCD) error {
	status := "Unknown"

	ss := argocd.NewStatefulSetWithSuffix("application-controller", "application-controller", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, ss.Name, ss) {
		status = "Pending"

		if ss.Spec.Replicas != nil {
			if ss.Status.ReadyReplicas == *ss.Spec.Replicas {
				status = "Running"
			}
		}
	}

	if cr.Status.ApplicationController != status {
		cr.Status.ApplicationController = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}

// reconcileStatusDex will ensure that the Dex status is updated for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileStatusDex(cr *argoprojv1a1.ArgoCD) error {
	status := "Unknown"

	deploy := argocd.NewDeploymentWithSuffix("dex-server", "dex-server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		status = "Pending"

		if deploy.Spec.Replicas != nil {
			if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
				status = "Running"
			}
		}
	}

	if cr.Status.Dex != status {
		cr.Status.Dex = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}
