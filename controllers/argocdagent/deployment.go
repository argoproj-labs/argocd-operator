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
	"os"
	"reflect"
	"strconv"
	"strings"

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
		if !hasPrincipal(cr) || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
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
	if !hasPrincipal(cr) || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
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
						ImagePullPolicy: argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy),
						Name:            generateAgentResourceName(cr.Name, compName),
						Env:             buildPrincipalContainerEnv(cr),
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
			Protocol:      corev1.ProtocolTCP,
		},
		{
			ContainerPort: 8000,
			Name:          "metrics",
			Protocol:      corev1.ProtocolTCP,
		},
		{
			ContainerPort: 6379,
			Name:          "redis",
			Protocol:      corev1.ProtocolTCP,
		},
		{
			ContainerPort: 9090,
			Name:          "resource-proxy",
			Protocol:      corev1.ProtocolTCP,
		},
		{
			ContainerPort: 8003,
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

func buildPrincipalImage(cr *argoproj.ArgoCD) string {
	// Check CR specification first
	if hasServer(cr) && cr.Spec.ArgoCDAgent.Principal.Server.Image != "" {
		return cr.Spec.ArgoCDAgent.Principal.Server.Image
	}

	// Value specified in the environment take precedence over the default
	if env := os.Getenv(EnvArgoCDPrincipalImage); env != "" {
		return env
	}

	// Use the default image and version if not specified in the CR or environment variable
	return common.ArgoCDAgentPrincipalDefaultImageName
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

	if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy, argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy)) {
		log.Info("deployment image pull policy is being updated")
		changed = true
		deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy)
	}

	if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Name, generateAgentResourceName(cr.Name, compName)) {
		log.Info("deployment container name is being updated")
		changed = true
		deployment.Spec.Template.Spec.Containers[0].Name = generateAgentResourceName(cr.Name, compName)
	}

	if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Env, buildPrincipalContainerEnv(cr)) {
		log.Info("deployment container env is being updated")
		changed = true
		deployment.Spec.Template.Spec.Containers[0].Env = buildPrincipalContainerEnv(cr)
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

func buildPrincipalContainerEnv(cr *argoproj.ArgoCD) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{
			Name:  EnvArgoCDPrincipalLogLevel,
			Value: getPrincipalLogLevel(cr),
		}, {
			Name:  EnvArgoCDPrincipalNamespace,
			Value: cr.Namespace,
		}, {
			Name:  EnvArgoCDPrincipalAllowedNamespaces,
			Value: getPrincipalAllowedNamespaces(cr),
		}, {
			Name:  EnvArgoCDPrincipalNamespaceCreateEnable,
			Value: getPrincipalNamespaceCreateEnable(cr),
		}, {
			Name:  EnvArgoCDPrincipalNamespaceCreatePattern,
			Value: getPrincipalNamespaceCreatePattern(cr),
		}, {
			Name:  EnvArgoCDPrincipalNamespaceCreateLabels,
			Value: getPrincipalNamespaceCreateLabels(cr),
		}, {
			Name:  EnvArgoCDPrincipalTLSServerAllowGenerate,
			Value: getPrincipalTLSServerAllowGenerate(cr),
		}, {
			Name:  EnvArgoCDPrincipalJWTAllowGenerate,
			Value: getPrincipalJWTAllowGenerate(cr),
		}, {
			Name:  EnvArgoCDPrincipalAuth,
			Value: getPrincipalAuth(cr),
		}, {
			Name:  EnvArgoCDPrincipalEnableResourceProxy,
			Value: "true",
		}, {
			Name:  EnvArgoCDPrincipalKeepAliveMinInterval,
			Value: getPrincipalKeepAliveMinInterval(cr),
		}, {
			Name:  EnvArgoCDPrincipalRedisServerAddress,
			Value: getPrincipalRedisServerAddress(cr),
		}, {
			Name:  EnvArgoCDPrincipalRedisCompressionType,
			Value: getPrincipalRedisCompressionType(cr),
		}, {
			Name:  EnvArgoCDPrincipalLogFormat,
			Value: getPrincipalLogFormat(cr),
		}, {
			Name:  EnvArgoCDPrincipalEnableWebSocket,
			Value: getPrincipalEnableWebSocket(cr),
		}, {
			Name:  EnvArgoCDPrincipalTLSSecretName,
			Value: getPrincipalTLSServerSecretName(cr),
		}, {
			Name:  EnvArgoCDPrincipalTLSServerRootCASecretName,
			Value: getPrincipalTlsServerRootCASecretName(cr),
		}, {
			Name:  EnvArgoCDPrincipalResourceProxySecretName,
			Value: getPrincipalResourceProxySecretName(cr),
		}, {
			Name:  EnvArgoCDPrincipalResourceProxyCaSecretName,
			Value: getPrincipalResourceProxyCaSecretName(cr),
		}, {
			Name:  EnvArgoCDPrincipalJwtSecretName,
			Value: getPrincipalJWTSecretName(cr),
		}, {
			Name: EnvRedisPassword,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: PrincipalRedisPasswordKey,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: fmt.Sprintf("%s-%s", cr.Name, PrincipalRedisSecretnameSuffix),
					},
					Optional: ptr.To(true),
				},
			},
		},
	}

	// Add custom environment variables if specified in the CR
	if hasServer(cr) && cr.Spec.ArgoCDAgent.Principal.Server.Env != nil {
		env = append(env, cr.Spec.ArgoCDAgent.Principal.Server.Env...)
	}

	return env
}

