// Copyright 2019 ArgoCD Operator Developers
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

package argoproj

const (
	// ArgoCDAppName is the application name for labels.
	ArgoCDAppName = "argocd"

	// ArgoCDCASuffix is the name suffix for ArgoCD CA resources.
	ArgoCDCASuffix = "ca"

	// ArgoCDConfigMapName is the upstream hard-coded ArgoCD ConfigMap name.
	ArgoCDConfigMapName = "argocd-cm"

	// ArgoCDExportName is the export name for labels.
	ArgoCDExportName = "argocd.export"

	// ArgoCDGrafanaConfigMapSuffix is the default suffix for the Grafana configuration ConfigMap.
	ArgoCDGrafanaConfigMapSuffix = "grafana-config"

	// ArgoCDGrafanaDashboardConfigMapSuffix is the default suffix for the Grafana dashboards ConfigMap.
	ArgoCDGrafanaDashboardConfigMapSuffix = "grafana-dashboards"

	// ArgoCDDefaultArgoImage is the ArgoCD container image to use when not specified.
	ArgoCDDefaultArgoImage = "argoproj/argocd"

	// ArgoCDDefaultArgoServerOperationProcessors is the number of ArgoCD Server Operation Processors to use when not specified.
	ArgoCDDefaultArgoServerOperationProcessors = int32(10)

	// ArgoCDDefaultArgoServerStatusProcessors is the number of ArgoCD Server Status Processors to use when not specified.
	ArgoCDDefaultArgoServerStatusProcessors = int32(20)

	// ArgoCDDefaultArgoVersion is the ArgoCD container image tag to use when not specified.
	ArgoCDDefaultArgoVersion = "v1.4.1"

	// ArgoCDDefaultDexImage is the Dex container image to use when not specified.
	ArgoCDDefaultDexImage = "quay.io/dexidp/dex"

	// ArgoCDDefaultDexOAuthRedirectPath is the default path to use for the OAuth Redirect URI.
	ArgoCDDefaultDexOAuthRedirectPath = "/api/dex/callback"

	// ArgoCDDefaultDexServiceAccountName is the default Service Account name for the Dex server.
	ArgoCDDefaultDexServiceAccountName = "argocd-dex-server"

	// ArgoCDDefaultDexVersion is the Dex container image tag to use when not specified.
	ArgoCDDefaultDexVersion = "v2.14.0"

	// ArgoCDDefaultExportLocalCapicity is the default capacity to use for local export.
	ArgoCDDefaultExportLocalCapicity = "2Gi"

	// ArgoCDDefaultGrafanaAdminUsername is the Grafana admin username to use when not specified.
	ArgoCDDefaultGrafanaAdminUsername = "admin"

	// ArgoCDDefaultGrafanaAdminPasswordLength is the length of the generated default Grafana admin password.
	ArgoCDDefaultGrafanaAdminPasswordLength = 32

	// ArgoCDDefaultGrafanaAdminPasswordNumDigits is the number of digits to use for the generated default Grafana admin password.
	ArgoCDDefaultGrafanaAdminPasswordNumDigits = 5

	// ArgoCDDefaultGrafanaAdminPasswordNumSymbols is the number of symbols to use for the generated default Grafana admin password.
	ArgoCDDefaultGrafanaAdminPasswordNumSymbols = 5

	// ArgoCDDefaultGrafanaImage is the Grafana container image to use when not specified.
	ArgoCDDefaultGrafanaImage = "grafana/grafana"

	// ArgoCDDefaultGrafanaReplicas is the default Grafana replica count.
	ArgoCDDefaultGrafanaReplicas = int32(1)

	// ArgoCDDefaultGrafanaSecretKeyLength is the length of the generated default Grafana secret key.
	ArgoCDDefaultGrafanaSecretKeyLength = 20

	// ArgoCDDefaultGrafanaSecretKeyNumDigits is the number of digits to use for the generated default Grafana secret key.
	ArgoCDDefaultGrafanaSecretKeyNumDigits = 5

	// ArgoCDDefaultGrafanaSecretKeyNumSymbols is the number of symbols to use for the generated default Grafana secret key.
	ArgoCDDefaultGrafanaSecretKeyNumSymbols = 0

	// ArgoCDDefaultGrafanaConfigPath is the default Grafana configuration directory when not specified.
	ArgoCDDefaultGrafanaConfigPath = "/var/lib/grafana"

	// ArgoCDDefaultGrafanaVersion is the Grafana container image tag to use when not specified.
	ArgoCDDefaultGrafanaVersion = "6.5.1"

	// ArgoCDDefaultIngressPath is the path to use for the Ingress when not specified.
	ArgoCDDefaultIngressPath = "/"

	// ArgoCDDefaultPrometheusReplicas is the default Prometheus replica count.
	ArgoCDDefaultPrometheusReplicas = int32(1)

	// ArgoCDDefaultRedisImage is the Redis container image to use when not specified.
	ArgoCDDefaultRedisImage = "redis"

	// ArgoCDDefaultRedisVersion is the Redis container image tag to use when not specified.
	ArgoCDDefaultRedisVersion = "5.0.3"

	// ArgoCDKnownHostsConfigMapName is the upstream hard-coded SSH known hosts data ConfigMap name.
	ArgoCDKnownHostsConfigMapName = "argocd-ssh-known-hosts-cm"

	// ArgoCDKeyComponent is the resource component key for labels.
	ArgoCDKeyComponent = "app.kubernetes.io/component"

	// ArgoCDKeyDexOAuthRedirectURI is the key for the OAuth Redirect URI annotation.
	ArgoCDKeyDexOAuthRedirectURI = "serviceaccounts.openshift.io/oauth-redirecturi.argocd"

	// ArgoCDKeyDexConfig is the key for dex configuration.
	ArgoCDKeyDexConfig = "dex.config"

	// ArgoCDKeyGrafanaAdminUsername is the admin username key for labels.
	ArgoCDKeyGrafanaAdminUsername = "admin.username"

	// ArgoCDKeyGrafanaAdminPassword is the admin password key for labels.
	ArgoCDKeyGrafanaAdminPassword = "admin.password"

	// ArgoCDKeyGrafanaSecretKey is the "secret key" key for labels.
	ArgoCDKeyGrafanaSecretKey = "secret.key"

	// ArgoCDKeyIngressBackendProtocol is the backend-protocol key for labels.
	ArgoCDKeyIngressBackendProtocol = "nginx.ingress.kubernetes.io/backend-protocol"

	// ArgoCDKeyIngressClass is the ingress class key for labels.
	ArgoCDKeyIngressClass = "kubernetes.io/ingress.class"

	// ArgoCDKeyIngressSSLRedirect is the ssl force-redirect key for labels.
	ArgoCDKeyIngressSSLRedirect = "nginx.ingress.kubernetes.io/force-ssl-redirect"

	// ArgoCDKeyIngressSSLPassthrough is the ssl passthrough key for labels.
	ArgoCDKeyIngressSSLPassthrough = "nginx.ingress.kubernetes.io/ssl-passthrough"

	// ArgoCDKeyMetrics is the resource metrics key for labels.
	ArgoCDKeyMetrics = "metrics"

	// ArgoCDKeyName is the resource name key for labels.
	ArgoCDKeyName = "app.kubernetes.io/name"

	// ArgoCDKeyPartOf is the resource part-of key for labels.
	ArgoCDKeyPartOf = "app.kubernetes.io/part-of"

	// ArgoCDKeyPrometheus is the resource prometheus key for labels.
	ArgoCDKeyPrometheus = "prometheus"

	// ArgoCDKeyRelease is the prometheus release key for labels.
	ArgoCDKeyRelease = "release"

	// ArgoCDKeyServerURL is the key for server url.
	ArgoCDKeyServerURL = "url"

	// ArgoCDKeySSHKnownHosts is the resource ssh_known_hosts key for labels.
	ArgoCDKeySSHKnownHosts = "ssh_known_hosts"

	// ArgoCDRBACConfigMapName is the upstream hard-coded RBAC ConfigMap name.
	ArgoCDRBACConfigMapName = "argocd-rbac-cm"

	// ArgoCDSecretName is the upstream hard-coded ArgoCD Secret name.
	ArgoCDSecretName = "argocd-secret"

	// ArgoCDStatusCompleted is the completed status value.
	ArgoCDStatusCompleted = "Completed"

	// ArgoCDTLSCertsConfigMapName is the upstream hard-coded TLS certificate data ConfigMap name.
	ArgoCDTLSCertsConfigMapName = "argocd-tls-certs-cm"
)
