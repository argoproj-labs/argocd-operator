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

package agent

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// ReconcileAgentDeployment reconciles the ArgoCD agent's agent deployment.
// It creates, updates, or deletes the deployment based on the ArgoCD CR configuration.
func ReconcileAgentDeployment(client client.Client, compName, saName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme) error {
	deployment := buildDeployment(compName, cr)

	// Check if deployment already exists
	exists := true
	if err := argoutil.FetchObject(client, cr.Namespace, deployment.Name, deployment); err != nil {
		if !apiError.IsNotFound(err) {
			return fmt.Errorf("failed to get existing agent deployment %s in namespace %s: %v", deployment.Name, cr.Namespace, err)
		}
		exists = false
	}

	// If deployment exists, handle updates or deletion
	if exists {
		if !has(cr) || !cr.Spec.ArgoCDAgent.Agent.IsEnabled() {
			argoutil.LogResourceDeletion(log, deployment, "agent deployment is being deleted as agent is disabled")
			if err := client.Delete(context.TODO(), deployment); err != nil {
				return fmt.Errorf("failed to delete agent deployment %s in namespace %s: %v", deployment.Name, cr.Namespace, err)
			}
			return nil
		}

		deployment, changed := updateDeploymentIfChanged(compName, saName, cr, deployment)
		if changed {
			argoutil.LogResourceUpdate(log, deployment, "agent deployment is being updated")
			if err := client.Update(context.TODO(), deployment); err != nil {
				return fmt.Errorf("failed to update agent deployment %s in namespace %s: %v", deployment.Name, cr.Namespace, err)
			}
		}
		return nil
	}

	// If deployment doesn't exist and agent is disabled, nothing to do
	if !has(cr) || !cr.Spec.ArgoCDAgent.Agent.IsEnabled() {
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, deployment, scheme); err != nil {
		return fmt.Errorf("failed to set ArgoCD CR %s as owner for service %s: %w", cr.Name, deployment.Name, err)
	}

	argoutil.LogResourceCreation(log, deployment)
	deployment.Spec = buildAgentSpec(compName, saName, cr)
	if err := client.Create(context.TODO(), deployment); err != nil {
		return fmt.Errorf("failed to create agent deployment %s in namespace %s: %v", deployment.Name, cr.Namespace, err)
	}
	return nil
}

func buildDeployment(compName string, cr *argoproj.ArgoCD) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, compName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, compName),
		},
	}
}

func buildAgentSpec(compName, saName string, cr *argoproj.ArgoCD) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		Selector: buildSelector(compName, cr),
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: buildLabelsForAgent(cr.Name, compName),
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Image:           buildAgentImage(cr),
						ImagePullPolicy: corev1.PullAlways,
						Name:            generateAgentResourceName(cr.Name, compName),
						Env:             buildAgentContainerEnv(cr),
						Args:            buildArgs(compName),
						SecurityContext: buildSecurityContext(),
						Ports:           buildPorts(),
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
		MatchLabels: buildLabelsForAgent(cr.Name, compName),
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

func buildPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			ContainerPort: 8181,
			Name:          "metrics",
			Protocol:      corev1.ProtocolTCP,
		},
		{
			ContainerPort: 8002,
			Name:          "healthz",
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

func buildArgs(compName string) []string {
	args := make([]string, 0)
	args = append(args, compName)
	return args
}

func buildAgentImage(cr *argoproj.ArgoCD) string {
	// Check CR specification first
	if hasClient(cr) && cr.Spec.ArgoCDAgent.Agent.Client.Image != "" {
		return cr.Spec.ArgoCDAgent.Agent.Client.Image
	}

	// Value specified in the environment take precedence over the default
	if env := os.Getenv(EnvArgoCDAgentImage); env != "" {
		return env
	}

	// Use the default image and version if not specified in the CR or environment variable
	return common.ArgoCDAgentAgentDefaultImageName
}

func buildVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "userpass-passwd",
			MountPath: "/app/config/creds",
		},
	}
}

func buildVolumes() []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "userpass-passwd",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "argocd-agent-agent-userpass",
					Items: []corev1.KeyToPath{
						{
							Key:  "credentials",
							Path: "userpass.creds",
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

	if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Image, buildAgentImage(cr)) {
		log.Info("deployment image is being updated")
		changed = true
		deployment.Spec.Template.Spec.Containers[0].Image = buildAgentImage(cr)
	}

	if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Name, generateAgentResourceName(cr.Name, compName)) {
		log.Info("deployment container name is being updated")
		changed = true
		deployment.Spec.Template.Spec.Containers[0].Name = generateAgentResourceName(cr.Name, compName)
	}

	if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Env, buildAgentContainerEnv(cr)) {
		log.Info("deployment container env is being updated")
		changed = true
		deployment.Spec.Template.Spec.Containers[0].Env = buildAgentContainerEnv(cr)
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

	if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Ports, buildPorts()) {
		log.Info("deployment container ports is being updated")
		changed = true
		deployment.Spec.Template.Spec.Containers[0].Ports = buildPorts()
	}

	if !reflect.DeepEqual(deployment.Spec.Template.Spec.ServiceAccountName, saName) {
		log.Info("deployment service account name is being updated")
		changed = true
		deployment.Spec.Template.Spec.ServiceAccountName = saName
	}

	return deployment, changed
}

