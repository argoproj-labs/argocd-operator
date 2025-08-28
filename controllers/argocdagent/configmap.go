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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// Configuration keys for the Principal component ConfigMap
// These constants define the keys used in the ConfigMap data section
const (
	PrincipalAllowedNamespaces         = "principal.allowed-namespaces"
	PrincipalAuth                      = "principal.auth"
	PrincipalLogLevel                  = "principal.log.level"
	PrincipalMetricsPort               = "principal.metrics.port"
	PrincipalNamespace                 = "principal.namespace"
	PrincipalRedisAddress              = "principal.redis-server-address"
	PrincipalLogFormat                 = "principal.log.format"
	PrincipalJwtAllowGenerate          = "principal.jwt.allow-generate"
	PrincipalJWTKeyPath                = "principal.jwt.key-path"
	PrincipalJwtSecretName             = "principal.jwt.secret-name" // #nosec G101
	PrincipalListenHost                = "principal.listen.host"
	PrincipalListenPort                = "principal.listen.port"
	PrincipalNamespaceCreateEnable     = "principal.namespace-create.enable"
	PrincipalNamespaceCreatePattern    = "principal.namespace-create.pattern"
	PrincipalNamespaceCreateLabels     = "principal.namespace-create.labels"
	PrincipalResourceProxyCAPath       = "principal.resource-proxy.ca.path"
	PrincipalResourceProxyCASecretName = "principal.resource-proxy.ca.secret-name"
	PrincipalResourceProxySecretName   = "principal.resource-proxy.secret-name"
	PrincipalResourceProxyTLSCertPath  = "principal.resource-proxy.tls.cert-path"
	PrincipalResourceProxyTLSKeyPath   = "principal.resource-proxy.tls.key-path"
	PrincipalResourceProxyTLSCAPath    = "principal.resource-proxy.tls.ca-path"
	PrincipalTLSServerCertPath         = "principal.tls.server.cert-path"
	PrincipalTLSServerKeyPath          = "principal.tls.server.key-path"
	PrincipalTLSServerAllowGenerate    = "principal.tls.server.allow-generate"
	PrincipalTLSServerRootCAPath       = "principal.tls.server.root-ca-path"
	PrincipalTLSClientCertRequire      = "principal.tls.client-cert.require"
	PrincipalTLSClientCertMatchSubject = "principal.tls.client-cert.match-subject"
	PrincipalTLSSecretName             = "principal.tls.secret-name"                // #nosec G101
	PrincipalTLSServerRootCASecretName = "principal.tls.server.root-ca-secret-name" // #nosec G101
	PrincipalEnableWebSocket           = "principal.enable-websocket"
	PrincipalEnableResourceProxy       = "principal.enable-resource-proxy"
	PrincipalKeepAliveMinInterval      = "principal.keep-alive-min-interval"
	PrincipalRedisCompressionType      = "principal.redis-compression-type"
	PrincipalPprofPort                 = "principal.pprof-port"
	PrincipalRedisPassword             = "auth"

	// Environment variable names for Principal configuration
	// These constants define the environment variable names that can override default values
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
	EnvRedisPassword                            = "REDIS_PASSWORD"
	EnvArgoCDPrincipalImage                     = "ARGOCD_PRINCIPAL_IMAGE"
)

const cmSuffix = "-agent-principal-params"

// ReconcilePrincipalConfigMap manages the lifecycle of the Principal component's ConfigMap.
// It creates, updates, or deletes the ConfigMap based on the ArgoCD CR's Principal configuration.
// The ConfigMap contains all the configuration parameters needed by the Principal component.
func ReconcilePrincipalConfigMap(client client.Client, compName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme) error {
	cm := buildConfigMap(cr, compName)
	expectedData := buildData(cr)

	// Check if the ConfigMap already exists
	exists := true
	if err := client.Get(context.TODO(), types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, cm); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get existing principal configmap %s in namespace %s: %v", cm.Name, cr.Namespace, err)
		}
		exists = false
	}

	// If ConfigMap exists, handle updates or deletion
	if exists {
		if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
			argoutil.LogResourceDeletion(log, cm, "principal configmap is being deleted as principal is disabled")
			if err := client.Delete(context.TODO(), cm); err != nil {
				return fmt.Errorf("failed to delete principal configmap %s: %v", cm.Name, err)
			}
			return nil
		}

		if !reflect.DeepEqual(cm.Data, expectedData) {
			cm.Data = expectedData
			argoutil.LogResourceUpdate(log, cm, "principal configmap is being updated")
			if err := client.Update(context.TODO(), cm); err != nil {
				return fmt.Errorf("failed to update principal configmap %s: %v", cm.Name, err)
			}
		}
		return nil
	}

	// If ConfigMap doesn't exist and Principal is disabled, nothing to do
	if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, cm, scheme); err != nil {
		return fmt.Errorf("failed to set ArgoCD CR %s as owner for configmap %s: %v", cr.Name, cm.Name, err)
	}

	cm.Data = expectedData

	argoutil.LogResourceCreation(log, cm)
	if err := client.Create(context.TODO(), cm); err != nil {
		return fmt.Errorf("failed to create principal configmap %s: %v", cm.Name, err)
	}
	return nil
}

