// Copyright 2025 ArgoCD Operator Developers
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

package argocdagent

import (
	"context"
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// ReconcilePrincipalDeployment reconciles the ArgoCD agent principal deployment.
// It creates, updates, or deletes the deployment based on the ArgoCD CR configuration.
func ReconcilePrincipalDeployment(client client.Client, compName, saName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme) error {
	deployment := buildDeployment(compName, cr)

	// Check if deployment already exists
	exists := true
	if err := argoutil.FetchObject(client, cr.Namespace, deployment.Name, deployment); err != nil {
		if !apiError.IsNotFound(err) {
			return fmt.Errorf("failed to get existing principal deployment %s in namespace %s: %v", deployment.Name, cr.Namespace, err)
		}
		exists = false
	}

	// If deployment exists, handle updates or deletion
	if exists {
		if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
			argoutil.LogResourceDeletion(log, deployment, "principal deployment is being deleted as principal is disabled")
			if err := client.Delete(context.TODO(), deployment); err != nil {
				return fmt.Errorf("failed to delete principal deployment %s in namespace %s: %v", deployment.Name, cr.Namespace, err)
			}
			return nil
		}

		deployment, changed := updateDeploymentIfChanged(compName, saName, cr, deployment)
		if changed {
			argoutil.LogResourceUpdate(log, deployment, "principal deployment is being updated")
			if err := client.Update(context.TODO(), deployment); err != nil {
				return fmt.Errorf("failed to update principal deployment %s in namespace %s: %v", deployment.Name, cr.Namespace, err)
			}
		}
		return nil
	}

	// If deployment doesn't exist and principal is disabled, nothing to do
	if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, deployment, scheme); err != nil {
		return fmt.Errorf("failed to set ArgoCD CR %s as owner for service %s: %w", cr.Name, deployment.Name, err)
	}

	argoutil.LogResourceCreation(log, deployment)
	deployment.Spec = buildPrincipalSpec(compName, saName, cr)
	if err := client.Create(context.TODO(), deployment); err != nil {
		return fmt.Errorf("failed to create principal deployment %s in namespace %s: %v", deployment.Name, cr.Namespace, err)
	}
	return nil
}

func buildDeployment(compName string, cr *argoproj.ArgoCD) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, compName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, compName),
		},
	}
}

func buildPrincipalSpec(compName, saName string, cr *argoproj.ArgoCD) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		Selector: buildSelector(compName, cr),
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: buildLabelsForAgentPrincipal(cr.Name, compName),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Image:           buildPrincipalImage(cr),
						ImagePullPolicy: corev1.PullAlways,
						Name:            generateAgentResourceName(cr.Name, compName),
						Env:             buildPrincipalContainerEnv(),
						Args:            buildArgs(compName),
						SecurityContext: buildSecurityContext(),
						Ports:           buildPorts(compName),
						VolumeMounts:    buildVolumeMounts(),
					},
				},
				ServiceAccountName: saName,
				Volumes:            buildVolumes(),
			},
		},
	}
}

func buildSelector(compName string, cr *argoproj.ArgoCD) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: buildLabelsForAgentPrincipal(cr.Name, compName),
	}
}

func buildSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"ALL",
			},
		},
		ReadOnlyRootFilesystem: ptr.To(true),
		RunAsNonRoot:           ptr.To(true),
		SeccompProfile: &corev1.SeccompProfile{
			Type: "RuntimeDefault",
		},
	}
}

func buildPorts(compName string) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			ContainerPort: 8443,
			Name:          compName,
		}, {
			ContainerPort: 8000,
			Name:          "metrics",
		},
	}
}

func buildArgs(compName string) []string {
	args := make([]string, 0)
	args = append(args, compName)
	return args
}

func buildPrincipalImage(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.Image != "" {
		return cr.Spec.ArgoCDAgent.Principal.Image
	}
	return "quay.io/argoproj/argocd-agent:v1"
}

func buildVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "jwt-secret",
			MountPath: "/app/config/jwt",
		},
		{
			Name:      "userpass-passwd",
			MountPath: "/app/config/userpass",
		},
	}
}

func buildVolumes() []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "jwt-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "argocd-agent-jwt",
					Items: []corev1.KeyToPath{
						{
							Key:  "jwt.key",
							Path: "jwt.key",
						},
					},
					Optional: ptr.To(true),
				},
			},
		},
		{
			Name: "userpass-passwd",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "argocd-agent-principal-userpass",
					Items: []corev1.KeyToPath{
						{
							Key:  "passwd",
							Path: "passwd",
						},
					},
					Optional: ptr.To(true),
				},
			},
		},
	}
}