// These constants are environment variables that correspond to the environment variables
// used to configure Argo CD agent, and should match the names exactly from the agent
const (
	EnvArgoCDPrincipalLogLevel                  = "ARGOCD_PRINCIPAL_LOG_LEVEL"
	EnvArgoCDPrincipalLogFormat                 = "ARGOCD_PRINCIPAL_LOG_FORMAT"
	EnvArgoCDPrincipalNamespace                 = "ARGOCD_PRINCIPAL_NAMESPACE"
	EnvArgoCDPrincipalAllowedNamespaces         = "ARGOCD_PRINCIPAL_ALLOWED_NAMESPACES"
	EnvArgoCDPrincipalNamespaceCreateEnable     = "ARGOCD_PRINCIPAL_NAMESPACE_CREATE_ENABLE"
	EnvArgoCDPrincipalNamespaceCreatePattern    = "ARGOCD_PRINCIPAL_NAMESPACE_CREATE_PATTERN"
	EnvArgoCDPrincipalNamespaceCreateLabels     = "ARGOCD_PRINCIPAL_NAMESPACE_CREATE_LABELS"
	EnvArgoCDPrincipalTLSServerAllowGenerate    = "ARGOCD_PRINCIPAL_TLS_SERVER_ALLOW_GENERATE"
	EnvArgoCDPrincipalJWTAllowGenerate          = "ARGOCD_PRINCIPAL_JWT_ALLOW_GENERATE"
	EnvArgoCDPrincipalAuth                      = "ARGOCD_PRINCIPAL_AUTH"
	EnvArgoCDPrincipalEnableWebSocket           = "ARGOCD_PRINCIPAL_ENABLE_WEBSOCKET"
	EnvArgoCDPrincipalEnableResourceProxy       = "ARGOCD_PRINCIPAL_ENABLE_RESOURCE_PROXY"
	EnvArgoCDPrincipalKeepAliveMinInterval      = "ARGOCD_PRINCIPAL_KEEP_ALIVE_MIN_INTERVAL"
	EnvArgoCDPrincipalRedisServerAddress        = "ARGOCD_PRINCIPAL_REDIS_SERVER_ADDRESS"
	EnvArgoCDPrincipalRedisCompressionType      = "ARGOCD_PRINCIPAL_REDIS_COMPRESSION_TYPE"
	EnvArgoCDPrincipalTLSSecretName             = "ARGOCD_PRINCIPAL_TLS_SECRET_NAME"
	EnvArgoCDPrincipalTLSServerRootCASecretName = "ARGOCD_PRINCIPAL_TLS_SERVER_ROOT_CA_SECRET_NAME"
	EnvArgoCDPrincipalResourceProxySecretName   = "ARGOCD_PRINCIPAL_RESOURCE_PROXY_SECRET_NAME"
	EnvArgoCDPrincipalResourceProxyCaSecretName = "ARGOCD_PRINCIPAL_RESOURCE_PROXY_CA_SECRET_NAME"
	EnvArgoCDPrincipalJwtSecretName             = "ARGOCD_PRINCIPAL_JWT_SECRET_NAME"
	EnvArgoCDPrincipalImage                     = "ARGOCD_PRINCIPAL_IMAGE"
	EnvRedisPassword                            = "REDIS_PASSWORD"
	PrincipalRedisPasswordKey                   = "admin.password"
	PrincipalRedisSecretnameSuffix              = "redis-initial-password" // #nosec G101
)