func buildConfigMap(cr *argoproj.ArgoCD, compName string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + cmSuffix,
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, compName),
		},
	}
}

func buildData(cr *argoproj.ArgoCD) map[string]string {
	expectedData := make(map[string]string)

	// Basic configuration
	expectedData[PrincipalNamespace] = cr.Namespace
	expectedData[PrincipalAllowedNamespaces] = getPrincipalAllowedNamespaces(cr)
	expectedData[PrincipalAuth] = getPrincipalAuth(cr)
	expectedData[PrincipalJwtAllowGenerate] = getPrincipalJwtAllowGenerate(cr)
	expectedData[PrincipalLogLevel] = getPrincipalLogLevel(cr)
	expectedData[PrincipalMetricsPort] = getPrincipalMetricsPort()
	expectedData[PrincipalRedisAddress] = getPrincipalRedisServerAddress()

	// Network and Server Configuration
	expectedData[PrincipalListenHost] = getPrincipalListenHost()
	expectedData[PrincipalListenPort] = getPrincipalListenPort()
	expectedData[PrincipalLogFormat] = getPrincipalLogFormat()

	// Namespace Management
	expectedData[PrincipalNamespaceCreateEnable] = getPrincipalNamespaceCreateEnable()
	expectedData[PrincipalNamespaceCreatePattern] = getPrincipalNamespaceCreatePattern()
	expectedData[PrincipalNamespaceCreateLabels] = getPrincipalNamespaceCreateLabels()

	// TLS Server Configuration
	expectedData[PrincipalTLSServerCertPath] = getPrincipalTLSServerCertPath()
	expectedData[PrincipalTLSServerKeyPath] = getPrincipalTLSServerKeyPath()
	expectedData[PrincipalTLSServerAllowGenerate] = getPrincipalTLSServerAllowGenerate()
	expectedData[PrincipalTLSServerRootCAPath] = getPrincipalTLSServerRootCAPath()

	// TLS Client Configuration
	expectedData[PrincipalTLSClientCertRequire] = getPrincipalTLSClientCertRequire()
	expectedData[PrincipalTLSClientCertMatchSubject] = getPrincipalTLSClientCertMatchSubject()

	// Resource Proxy Configuration
	expectedData[PrincipalResourceProxyTLSCertPath] = getPrincipalResourceProxyTLSCertPath()
	expectedData[PrincipalResourceProxyTLSKeyPath] = getPrincipalResourceProxyTLSKeyPath()
	expectedData[PrincipalResourceProxyTLSCAPath] = getPrincipalResourceProxyTLSCAPath()
	expectedData[PrincipalResourceProxyCAPath] = getPrincipalResourceProxyCaPath()
	expectedData[PrincipalResourceProxyCASecretName] = getPrincipalResourceProxyCaSecretName()
	expectedData[PrincipalResourceProxySecretName] = getPrincipalResourceProxySecretName()
	expectedData[PrincipalTLSSecretName] = getPrincipalTlsSecretName()
	expectedData[PrincipalTLSServerRootCASecretName] = getPrincipalTlsServerRootCASecretName()

	// JWT Configuration
	expectedData[PrincipalJWTKeyPath] = getPrincipalJWTKeyPath()
	expectedData[PrincipalJwtSecretName] = getPrincipalJwtSecretName()

	// Feature Configuration
	expectedData[PrincipalEnableWebSocket] = getPrincipalEnableWebSocket()
	expectedData[PrincipalEnableResourceProxy] = getPrincipalEnableResourceProxy()
	expectedData[PrincipalKeepAliveMinInterval] = getPrincipalKeepAliveMinInterval()

	// Redis Configuration
	expectedData[PrincipalRedisCompressionType] = getPrincipalRedisCompressionType()

	// Profiling Configuration
	expectedData[PrincipalPprofPort] = getPrincipalPprofPort()

	return expectedData
}

// Network and Server Configuration
func getPrincipalListenHost() string {
	return os.Getenv(EnvArgoCDPrincipalListenHost)
}