// updateDeploymentIfChanged compares the current deployment with the desired state
// and updates it if any changes are detected. Returns the updated deployment and a boolean
// indicating whether any changes were made.
func updateDeploymentIfChanged(compName, saName string, cr *argoproj.ArgoCD, deployment *appsv1.Deployment) (*appsv1.Deployment, bool) {
	changed := false

	if !reflect.DeepEqual(deployment.Spec.Selector, buildSelector(compName, cr)) {
		log.Info("deployment selector is being updated")
		changed = true
		deployment.Spec.Selector = buildSelector(compName, cr)
	}

	if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Image, buildPrincipalImage(cr)) {
		log.Info("deployment image is being updated")
		changed = true
		deployment.Spec.Template.Spec.Containers[0].Image = buildPrincipalImage(cr)
	}

	if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Name, generateAgentResourceName(cr.Name, compName)) {
		log.Info("deployment container name is being updated")
		changed = true
		deployment.Spec.Template.Spec.Containers[0].Name = generateAgentResourceName(cr.Name, compName)
	}

	if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Env, buildPrincipalContainerEnv()) {
		log.Info("deployment container env is being updated")
		changed = true
		deployment.Spec.Template.Spec.Containers[0].Env = buildPrincipalContainerEnv()
	}

	if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Args, buildArgs(compName)) {
		log.Info("deployment container args is being updated")
		changed = true
		deployment.Spec.Template.Spec.Containers[0].Args = buildArgs(compName)
	}

	if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].SecurityContext, buildSecurityContext()) {
		log.Info("deployment container security context is being updated")
		changed = true
		deployment.Spec.Template.Spec.Containers[0].SecurityContext = buildSecurityContext()
	}

	if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Ports, buildPorts(compName)) {
		log.Info("deployment container ports is being updated")
		changed = true
		deployment.Spec.Template.Spec.Containers[0].Ports = buildPorts(compName)
	}

	if !reflect.DeepEqual(deployment.Spec.Template.Spec.ServiceAccountName, saName) {
		log.Info("deployment service account name is being updated")
		changed = true
		deployment.Spec.Template.Spec.ServiceAccountName = saName
	}

	return deployment, changed
}

func buildPrincipalContainerEnv() []corev1.EnvVar {

	ref := corev1.LocalObjectReference{
		Name: "argocd-agent-params",
	}

	env := []corev1.EnvVar{
		{
			Name: EnvArgoCDPrincipalListenHost,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalListenHost,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalListenPort,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalListenPort,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalLogLevel,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalLogLevel,
					LocalObjectReference: ref,
				},
			},
		}, {
			Name: EnvArgoCDPrincipalMetricsPort,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalMetricsPort,
					LocalObjectReference: ref,
				},
			},
		}, {
			Name: EnvArgoCDPrincipalNamespace,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalNamespace,
					LocalObjectReference: ref,
				},
			},
		}, {
			Name: EnvArgoCDPrincipalAllowedNamespaces,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalAllowedNamespaces,
					LocalObjectReference: ref,
				},
			},
		}, {
			Name: EnvArgoCDPrincipalNamespaceCreateEnable,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalNamespaceCreateEnable,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalNamespaceCreatePattern,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalNamespaceCreatePattern,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalNamespaceCreateLabels,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalNamespaceCreateLabels,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalTLSServerCertPath,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalTLSServerCertPath,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalTLSServerKeyPath,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalTLSServerKeyPath,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalTLSServerAllowGenerate,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalTLSServerAllowGenerate,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalTLSClientCertRequire,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalTLSClientCertRequire,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalTLSServerRootCAPath,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalTLSServerRootCAPath,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalTLSClientCertMatchSubject,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalTLSClientCertMatchSubject,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalJWTAllowGenerate,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalJwtAllowGenerate,
					LocalObjectReference: ref,
				},
			},
		}, {
			Name: EnvArgoCDPrincipalJWTKeyPath,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalJWTKeyPath,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalAuth,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalAuth,
					LocalObjectReference: ref,
				},
			},
		}, {
			Name: EnvArgoCDPrincipalEnableResourceProxy,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalEnableResourceProxy,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalKeepAliveMinInterval,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalKeepAliveMinInterval,
					LocalObjectReference: ref,
				},
			},
		}, {
			Name: EnvArgoCDPrincipalRedisServerAddress,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalRedisAddress,
					LocalObjectReference: ref,
				},
			},
		}, {
			Name: EnvArgoCDPrincipalRedisCompressionType,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalRedisCompressionType,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalPprofPort,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalPprofPort,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalLogFormat,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalLogFormat,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalResourceProxyTLSCertPath,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalResourceProxyTLSCertPath,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalResourceProxyTLSKeyPath,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalResourceProxyTLSKeyPath,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalResourceProxyTLSCAPath,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalResourceProxyTLSCAPath,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}, {
			Name: EnvArgoCDPrincipalEnableWebSocket,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key:                  PrincipalEnableWebSocket,
					LocalObjectReference: ref,
					Optional:             ptr.To(true),
				},
			},
		}}

	return env
}