// Logging Configuration
func getPrincipalLogLevel(cr *argoproj.ArgoCD) string {
	if hasServer(cr) && cr.Spec.ArgoCDAgent.Principal.Server.LogLevel != "" {
		return cr.Spec.ArgoCDAgent.Principal.Server.LogLevel
	}
	return "info"
}

func getPrincipalLogFormat(cr *argoproj.ArgoCD) string {
	if hasServer(cr) && cr.Spec.ArgoCDAgent.Principal.Server.LogFormat != "" {
		return cr.Spec.ArgoCDAgent.Principal.Server.LogFormat
	}
	return "text"
}

func getPrincipalAllowedNamespaces(cr *argoproj.ArgoCD) string {
	if hasNamespace(cr) &&
		cr.Spec.ArgoCDAgent.Principal.Namespace.AllowedNamespaces != nil &&
		len(cr.Spec.ArgoCDAgent.Principal.Namespace.AllowedNamespaces) > 0 {
		return strings.Join(cr.Spec.ArgoCDAgent.Principal.Namespace.AllowedNamespaces, ",")
	}
	return ""
}

func getPrincipalNamespaceCreateEnable(cr *argoproj.ArgoCD) string {
	if hasNamespace(cr) && cr.Spec.ArgoCDAgent.Principal.Namespace.EnableNamespaceCreate != nil {
		return strconv.FormatBool(*cr.Spec.ArgoCDAgent.Principal.Namespace.EnableNamespaceCreate)
	}
	return "false"
}

func getPrincipalNamespaceCreatePattern(cr *argoproj.ArgoCD) string {
	if hasNamespace(cr) && cr.Spec.ArgoCDAgent.Principal.Namespace.NamespaceCreatePattern != "" {
		return cr.Spec.ArgoCDAgent.Principal.Namespace.NamespaceCreatePattern
	}
	return ""
}

func getPrincipalNamespaceCreateLabels(cr *argoproj.ArgoCD) string {
	if hasNamespace(cr) && len(cr.Spec.ArgoCDAgent.Principal.Namespace.NamespaceCreateLabels) > 0 {
		return strings.Join(cr.Spec.ArgoCDAgent.Principal.Namespace.NamespaceCreateLabels, ",")
	}
	return ""
}

func getPrincipalTLSServerAllowGenerate(cr *argoproj.ArgoCD) string {
	if hasTLS(cr) && cr.Spec.ArgoCDAgent.Principal.TLS.InsecureGenerate != nil {
		return strconv.FormatBool(*cr.Spec.ArgoCDAgent.Principal.TLS.InsecureGenerate)
	}
	return "false"
}

// JWT Configuration
func getPrincipalJWTAllowGenerate(cr *argoproj.ArgoCD) string {
	if hasJWT(cr) && cr.Spec.ArgoCDAgent.Principal.JWT.InsecureGenerate != nil {
		return strconv.FormatBool(*cr.Spec.ArgoCDAgent.Principal.JWT.InsecureGenerate)
	}
	return "false"
}

// Authentication Configuration
func getPrincipalAuth(cr *argoproj.ArgoCD) string {
	if hasServer(cr) && cr.Spec.ArgoCDAgent.Principal.Server.Auth != "" {
		return cr.Spec.ArgoCDAgent.Principal.Server.Auth
	}
	return "mtls:CN=([^,]+)"
}

