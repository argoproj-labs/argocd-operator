// Copyright 2019 Argo CD Operator Developers
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
	"time"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/resources"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// getArgoRepoCommand will return the command for the ArgoCD Repo component.
func getArgoRepoCommand(cr *v1alpha1.ArgoCD) []string {
	cmd := make([]string, 0)

	cmd = append(cmd, "uid_entrypoint.sh")
	cmd = append(cmd, "argocd-repo-server")

	cmd = append(cmd, "--redis")
	cmd = append(cmd, getRedisServerAddress(cr))

	return cmd
}

// getArgoRepoResources will return the ResourceRequirements for the Argo CD Repo server container.
func getArgoRepoResources(cr *v1alpha1.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Repo.Resources != nil {
		resources = *cr.Spec.Repo.Resources
	}

	return resources
}

// geRepoServerAddress will return the Argo CD repo server address.
func geRepoServerAddress(cr *v1alpha1.ArgoCD) string {
	return fmt.Sprintf("%s:%d", common.NameWithSuffix(cr.ObjectMeta, "repo-server"), common.ArgoCDDefaultRepoServerPort)
}

// reconcileRepoDeployment will ensure the Deployment resource is present for the ArgoCD Repo component.
func (r *ArgoClusterReconciler) reconcileRepoDeployment(cr *v1alpha1.ArgoCD) error {
	deploy := resources.NewDeploymentWithSuffix(cr.ObjectMeta, "repo-server", "repo-server")
	if resources.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		actualImage := deploy.Spec.Template.Spec.Containers[0].Image
		desiredImage := getArgoContainerImage(cr)
		if actualImage != desiredImage {
			deploy.Spec.Template.Spec.Containers[0].Image = desiredImage
			deploy.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			return r.Client.Update(context.TODO(), deploy)
		}
		return nil // Deployment found with nothing to do, move along...
	}

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
			}, {
				Name:      "tls-certs",
				MountPath: "/app/config/tls",
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

	ctrl.SetControllerReference(cr, deploy, r.Scheme)
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileRepoServerServiceMonitor will ensure that the ServiceMonitor is present for the Repo Server metrics Service.
func (r *ArgoClusterReconciler) reconcileRepoServerServiceMonitor(cr *v1alpha1.ArgoCD) error {
	sm := resources.NewServiceMonitorWithSuffix(cr.ObjectMeta, "repo-server-metrics")
	if resources.IsObjectFound(r.Client, cr.Namespace, sm.Name, sm) {
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
			common.ArgoCDKeyName: common.NameWithSuffix(cr.ObjectMeta, "repo-server"),
		},
	}
	sm.Spec.Endpoints = []monitoringv1.Endpoint{
		{
			Port: common.ArgoCDKeyMetrics,
		},
	}

	ctrl.SetControllerReference(cr, sm, r.Scheme)
	return r.Client.Create(context.TODO(), sm)
}

// reconcileRepoService will ensure that the Service for the Argo CD repo server is present.
func (r *ArgoClusterReconciler) reconcileRepoService(cr *v1alpha1.ArgoCD) error {
	svc := resources.NewServiceWithSuffix(cr.ObjectMeta, "repo-server", "repo-server")
	if resources.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: common.NameWithSuffix(cr.ObjectMeta, "repo-server"),
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

	ctrl.SetControllerReference(cr, svc, r.Scheme)
	return r.Client.Create(context.TODO(), svc)
}

// reconcileStatusRepo will ensure that the Repo status is updated for the given ArgoCD.
func (r *ArgoClusterReconciler) reconcileStatusRepo(cr *v1alpha1.ArgoCD) error {
	status := "Unknown"

	deploy := resources.NewDeploymentWithSuffix(cr.ObjectMeta, "repo-server", "repo-server")
	if resources.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		status = "Pending"

		if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
			status = "Running"
		}
	}

	if cr.Status.Repo != status {
		cr.Status.Repo = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}
