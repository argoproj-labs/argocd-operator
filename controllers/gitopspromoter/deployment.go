// Copyright 2026 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package gitopspromoter handles reconcilation related to the GitOps Promoter
package gitopspromoter

import (
	"context"
	"fmt"
	"os"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	EnvGitOpsPromoterImage = "GITOPS_PROMOTER_IMAGE"
)

func ReconcilePromoterControllerDeployment(client client.Client, compName string, sa *corev1.ServiceAccount, cr *argoproj.ArgoCD, scheme *runtime.Scheme) (*appsv1.Deployment, error) {
	deployment := buildControllerDeployment(cr, compName)

	exists := true
	if err := argoutil.FetchObject(client, deployment.Namespace, deployment.Name, deployment); err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
		exists = false
	}

	if exists {
		if !cr.Spec.Promoter.IsEnabled() {
			argoutil.LogResourceDeletion(log, deployment, "promoter controller deployment is being deleted due to being disabled")
			if err := client.Delete(context.Background(), deployment); err != nil {
				return nil, fmt.Errorf("failed to delete deployment %s: %v", deployment.Name, err)
			}
			return deployment, nil
		}

		changed := false
		if !reflect.DeepEqual(deployment.Spec.Selector, buildSelector(compName, cr)) {
			deployment.Spec.Selector = buildSelector(compName, cr)
			changed = true
		}

		if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Command, buildContainerCommand()) {
			deployment.Spec.Template.Spec.Containers[0].Command = buildContainerCommand()
			changed = true
		}

		if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Image, selectControllerImage(cr)) {
			deployment.Spec.Template.Spec.Containers[0].Image = selectControllerImage(cr)
			changed = true
		}

		if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy, argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy)) {
			deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy)
			changed = true
		}

		if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Name, generatePromoterResourceName(compName, cr)) {
			deployment.Spec.Template.Spec.Containers[0].Name = generatePromoterResourceName(compName, cr)
			changed = true
		}

		if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Env, cr.Spec.Promoter.Env) {
			deployment.Spec.Template.Spec.Containers[0].Env = cr.Spec.Promoter.Env
			changed = true
		}

		if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].SecurityContext, buildSecurityContext()) {
			deployment.Spec.Template.Spec.Containers[0].SecurityContext = buildSecurityContext()
			changed = true
		}

		if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Resources, getControllerResources(cr)) {
			deployment.Spec.Template.Spec.Containers[0].Resources = getControllerResources(cr)
			changed = true
		}

		if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].LivenessProbe, buildLivenessProbe()) {
			deployment.Spec.Template.Spec.Containers[0].LivenessProbe = buildLivenessProbe()
			changed = true
		}

		if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].ReadinessProbe, buildReadinessProbe()) {
			deployment.Spec.Template.Spec.Containers[0].ReadinessProbe = buildReadinessProbe()
			changed = true
		}

		if !reflect.DeepEqual(deployment.Spec.Template.Spec.ServiceAccountName, sa.Name) {
			deployment.Spec.Template.Spec.ServiceAccountName = sa.Name
			changed = true
		}

		if !reflect.DeepEqual(deployment.Spec.Template.Spec.TerminationGracePeriodSeconds, ptr.To(int64(10))) {
			deployment.Spec.Template.Spec.TerminationGracePeriodSeconds = ptr.To(int64(10))
			changed = true
		}

		if changed {
			argoutil.LogResourceUpdate(log, deployment)
			if err := client.Update(context.Background(), deployment); err != nil {
				return nil, err
			}
			return deployment, nil
		}
		return deployment, nil
	}

	if !cr.Spec.Promoter.IsEnabled() {
		return deployment, nil
	}
	deployment.Spec = buildControllerDeploymentSpec(compName, sa, cr)
	if err := controllerutil.SetControllerReference(cr, deployment, scheme); err != nil {
		return nil, fmt.Errorf("failed to set argocd cr %s as owner for service account %s: %v", cr.Name, sa.Name, err)
	}

	argoutil.LogResourceCreation(log, deployment)
	if err := client.Create(context.Background(), deployment); err != nil {
		return nil, fmt.Errorf("failed to create controller configuration %s: %v", deployment.Name, err)
	}
	return deployment, nil
}

func buildControllerDeployment(cr *argoproj.ArgoCD, compName string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generatePromoterResourceName(compName, cr),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForPromoterResources(compName, cr),
		},
	}
}

func buildControllerDeploymentSpec(compName string, sa *corev1.ServiceAccount, cr *argoproj.ArgoCD) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		Selector: buildSelector(compName, cr),
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: buildLabelsForPromoterResources(compName, cr),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Command:         buildContainerCommand(),
						Image:           selectControllerImage(cr),
						ImagePullPolicy: argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy),
						Name:            generatePromoterResourceName(compName, cr),
						Env:             cr.Spec.Promoter.Env,
						SecurityContext: buildSecurityContext(),
						Resources:       getControllerResources(cr),
						LivenessProbe:   buildLivenessProbe(),
						ReadinessProbe:  buildReadinessProbe(),
					},
				},
				ServiceAccountName:            sa.Name,
				TerminationGracePeriodSeconds: ptr.To(int64(10)),
			},
		},
	}
}

func buildSelector(compName string, cr *argoproj.ArgoCD) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: buildLabelsForPromoterResources(compName, cr),
	}
}

func buildContainerCommand() []string {
	return []string{"/usr/bin/tini", "--", "/gitops-promoter", "controller"}
}

func selectControllerImage(cr *argoproj.ArgoCD) string {
	if cr.Spec.Promoter.Image != "" {
		return cr.Spec.Promoter.Image
	}

	if image := os.Getenv(EnvGitOpsPromoterImage); image != "" {
		return image
	}

	return common.GitOpsPromoterDefaultImageName
}

func buildSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}

func getControllerResources(cr *argoproj.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}
	if cr.Spec.Promoter != nil && cr.Spec.Promoter.Resources != nil {
		resources = *cr.Spec.Promoter.Resources
	}
	return resources
}

func buildLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/healthz",
				Port: intstr.FromInt(9081),
			},
		},
		InitialDelaySeconds: 15,
		PeriodSeconds:       20,
	}
}

func buildReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/readyz",
				Port: intstr.FromInt(9081),
			},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       10,
	}
}