// WebSocket Configuration
func getPrincipalEnableWebSocket(cr *argoproj.ArgoCD) string {
	if hasServer(cr) && cr.Spec.ArgoCDAgent.Principal.Server.EnableWebSocket != nil {
		return strconv.FormatBool(*cr.Spec.ArgoCDAgent.Principal.Server.EnableWebSocket)
	}
	return "false"
}

// Keep Alive Configuration
func getPrincipalKeepAliveMinInterval(cr *argoproj.ArgoCD) string {
	if hasServer(cr) && cr.Spec.ArgoCDAgent.Principal.Server.KeepAliveMinInterval != "" {
		return cr.Spec.ArgoCDAgent.Principal.Server.KeepAliveMinInterval
	}
	return "30s"
}

// Redis Configuration
func getPrincipalRedisServerAddress(cr *argoproj.ArgoCD) string {
	if hasRedis(cr) && cr.Spec.ArgoCDAgent.Principal.Redis.ServerAddress != "" {
		return cr.Spec.ArgoCDAgent.Principal.Redis.ServerAddress
	}
	return fmt.Sprintf("%s-%s:%d", cr.Name, "redis", common.ArgoCDDefaultRedisPort)
}

func getPrincipalRedisCompressionType(cr *argoproj.ArgoCD) string {
	if hasRedis(cr) && cr.Spec.ArgoCDAgent.Principal.Redis.CompressionType != "" {
		return cr.Spec.ArgoCDAgent.Principal.Redis.CompressionType
	}
	return "gzip"
}

func getPrincipalJWTSecretName(cr *argoproj.ArgoCD) string {
	if hasJWT(cr) && cr.Spec.ArgoCDAgent.Principal.JWT.SecretName != "" {
		return cr.Spec.ArgoCDAgent.Principal.JWT.SecretName
	}
	return "argocd-agent-jwt"
}

func getPrincipalResourceProxyCaSecretName(cr *argoproj.ArgoCD) string {
	if hasResourceProxy(cr) && cr.Spec.ArgoCDAgent.Principal.ResourceProxy.CASecretName != "" {
		return cr.Spec.ArgoCDAgent.Principal.ResourceProxy.CASecretName
	}
	return "argocd-agent-ca"
}

func getPrincipalResourceProxySecretName(cr *argoproj.ArgoCD) string {
	if hasResourceProxy(cr) && cr.Spec.ArgoCDAgent.Principal.ResourceProxy.SecretName != "" {
		return cr.Spec.ArgoCDAgent.Principal.ResourceProxy.SecretName
	}
	return "argocd-agent-resource-proxy-tls"
}

func getPrincipalTLSServerSecretName(cr *argoproj.ArgoCD) string {
	if hasTLS(cr) && cr.Spec.ArgoCDAgent.Principal.TLS.SecretName != "" {
		return cr.Spec.ArgoCDAgent.Principal.TLS.SecretName
	}
	return "argocd-agent-principal-tls"
}

func getPrincipalTlsServerRootCASecretName(cr *argoproj.ArgoCD) string {
	if hasTLS(cr) && cr.Spec.ArgoCDAgent.Principal.TLS.RootCASecretName != "" {
		return cr.Spec.ArgoCDAgent.Principal.TLS.RootCASecretName
	}
	return "argocd-agent-ca"
}

func hasPrincipal(cr *argoproj.ArgoCD) bool {
	return cr.Spec.ArgoCDAgent != nil && cr.Spec.ArgoCDAgent.Principal != nil
}

func hasServer(cr *argoproj.ArgoCD) bool {
	return cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.Server != nil
}

func hasNamespace(cr *argoproj.ArgoCD) bool {
	return cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.Namespace != nil
}

func hasTLS(cr *argoproj.ArgoCD) bool {
	return cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS != nil
}

func hasResourceProxy(cr *argoproj.ArgoCD) bool {
	return cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy != nil
}

func hasJWT(cr *argoproj.ArgoCD) bool {
	return cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.JWT != nil
}

func hasRedis(cr *argoproj.ArgoCD) bool {
	return cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.Redis != nil
}
