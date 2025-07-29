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
		fmt.Println("------------------------------------------------------------------------------")
		fmt.Println("deployment.Spec.Template.Spec.Containers[0].Ports", deployment.Spec.Template.Spec.Containers[0].Ports)
		fmt.Println("buildPorts(compName)", buildPorts(compName))
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
			Name:  EnvArgoCDPrincipalListenHost,
			Value: getPrincipalListenHost(cr),
		}, {
			Name:  EnvArgoCDPrincipalListenPort,
			Value: getPrincipalListenPort(cr),
		}, {
			Name:  EnvArgoCDPrincipalLogLevel,
			Value: getPrincipalLogLevel(cr),
		}, {
			Name:  EnvArgoCDPrincipalMetricsPort,
			Value: getPrincipalMetricsPort(cr),
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
			Name:  EnvArgoCDPrincipalTLSServerCertPath,
			Value: getPrincipalTLSServerCertPath(cr),
		}, {
			Name:  EnvArgoCDPrincipalTLSServerKeyPath,
			Value: getPrincipalTLSServerKeyPath(cr),
		}, {
			Name:  EnvArgoCDPrincipalTLSServerAllowGenerate,
			Value: getPrincipalTLSServerAllowGenerate(cr),
		}, {
			Name:  EnvArgoCDPrincipalTLSClientCertRequire,
			Value: getPrincipalTLSClientCertRequire(cr),
		}, {
			Name:  EnvArgoCDPrincipalTLSServerRootCAPath,
			Value: getPrincipalTLSServerRootCAPath(cr),
		}, {
			Name:  EnvArgoCDPrincipalTLSClientCertMatchSubject,
			Value: getPrincipalTLSClientCertMatchSubject(cr),
		}, {
			Name:  EnvArgoCDPrincipalJWTAllowGenerate,
			Value: getPrincipalJwtAllowGenerate(cr),
		}, {
			Name:  EnvArgoCDPrincipalJWTKeyPath,
			Value: getPrincipalJWTKeyPath(cr),
		}, {
			Name:  EnvArgoCDPrincipalAuth,
			Value: getPrincipalAuth(cr),
		}, {
			Name:  EnvArgoCDPrincipalEnableResourceProxy,
			Value: getPrincipalEnableResourceProxy(cr),
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
			Name:  EnvArgoCDPrincipalPprofPort,
			Value: getPrincipalPprofPort(cr),
		}, {
			Name:  EnvArgoCDPrincipalLogFormat,
			Value: getPrincipalLogFormat(cr),
		}, {
			Name:  EnvArgoCDPrincipalResourceProxyTLSCertPath,
			Value: getPrincipalResourceProxyTLSCertPath(cr),
		}, {
			Name:  EnvArgoCDPrincipalResourceProxyTLSKeyPath,
			Value: getPrincipalResourceProxyTLSKeyPath(cr),
		}, {
			Name:  EnvArgoCDPrincipalResourceProxyTLSCAPath,
			Value: getPrincipalResourceProxyTLSCAPath(cr),
		}, {
			Name:  EnvArgoCDPrincipalEnableWebSocket,
			Value: getPrincipalEnableWebSocket(cr),
		}, {
			Name:  EnvArgoCDPrincipalTlsSecretName,
			Value: getPrincipalTlsSecretName(cr),
		}, {
			Name:  EnvArgoCDPrincipalTlsServerRootCASecretName,
			Value: getPrincipalTlsServerRootCASecretName(cr),
		}, {
			Name:  EnvArgoCDPrincipalResourceProxySecretName,
			Value: getPrincipalResourceProxySecretName(cr),
		}, {
			Name:  EnvArgoCDPrincipalResourceProxyCaPath,
			Value: getPrincipalResourceProxyCaPath(cr),
		}, {
			Name:  EnvArgoCDPrincipalResourceProxyCaSecretName,
			Value: getPrincipalResourceProxyCaSecretName(cr),
		}, {
			Name:  EnvArgoCDPrincipalJwtSecretName,
			Value: getPrincipalJwtSecretName(cr),
		}, {
			Name:  EnvArgoCDPrincipalHealthzPort,
			Value: getPrincipalHealthzPort(cr),
		},
	}

	return env
}

