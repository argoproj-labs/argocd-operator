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
	binaryControllerCmd    = "controller"
	binaryAPIServerCmd     = "apiserver"
)

// deploymentReconciler represents the functions to fill in spots in the deployment spec that
// differ between the components
type deploymentConfig struct {
	command         []string
	args            []string
	securityContext *corev1.SecurityContext
	livenessProbe   *corev1.Probe
	readinessProbe  *corev1.Probe
}

func ReconcilePromoterControllerDeployment(client client.Client, compName string, sa *corev1.ServiceAccount, cr *argoproj.ArgoCD, scheme *runtime.Scheme) (*appsv1.Deployment, error) {
	config := deploymentConfig{
		command:         buildContainerCommand(binaryControllerCmd),
		securityContext: buildControllerSecurityContext(),
		livenessProbe:   buildControllerLivenessProbe(),
		readinessProbe:  buildControllerReadinessProbe(),
	}

	deployment, err := ReconcilePromoterDeployment(client, compName, sa, cr, scheme, config, true)
	if err != nil {
		return nil, err
	}
	return deployment, nil
}

func ReconcilePromoterAPIServerDeployment(client client.Client, compName string, sa *corev1.ServiceAccount, cr *argoproj.ArgoCD, scheme *runtime.Scheme) (*appsv1.Deployment, error) {
	config := deploymentConfig{
		command:         buildContainerCommand(binaryAPIServerCmd),
		args:            buildAPIServerArgs(),
		securityContext: buildAPIServerSecurityContext(),
		livenessProbe:   buildAPIServerLivenessProbe(),
		readinessProbe:  buildAPIServerReadinessProbe(),
	}

	enabled := cr.Spec.Promoter == nil || cr.Spec.Promoter.APIServer.IsEnabled()
	deployment, err := ReconcilePromoterDeployment(client, compName, sa, cr, scheme, config, enabled)
	if err != nil {
		return nil, err
	}
	return deployment, nil
}

func ReconcilePromoterDeployment(client client.Client, compName string, sa *corev1.ServiceAccount, cr *argoproj.ArgoCD, scheme *runtime.Scheme, config deploymentConfig, enabled bool) (*appsv1.Deployment, error) {
	deployment := buildDeployment(cr, compName)

	exists := true
	if err := argoutil.FetchObject(client, deployment.Namespace, deployment.Name, deployment); err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
		exists = false
	}

	// TODO: add reconcilation logic for the args and also custom settings of the args because the promoter has no env args ATM
	if exists {
		if !cr.Spec.Promoter.IsEnabled() || !enabled {
			argoutil.LogResourceDeletion(log, deployment, "promoter deployment is being deleted due to being disabled")
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

		// if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Command, config.command) {
		// 	deployment.Spec.Template.Spec.Containers[0].Command = config.command
		// 	changed = true
		// }

		if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Image, selectImage(cr)) {
			deployment.Spec.Template.Spec.Containers[0].Image = selectImage(cr)
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

		if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].SecurityContext, config.securityContext) {
			deployment.Spec.Template.Spec.Containers[0].SecurityContext = config.securityContext
			changed = true
		}

		if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Resources, getResources(cr)) {
			deployment.Spec.Template.Spec.Containers[0].Resources = getResources(cr)
			changed = true
		}

		if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].LivenessProbe, config.livenessProbe) {
			deployment.Spec.Template.Spec.Containers[0].LivenessProbe = config.livenessProbe
			changed = true
		}

		if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].ReadinessProbe, config.livenessProbe) {
			deployment.Spec.Template.Spec.Containers[0].ReadinessProbe = config.readinessProbe
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

	if !cr.Spec.Promoter.IsEnabled() || !enabled {
		return deployment, nil
	}
	deployment.Spec = buildDeploymentSpec(compName, sa, cr, config)
	if err := controllerutil.SetControllerReference(cr, deployment, scheme); err != nil {
		return nil, fmt.Errorf("failed to set argocd cr %s as owner for deployment %s: %v", cr.Name, sa.Name, err)
	}

	argoutil.LogResourceCreation(log, deployment)
	if err := client.Create(context.Background(), deployment); err != nil {
		return nil, fmt.Errorf("failed to create deployment %s: %v", deployment.Name, err)
	}
	return deployment, nil
}

func buildDeployment(cr *argoproj.ArgoCD, compName string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generatePromoterResourceName(compName, cr),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForPromoterResources(compName, cr),
		},
	}
}

func buildDeploymentSpec(compName string, sa *corev1.ServiceAccount, cr *argoproj.ArgoCD, config deploymentConfig) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		Selector: buildSelector(compName, cr),
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: buildLabelsForPromoterResources(compName, cr),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Command:         config.command,
						Image:           selectImage(cr),
						ImagePullPolicy: argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy),
						Name:            generatePromoterResourceName(compName, cr),
						Args:            config.args,
						Env:             cr.Spec.Promoter.Env,
						SecurityContext: config.securityContext,
						Resources:       getResources(cr),
						LivenessProbe:   config.livenessProbe,
						ReadinessProbe:  config.readinessProbe,
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

func buildContainerCommand(cmd string) []string {
	return []string{"/usr/bin/tini", "--", "/gitops-promoter", cmd}
}

func selectImage(cr *argoproj.ArgoCD) string {
	if cr.Spec.Promoter.Image != "" {
		return cr.Spec.Promoter.Image
	}

	if image := os.Getenv(EnvGitOpsPromoterImage); image != "" {
		return image
	}

	return common.GitOpsPromoterDefaultImageName
}

func buildControllerSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}

func buildAPIServerSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		RunAsNonRoot: ptr.To(true),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

func getResources(cr *argoproj.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}
	if cr.Spec.Promoter != nil && cr.Spec.Promoter.Resources != nil {
		resources = *cr.Spec.Promoter.Resources
	}
	return resources
}

func buildControllerLivenessProbe() *corev1.Probe {
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

func buildControllerReadinessProbe() *corev1.Probe {
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

func buildAPIServerLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/healthz",
				Port:   intstr.FromString("https"),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 15,
		PeriodSeconds:       20,
	}
}

func buildAPIServerReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/readyz",
				Port:   intstr.FromString("https"),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       10,
	}
}

// #TODO: Allow for custom args through cr it seems that
func buildAPIServerArgs() []string {
	return []string{
		"--insecure-skip-tls-verify",
	}
}