func buildAgentContainerEnv(cr *argoproj.ArgoCD) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{
			Name:  EnvArgoCDAgentLogLevel,
			Value: getAgentLogLevel(cr),
		},
		{
			Name:  EnvArgoCDAgentNamespace,
			Value: cr.Namespace,
		},
		{
			Name:  EnvArgoCDAgentServerAddress,
			Value: getAgentPrincipalServerAddress(cr),
		},
		{
			Name:  EnvArgoCDAgentServerPort,
			Value: getAgentPrincipalServerPort(cr),
		},
		{
			Name:  EnvArgoCDAgentLogFormat,
			Value: getAgentLogFormat(cr),
		},
		{
			Name:  EnvArgoCDAgentTLSSecretName,
			Value: getAgentTLSSecretName(cr),
		},
		{
			Name:  EnvArgoCDAgentTLSInsecure,
			Value: getAgentTLSInsecure(cr),
		},
		{
			Name:  EnvArgoCDAgentTLSRootCASecretName,
			Value: getAgentTLSRootCASecretName(cr),
		},
		{
			Name:  EnvArgoCDAgentMode,
			Value: getAgentMode(cr),
		},
		{
			Name:  EnvArgoCDAgentCreds,
			Value: getAgentCreds(cr),
		},
		{
			Name:  EnvArgoCDAgentEnableWebSocket,
			Value: getAgentEnableWebSocket(cr),
		},
		{
			Name:  EnvArgoCDAgentEnableCompression,
			Value: getAgentEnableCompression(cr),
		},
		{
			Name:  EnvArgoCDAgentKeepAliveInterval,
			Value: getAgentKeepAliveInterval(cr),
		},
		{
			Name:  EnvArgoCDAgentRedisAddress,
			Value: getAgentRedisAddress(cr),
		},
		{
			Name:  EnvArgoCDAgentEnableResourceProxy,
			Value: "true",
		},
		{
			Name: EnvRedisPassword,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: AgentRedisPasswordKey,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("%s-%s", cr.Name, AgentRedisSecretnameSuffix),
					},
					Optional: ptr.To(true),
				},
			},
		},
	}

	// Add custom environment variables if specified in the CR
	if hasClient(cr) && cr.Spec.ArgoCDAgent.Agent.Client.Env != nil {
		env = append(env, cr.Spec.ArgoCDAgent.Agent.Client.Env...)
	}

	return env
}

// These constants are environment variables that correspond to the environment variables
// used to configure Argo CD Agent's agent deployment, and should match the names exactly from the agent
const (
	EnvArgoCDAgentServerAddress       = "ARGOCD_AGENT_REMOTE_SERVER"
	EnvArgoCDAgentServerPort          = "ARGOCD_AGENT_REMOTE_PORT"
	EnvArgoCDAgentLogLevel            = "ARGOCD_AGENT_LOG_LEVEL"
	EnvArgoCDAgentLogFormat           = "ARGOCD_AGENT_LOG_FORMAT"
	EnvArgoCDAgentNamespace           = "ARGOCD_AGENT_NAMESPACE"
	EnvArgoCDAgentTLSSecretName       = "ARGOCD_AGENT_TLS_SECRET_NAME" // #nosec G101
	EnvArgoCDAgentTLSInsecure         = "ARGOCD_AGENT_TLS_INSECURE"
	EnvArgoCDAgentTLSRootCASecretName = "ARGOCD_AGENT_TLS_ROOT_CA_SECRET_NAME" // #nosec G101
	EnvArgoCDAgentMode                = "ARGOCD_AGENT_MODE"
	EnvArgoCDAgentCreds               = "ARGOCD_AGENT_CREDS" // #nosec G101
	EnvArgoCDAgentEnableWebSocket     = "ARGOCD_AGENT_ENABLE_WEBSOCKET"
	EnvArgoCDAgentEnableCompression   = "ARGOCD_AGENT_ENABLE_COMPRESSION"
	EnvArgoCDAgentKeepAliveInterval   = "ARGOCD_AGENT_KEEP_ALIVE_PING_INTERVAL"
	EnvArgoCDAgentEnableResourceProxy = "ARGOCD_AGENT_ENABLE_RESOURCE_PROXY"
	EnvArgoCDAgentImage               = "ARGOCD_AGENT_IMAGE"
	EnvArgoCDAgentRedisAddress        = "REDIS_ADDR"
	EnvRedisPassword                  = "REDIS_PASSWORD"
	AgentRedisPasswordKey             = "admin.password"
	AgentRedisSecretnameSuffix        = "redis-initial-password" // #nosec G101
)