const (
	EnvArgoCDPrincipalListenHost                = "ARGOCD_PRINCIPAL_LISTEN_HOST"
	EnvArgoCDPrincipalListenPort                = "ARGOCD_PRINCIPAL_LISTEN_PORT"
	EnvArgoCDPrincipalLogLevel                  = "ARGOCD_PRINCIPAL_LOG_LEVEL"
	EnvArgoCDPrincipalLogFormat                 = "ARGOCD_PRINCIPAL_LOG_FORMAT"
	EnvArgoCDPrincipalMetricsPort               = "ARGOCD_PRINCIPAL_METRICS_PORT"
	EnvArgoCDPrincipalNamespace                 = "ARGOCD_PRINCIPAL_NAMESPACE"
	EnvArgoCDPrincipalAllowedNamespaces         = "ARGOCD_PRINCIPAL_ALLOWED_NAMESPACES"
	EnvArgoCDPrincipalNamespaceCreateEnable     = "ARGOCD_PRINCIPAL_NAMESPACE_CREATE_ENABLE"
	EnvArgoCDPrincipalNamespaceCreatePattern    = "ARGOCD_PRINCIPAL_NAMESPACE_CREATE_PATTERN"
	EnvArgoCDPrincipalNamespaceCreateLabels     = "ARGOCD_PRINCIPAL_NAMESPACE_CREATE_LABELS"
	EnvArgoCDPrincipalTLSServerCertPath         = "ARGOCD_PRINCIPAL_TLS_SERVER_CERT_PATH"
	EnvArgoCDPrincipalTLSServerKeyPath          = "ARGOCD_PRINCIPAL_TLS_SERVER_KEY_PATH"
	EnvArgoCDPrincipalTLSServerAllowGenerate    = "ARGOCD_PRINCIPAL_TLS_SERVER_ALLOW_GENERATE"
	EnvArgoCDPrincipalTLSServerRootCAPath       = "ARGOCD_PRINCIPAL_TLS_SERVER_ROOT_CA_PATH"
	EnvArgoCDPrincipalTLSClientCertRequire      = "ARGOCD_PRINCIPAL_TLS_CLIENT_CERT_REQUIRE"
	EnvArgoCDPrincipalTLSClientCertMatchSubject = "ARGOCD_PRINCIPAL_TLS_CLIENT_CERT_MATCH_SUBJECT"
	EnvArgoCDPrincipalResourceProxyTLSCertPath  = "ARGOCD_PRINCIPAL_RESOURCE_PROXY_TLS_CERT_PATH"
	EnvArgoCDPrincipalResourceProxyTLSKeyPath   = "ARGOCD_PRINCIPAL_RESOURCE_PROXY_TLS_KEY_PATH"
	EnvArgoCDPrincipalResourceProxyTLSCAPath    = "ARGOCD_PRINCIPAL_RESOURCE_PROXY_TLS_CA_PATH"
	EnvArgoCDPrincipalJWTKeyPath                = "ARGOCD_PRINCIPAL_JWT_KEY_PATH"
	EnvArgoCDPrincipalJWTAllowGenerate          = "ARGOCD_PRINCIPAL_JWT_ALLOW_GENERATE"
	EnvArgoCDPrincipalAuth                      = "ARGOCD_PRINCIPAL_AUTH"
	EnvArgoCDPrincipalEnableWebSocket           = "ARGOCD_PRINCIPAL_ENABLE_WEBSOCKET"
	EnvArgoCDPrincipalEnableResourceProxy       = "ARGOCD_PRINCIPAL_ENABLE_RESOURCE_PROXY"
	EnvArgoCDPrincipalKeepAliveMinInterval      = "ARGOCD_PRINCIPAL_KEEP_ALIVE_MIN_INTERVAL"
	EnvArgoCDPrincipalRedisServerAddress        = "ARGOCD_PRINCIPAL_REDIS_SERVER_ADDRESS"
	EnvArgoCDPrincipalRedisCompressionType      = "ARGOCD_PRINCIPAL_REDIS_COMPRESSION_TYPE"
	EnvArgoCDPrincipalPprofPort                 = "ARGOCD_PRINCIPAL_PPROF_PORT"
	EnvArgoCDPrincipalTlsSecretName             = "ARGOCD_PRINCIPAL_TLS_SECRET_NAME"
	EnvArgoCDPrincipalTlsServerRootCASecretName = "ARGOCD_PRINCIPAL_TLS_SERVER_ROOT_CA_SECRET_NAME"
	EnvArgoCDPrincipalResourceProxySecretName   = "ARGOCD_PRINCIPAL_RESOURCE_PROXY_SECRET_NAME"
	EnvArgoCDPrincipalResourceProxyCaSecretName = "ARGOCD_PRINCIPAL_RESOURCE_PROXY_CA_SECRET_NAME"
	EnvArgoCDPrincipalResourceProxyCaPath       = "ARGOCD_PRINCIPAL_RESOURCE_PROXY_CA_PATH"
	EnvArgoCDPrincipalJwtSecretName             = "ARGOCD_PRINCIPAL_JWT_SECRET_NAME"
	EnvArgoCDPrincipalHealthzPort               = "ARGOCD_PRINCIPAL_HEALTH_CHECK_PORT"
)