func getPrincipalListenPort() string {
	if value := os.Getenv(EnvArgoCDPrincipalListenPort); value != "" {
		return value
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

func getPrincipalLogFormat() string {
	if value := os.Getenv(EnvArgoCDPrincipalLogFormat); value != "" {
		return value
	}
	return "text"
}

// Metrics Configuration
func getPrincipalMetricsPort() string {
	if value := os.Getenv(EnvArgoCDPrincipalMetricsPort); value != "" {
		return value
	}
	return "8000"
}

func getPrincipalAllowedNamespaces(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil && cr.Spec.ArgoCDAgent.Principal != nil {
		return strings.Join(cr.Spec.ArgoCDAgent.Principal.AllowedNamespaces, ",")
	}
	return ""
}

func getPrincipalNamespaceCreateEnable() string {
	if value := os.Getenv(EnvArgoCDPrincipalNamespaceCreateEnable); value != "" {
		return value
	}
	return "false"
}

func getPrincipalNamespaceCreatePattern() string {
	return os.Getenv(EnvArgoCDPrincipalNamespaceCreatePattern)
}

func getPrincipalNamespaceCreateLabels() string {
	return os.Getenv(EnvArgoCDPrincipalNamespaceCreateLabels)
}

// TLS Server Configuration
func getPrincipalTLSServerCertPath() string {
	return os.Getenv(EnvArgoCDPrincipalTLSServerCertPath)
}

func getPrincipalTLSServerKeyPath() string {
	return os.Getenv(EnvArgoCDPrincipalTLSServerKeyPath)
}

func getPrincipalTLSServerAllowGenerate() string {
	if value := os.Getenv(EnvArgoCDPrincipalTLSServerAllowGenerate); value != "" {
		return value
	}
	return "false"
}

func getPrincipalTLSServerRootCAPath() string {
	return os.Getenv(EnvArgoCDPrincipalTLSServerRootCAPath)
}

// TLS Client Configuration
func getPrincipalTLSClientCertRequire() string {
	if value := os.Getenv(EnvArgoCDPrincipalTLSClientCertRequire); value != "" {
		return value
	}
	return "false"
}

func getPrincipalTLSClientCertMatchSubject() string {
	if value := os.Getenv(EnvArgoCDPrincipalTLSClientCertMatchSubject); value != "" {
		return value
	}
	return "false"
}

// Resource Proxy Configuration
func getPrincipalResourceProxyTLSCertPath() string {
	return os.Getenv(EnvArgoCDPrincipalResourceProxyTLSCertPath)
}

func getPrincipalResourceProxyTLSKeyPath() string {
	return os.Getenv(EnvArgoCDPrincipalResourceProxyTLSKeyPath)
}

func getPrincipalResourceProxyTLSCAPath() string {
	return os.Getenv(EnvArgoCDPrincipalResourceProxyTLSCAPath)
}

// JWT Configuration
func getPrincipalJWTKeyPath() string {
	return os.Getenv(EnvArgoCDPrincipalJWTKeyPath)
}

func getPrincipalJwtAllowGenerate(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil {
		return strconv.FormatBool(cr.Spec.ArgoCDAgent.Principal.JWTAllowGenerate)
	}
	return "false"
}

// Authentication Configuration
func getPrincipalAuth(cr *argoproj.ArgoCD) string {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil {
		return cr.Spec.ArgoCDAgent.Principal.Auth
	}
	return ""
}

// WebSocket Configuration
func getPrincipalEnableWebSocket() string {
	if value := os.Getenv(EnvArgoCDPrincipalEnableWebSocket); value != "" {
		return value
	}
	return "false"
}

// Resource Proxy Enable/Disable
func getPrincipalEnableResourceProxy() string {
	if value := os.Getenv(EnvArgoCDPrincipalEnableResourceProxy); value != "" {
		return value
	}
	return "true"
}

// Keep Alive Configuration
func getPrincipalKeepAliveMinInterval() string {
	if value := os.Getenv(EnvArgoCDPrincipalKeepAliveMinInterval); value != "" {
		return value
	}
	return "30s"
}

// Redis Configuration
func getPrincipalRedisServerAddress() string {
	if value := os.Getenv(EnvArgoCDPrincipalRedisServerAddress); value != "" {
		return value
	}
	return "argocd-redis:6379"
}

func getPrincipalRedisCompressionType() string {
	if value := os.Getenv(EnvArgoCDPrincipalRedisCompressionType); value != "" {
		return value
	}
	return "gzip"
}

// Profiling Configuration
func getPrincipalPprofPort() string {
	if value := os.Getenv(EnvArgoCDPrincipalPprofPort); value != "" {
		return value
	}
	return "0"
}

func getPrincipalJwtSecretName() string {
	if value := os.Getenv(EnvArgoCDPrincipalJwtSecretName); value != "" {
		return value
	}
	return "argocd-agent-jwt"
}

func getPrincipalResourceProxyCaPath() string {
	if value := os.Getenv(EnvArgoCDPrincipalResourceProxyCaPath); value != "" {
		return value
	}
	return ""
}

func getPrincipalResourceProxyCaSecretName() string {
	if value := os.Getenv(EnvArgoCDPrincipalResourceProxyCaSecretName); value != "" {
		return value
	}
	return "argocd-agent-ca"
}

func getPrincipalResourceProxySecretName() string {
	if value := os.Getenv(EnvArgoCDPrincipalResourceProxySecretName); value != "" {
		return value
	}
	return "argocd-agent-resource-proxy-tls"
}

func getPrincipalTlsSecretName() string {
	if value := os.Getenv(EnvArgoCDPrincipalTlsSecretName); value != "" {
		return value
	}
	return "argocd-agent-principal-tls"
}

func getPrincipalTlsServerRootCASecretName() string {
	if value := os.Getenv(EnvArgoCDPrincipalTlsServerRootCASecretName); value != "" {
		return value
	}
	return "argocd-agent-ca"
}