// Logging Configuration
func getAgentLogLevel(cr *argoproj.ArgoCD) string {
	if hasClient(cr) && cr.Spec.ArgoCDAgent.Agent.Client.LogLevel != "" {
		return cr.Spec.ArgoCDAgent.Agent.Client.LogLevel
	}
	return "info"
}

func getAgentLogFormat(cr *argoproj.ArgoCD) string {
	if hasClient(cr) && cr.Spec.ArgoCDAgent.Agent.Client.LogFormat != "" {
		return cr.Spec.ArgoCDAgent.Agent.Client.LogFormat
	}
	return "text"
}

func getAgentPrincipalServerAddress(cr *argoproj.ArgoCD) string {
	if hasClient(cr) && cr.Spec.ArgoCDAgent.Agent.Client.PrincipalServerAddress != "" {
		return cr.Spec.ArgoCDAgent.Agent.Client.PrincipalServerAddress
	}
	return ""
}

func getAgentPrincipalServerPort(cr *argoproj.ArgoCD) string {
	if hasClient(cr) && cr.Spec.ArgoCDAgent.Agent.Client.PrincipalServerPort != "" {
		return cr.Spec.ArgoCDAgent.Agent.Client.PrincipalServerPort
	}
	return "443"
}

func getAgentTLSSecretName(cr *argoproj.ArgoCD) string {
	if hasTLS(cr) && cr.Spec.ArgoCDAgent.Agent.TLS.SecretName != "" {
		return cr.Spec.ArgoCDAgent.Agent.TLS.SecretName
	}
	return "argocd-agent-client-tls"
}

func getAgentTLSInsecure(cr *argoproj.ArgoCD) string {
	if hasTLS(cr) && cr.Spec.ArgoCDAgent.Agent.TLS.Insecure != nil && *cr.Spec.ArgoCDAgent.Agent.TLS.Insecure {
		return "true"
	}
	return "false"
}

func getAgentTLSRootCASecretName(cr *argoproj.ArgoCD) string {
	if hasTLS(cr) && cr.Spec.ArgoCDAgent.Agent.TLS.RootCASecretName != "" {
		return cr.Spec.ArgoCDAgent.Agent.TLS.RootCASecretName
	}
	return "argocd-agent-ca"
}

func getAgentMode(cr *argoproj.ArgoCD) string {
	if hasClient(cr) && cr.Spec.ArgoCDAgent.Agent.Client.Mode != "" {
		return cr.Spec.ArgoCDAgent.Agent.Client.Mode
	}
	return "managed"
}

func getAgentCreds(cr *argoproj.ArgoCD) string {
	if hasClient(cr) && cr.Spec.ArgoCDAgent.Agent.Client.Creds != "" {
		return cr.Spec.ArgoCDAgent.Agent.Client.Creds
	}
	return "mtls:any"
}

// WebSocket Configuration
func getAgentEnableWebSocket(cr *argoproj.ArgoCD) string {
	if hasClient(cr) && cr.Spec.ArgoCDAgent.Agent.Client.EnableWebSocket != nil {
		return strconv.FormatBool(*cr.Spec.ArgoCDAgent.Agent.Client.EnableWebSocket)
	}
	return "false"
}

func getAgentEnableCompression(cr *argoproj.ArgoCD) string {
	if hasClient(cr) && cr.Spec.ArgoCDAgent.Agent.Client.EnableCompression != nil {
		return strconv.FormatBool(*cr.Spec.ArgoCDAgent.Agent.Client.EnableCompression)
	}
	return "false"
}

// Keep Alive Configuration
func getAgentKeepAliveInterval(cr *argoproj.ArgoCD) string {
	if hasClient(cr) && cr.Spec.ArgoCDAgent.Agent.Client.KeepAliveInterval != "" {
		return cr.Spec.ArgoCDAgent.Agent.Client.KeepAliveInterval
	}
	return "30s"
}

// Redis Configuration
func getAgentRedisAddress(cr *argoproj.ArgoCD) string {
	if hasRedis(cr) && cr.Spec.ArgoCDAgent.Agent.Redis.ServerAddress != "" {
		return cr.Spec.ArgoCDAgent.Agent.Redis.ServerAddress
	}
	return fmt.Sprintf("%s-%s:%d", cr.Name, "redis", common.ArgoCDDefaultRedisPort)
}

func has(cr *argoproj.ArgoCD) bool {
	return cr.Spec.ArgoCDAgent != nil && cr.Spec.ArgoCDAgent.Agent != nil
}

func hasClient(cr *argoproj.ArgoCD) bool {
	return cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Agent != nil &&
		cr.Spec.ArgoCDAgent.Agent.Client != nil
}

func hasTLS(cr *argoproj.ArgoCD) bool {
	return cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Agent != nil &&
		cr.Spec.ArgoCDAgent.Agent.TLS != nil
}

func hasRedis(cr *argoproj.ArgoCD) bool {
	return cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Agent != nil &&
		cr.Spec.ArgoCDAgent.Agent.Redis != nil
}