// Network and Server Configuration
func getPrincipalListenHost(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.ListenHost != "" {
		return cr.Spec.ArgoCDAgent.Principal.ListenHost
	}
	return ""
}

func getPrincipalListenPort(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.ListenPort != 0 {
		return strconv.Itoa(cr.Spec.ArgoCDAgent.Principal.ListenPort)
	}
	return "8443"
}

// Logging Configuration
func getPrincipalLogLevel(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.LogLevel != "" {
		return cr.Spec.ArgoCDAgent.Principal.LogLevel
	}
	return "info"
}

func getPrincipalLogFormat(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.LogFormat != "" {
		return cr.Spec.ArgoCDAgent.Principal.LogFormat
	}
	return "text"
}

// Metrics Configuration
func getPrincipalMetricsPort(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.MetricsPort != 0 {
		return strconv.Itoa(cr.Spec.ArgoCDAgent.Principal.MetricsPort)
	}
	return "8000"
}

func getPrincipalAllowedNamespaces(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.Namespace != nil &&
		cr.Spec.ArgoCDAgent.Principal.Namespace.AllowedNamespaces != nil &&
		len(cr.Spec.ArgoCDAgent.Principal.Namespace.AllowedNamespaces) > 0 {
		return strings.Join(cr.Spec.ArgoCDAgent.Principal.Namespace.AllowedNamespaces, ",")
	}
	return ""
}

func getPrincipalNamespaceCreateEnable(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.Namespace != nil &&
		cr.Spec.ArgoCDAgent.Principal.Namespace.CreateNamespace != nil {
		return strconv.FormatBool(*cr.Spec.ArgoCDAgent.Principal.Namespace.CreateNamespace)
	}
	return "false"
}

func getPrincipalNamespaceCreatePattern(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.Namespace != nil &&
		cr.Spec.ArgoCDAgent.Principal.Namespace.NamespaceCreatePattern != "" {
		return cr.Spec.ArgoCDAgent.Principal.Namespace.NamespaceCreatePattern
	}
	return ""
}

func getPrincipalNamespaceCreateLabels(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.Namespace != nil &&
		len(cr.Spec.ArgoCDAgent.Principal.Namespace.NamespaceCreateLabels) > 0 {
		return strings.Join(cr.Spec.ArgoCDAgent.Principal.Namespace.NamespaceCreateLabels, ",")
	}
	return ""
}

// TLS Server Configuration
func getPrincipalTLSServerCertPath(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS.Server != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS.Server.CertPath != "" {
		return cr.Spec.ArgoCDAgent.Principal.TLS.Server.CertPath
	}
	return ""
}

func getPrincipalTLSServerKeyPath(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS.Server != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS.Server.KeyPath != "" {
		return cr.Spec.ArgoCDAgent.Principal.TLS.Server.KeyPath
	}
	return ""
}

func getPrincipalTLSServerAllowGenerate(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS.Server != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS.Server.AllowGenerate != nil {
		return strconv.FormatBool(*cr.Spec.ArgoCDAgent.Principal.TLS.Server.AllowGenerate)
	}
	return "false"
}

func getPrincipalTLSServerRootCAPath(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS.Server != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS.Server.RootCAPath != "" {
		return cr.Spec.ArgoCDAgent.Principal.TLS.Server.RootCAPath
	}
	return ""
}

// TLS Client Configuration
func getPrincipalTLSClientCertRequire(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS.Client != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS.Client.Require != nil {
		return strconv.FormatBool(*cr.Spec.ArgoCDAgent.Principal.TLS.Client.Require)
	}
	return "false"
}

func getPrincipalTLSClientCertMatchSubject(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS.Client != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS.Client.MatchSubject != nil {
		return strconv.FormatBool(*cr.Spec.ArgoCDAgent.Principal.TLS.Client.MatchSubject)
	}
	return "false"
}

// Resource Proxy Configuration
func getPrincipalResourceProxyTLSCertPath(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy.TLS != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy.TLS.CertPath != "" {
		return cr.Spec.ArgoCDAgent.Principal.ResourceProxy.TLS.CertPath
	}
	return ""
}

