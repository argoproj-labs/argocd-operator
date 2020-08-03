// Copyright 2020 Argo CD Operator Developers
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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// getArgoApplicationControllerCommand will return the command for the ArgoCD Application Controller component.
func getArgoApplicationControllerCommand(cr *v1alpha1.ArgoCD) []string {
	cmd := make([]string, 0)
	cmd = append(cmd, "argocd-application-controller")

	cmd = append(cmd, "--operation-processors")
	cmd = append(cmd, fmt.Sprint(getArgoServerOperationProcessors(cr)))

	cmd = append(cmd, "--redis")
	cmd = append(cmd, getRedisServerAddress(cr))

	cmd = append(cmd, "--repo-server")
	cmd = append(cmd, common.NameWithSuffix(cr.ObjectMeta, "repo-server:8081"))

	cmd = append(cmd, "--status-processors")
	cmd = append(cmd, fmt.Sprint(getArgoServerStatusProcessors(cr)))

	return cmd
}

// getArgoApplicationControllerResources will return the ResourceRequirements for the Argo CD application controller container.
func getArgoApplicationControllerResources(cr *v1alpha1.ArgoCD) corev1.ResourceRequirements {
	// resources := corev1.ResourceRequirements{
	// 	Limits: corev1.ResourceList{
	// 		corev1.ResourceCPU:    resource.MustParse(common.ArgoCDDefaultControllerResourceLimitCPU),
	// 		corev1.ResourceMemory: resource.MustParse(common.ArgoCDDefaultControllerResourceLimitMemory),
	// 	},
	// 	Requests: corev1.ResourceList{
	// 		corev1.ResourceCPU:    resource.MustParse(common.ArgoCDDefaultControllerResourceRequestCPU),
	// 		corev1.ResourceMemory: resource.MustParse(common.ArgoCDDefaultControllerResourceRequestMemory),
	// 	},
	// }

	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Controller.Resources != nil {
		resources = *cr.Spec.Controller.Resources
	}

	return resources
}

// reconcileApplicationControllerDeployment will ensure the Deployment resource is present for the ArgoCD Application Controller component.
func (r *ArgoClusterReconciler) reconcileApplicationControllerDeployment(cr *v1alpha1.ArgoCD) error {
	deploy := resources.NewDeploymentWithSuffix(cr.ObjectMeta, "application-controller", "application-controller")
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
			Command:         r.getArgoImportCommand(cr),
			Env:             getArgoImportContainerEnv(export),
			Image:           getArgoImportContainerImage(export),
			ImagePullPolicy: corev1.PullAlways,
			Name:            "argocd-import",
			VolumeMounts:    getArgoImportVolumeMounts(export),
		}}

		podSpec.Volumes = getArgoImportVolumes(export)
	}

	podSpec.ServiceAccountName = "argocd-application-controller"

	ctrl.SetControllerReference(cr, deploy, r.Scheme)
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileMetricsService will ensure that the Service for the Argo CD application controller metrics is present.
func (r *ArgoClusterReconciler) reconcileMetricsService(cr *v1alpha1.ArgoCD) error {
	svc := resources.NewServiceWithSuffix(cr.ObjectMeta, "metrics", "metrics")
	if resources.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		// Service found, do nothing
		return nil
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: common.NameWithSuffix(cr.ObjectMeta, "application-controller"),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       8082,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8082),
		},
	}

	ctrl.SetControllerReference(cr, svc, r.Scheme)
	return r.Client.Create(context.TODO(), svc)
}

// reconcileStatusApplicationController will ensure that the ApplicationController Status is updated for the given ArgoCD.
func (r *ArgoClusterReconciler) reconcileStatusApplicationController(cr *v1alpha1.ArgoCD) error {
	status := "Unknown"

	deploy := resources.NewDeploymentWithSuffix(cr.ObjectMeta, "application-controller", "application-controller")
	if resources.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		status = "Pending"

		if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
			status = "Running"
		}
	}

	if cr.Status.ApplicationController != status {
		cr.Status.ApplicationController = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}