func getPrincipalResourceProxyTLSKeyPath(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy.TLS != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy.TLS.KeyPath != "" {
		return cr.Spec.ArgoCDAgent.Principal.ResourceProxy.TLS.KeyPath
	}
	return ""
}

func getPrincipalResourceProxyTLSCAPath(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy.TLS != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy.TLS.CAPath != "" {
		return cr.Spec.ArgoCDAgent.Principal.ResourceProxy.TLS.CAPath
	}
	return ""
}

// JWT Configuration
func getPrincipalJWTKeyPath(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.JWT != nil &&
		cr.Spec.ArgoCDAgent.Principal.JWT.KeyPath != "" {
		return cr.Spec.ArgoCDAgent.Principal.JWT.KeyPath
	}
	return ""
}

func getPrincipalJwtAllowGenerate(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.JWT != nil &&
		cr.Spec.ArgoCDAgent.Principal.JWT.AllowGenerate != nil {
		return strconv.FormatBool(*cr.Spec.ArgoCDAgent.Principal.JWT.AllowGenerate)
	}
	return "false"
}

// Authentication Configuration
func getPrincipalAuth(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.Auth != "" {
		return cr.Spec.ArgoCDAgent.Principal.Auth
	}
	return "mtls:CN=([^,]+)"
}

// WebSocket Configuration
func getPrincipalEnableWebSocket(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.EnableWebSocket != nil {
		return strconv.FormatBool(*cr.Spec.ArgoCDAgent.Principal.EnableWebSocket)
	}
	return "false"
}

// Resource Proxy Enable/Disable
func getPrincipalEnableResourceProxy(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy.Enable != nil {
		return strconv.FormatBool(*cr.Spec.ArgoCDAgent.Principal.ResourceProxy.Enable)
	}
	return "false"
}

// Keep Alive Configuration
func getPrincipalKeepAliveMinInterval(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.KeepAliveMinInterval != "" {
		return cr.Spec.ArgoCDAgent.Principal.KeepAliveMinInterval
	}
	return "30s"
}

// Redis Configuration
func getPrincipalRedisServerAddress(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.Redis != nil &&
		cr.Spec.ArgoCDAgent.Principal.Redis.ServerAddress != "" {
		return cr.Spec.ArgoCDAgent.Principal.Redis.ServerAddress
	}
	return "argocd-redis:6379"
}

func getPrincipalRedisCompressionType(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.Redis != nil &&
		cr.Spec.ArgoCDAgent.Principal.Redis.CompressionType != "" {
		return cr.Spec.ArgoCDAgent.Principal.Redis.CompressionType
	}
	return "gzip"
}

// Profiling Configuration
func getPrincipalPprofPort(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.PprofPort != 0 {
		return strconv.Itoa(cr.Spec.ArgoCDAgent.Principal.PprofPort)
	}
	return "0"
}

func getPrincipalJwtSecretName(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.JWT != nil &&
		cr.Spec.ArgoCDAgent.Principal.JWT.SecretName != "" {
		return cr.Spec.ArgoCDAgent.Principal.JWT.SecretName
	}
	return "argocd-agent-jwt"
}

func getPrincipalResourceProxyCaPath(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy.CA != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy.CA.CAPath != "" {
		return cr.Spec.ArgoCDAgent.Principal.ResourceProxy.CA.CAPath
	}
	return ""
}

func getPrincipalResourceProxyCaSecretName(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy.CA != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy.CA.SecretName != "" {
		return cr.Spec.ArgoCDAgent.Principal.ResourceProxy.CA.SecretName
	}
	return "argocd-agent-ca"
}

func getPrincipalResourceProxySecretName(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy != nil &&
		cr.Spec.ArgoCDAgent.Principal.ResourceProxy.SecretName != "" {
		return cr.Spec.ArgoCDAgent.Principal.ResourceProxy.SecretName
	}
	return "argocd-agent-resource-proxy-tls"
}

func getPrincipalTlsSecretName(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS.SecretName != "" {
		return cr.Spec.ArgoCDAgent.Principal.TLS.SecretName
	}
	return "argocd-agent-principal-tls"
}

func getPrincipalTlsServerRootCASecretName(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS.Server != nil &&
		cr.Spec.ArgoCDAgent.Principal.TLS.Server.RootCASecretName != "" {
		return cr.Spec.ArgoCDAgent.Principal.TLS.Server.RootCASecretName
	}
	return "argocd-agent-ca"
}

func getPrincipalHealthzPort(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.HealthzPort != 0 {
		return strconv.Itoa(cr.Spec.ArgoCDAgent.Principal.HealthzPort)
	}
	return "8003"
}
