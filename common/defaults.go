// Copyright 2020 ArgoCD Operator Developers
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

package common

const (
	// ArgoCDApplicationControllerComponent is the name of the application controller control plane component
	ArgoCDApplicationControllerComponent = "argocd-application-controller"

	// ArgoCDApplicationControllerDefaultShardReplicas is the default number of replicas that the ArgoCD Application Controller Should Use
	ArgocdApplicationControllerDefaultReplicas = 1

	// ArgoCDDefaultLogLevel is the default log level to be used by all ArgoCD components.
	ArgoCDDefaultLogLevel = "info"

	// ArgoCDDefaultLogFormat is the default log format to be used by all ArgoCD components.
	ArgoCDDefaultLogFormat = "text"

	// ArgoCDServerComponent is the name of the Dex server control plane component
	ArgoCDServerComponent = "argocd-server"

	// ArgoCDRedisComponent is the name of the Redis control plane component
	ArgoCDRedisComponent = "argocd-redis"

	// ArgoCDRedisHAComponent is the name of the Redis HA control plane component
	ArgoCDRedisHAComponent = "argocd-redis-ha"

	// ArgoCDDexServerComponent is the name of the Dex server control plane component
	ArgoCDDexServerComponent = "argocd-dex-server"

	// ArgoCDNotificationsControllerComponent is the name of the Notifications controller control plane component
	ArgoCDNotificationsControllerComponent = "argocd-notifications-controller"

	// ArgoCDOperatorGrafanaComponent is the name of the Grafana control plane component
	ArgoCDOperatorGrafanaComponent = "argocd-grafana"

	// ArgoCDDefaultAdminPasswordLength is the length of the generated default admin password.
	ArgoCDDefaultAdminPasswordLength = 32

	// ArgoCDDefaultAdminPasswordNumDigits is the number of digits to use for the generated default admin password.
	ArgoCDDefaultAdminPasswordNumDigits = 5

	// ArgoCDDefaultAdminPasswordNumSymbols is the number of symbols to use for the generated default admin password.
	ArgoCDDefaultAdminPasswordNumSymbols = 0

	// ArgoCDDefaultApplicationInstanceLabelKey is the default app name as a tracking label.
	ArgoCDDefaultApplicationInstanceLabelKey = "app.kubernetes.io/instance"

	// ArgoCDDefaultArgoImage is the ArgoCD container image to use when not specified.
	ArgoCDDefaultArgoImage = "quay.io/argoproj/argocd"

	// ArgoCDDefaultArgoVersion is the Argo CD container image digest to use when version not specified.
	ArgoCDDefaultArgoVersion = "sha256:6b9759eb9f3d73b4295d500a0426b64ee37f778bc2b42dd67479fbb973151969" // v2.8.16

	// ArgoCDDefaultBackupKeyLength is the length of the generated default backup key.
	ArgoCDDefaultBackupKeyLength = 32

	// ArgoCDDefaultBackupKeyNumDigits is the number of digits to use for the generated default backup key.
	ArgoCDDefaultBackupKeyNumDigits = 5

	// ArgoCDDefaultBackupKeyNumSymbols is the number of symbols to use for the generated default backup key.
	ArgoCDDefaultBackupKeyNumSymbols = 5

	// ArgoCDDefaultConfigManagementPlugins is the default configuration value for the config management plugins.
	ArgoCDDefaultConfigManagementPlugins = ""

	// ArgoCDDefaultControllerResourceLimitCPU is the default CPU limit when not specified for the Argo CD application
	// controller contianer.
	ArgoCDDefaultControllerResourceLimitCPU = "1000m"

	// ArgoCDDefaultControllerResourceLimitMemory is the default memory limit when not specified for the Argo CD
	// application controller contianer.
	ArgoCDDefaultControllerResourceLimitMemory = "64Mi"

	// ArgoCDDefaultControllerResourceRequestCPU is the default CPU requested when not specified for the Argo CD
	// application controller contianer.
	ArgoCDDefaultControllerResourceRequestCPU = "250m"

	// ArgoCDDefaultControllerResourceRequestMemory is the default memory requested when not specified for the Argo CD
	// application controller contianer.
	ArgoCDDefaultControllerResourceRequestMemory = "32Mi"

	// ArgoCDDefaultDexConfig is the default dex configuration.
	ArgoCDDefaultDexConfig = ""

	// ArgoCDDefaultDexImage is the Dex container image to use when not specified.
	ArgoCDDefaultDexImage = "ghcr.io/dexidp/dex"

	// ArgoCDDefaultDexOAuthRedirectPath is the default path to use for the OAuth Redirect URI.
	ArgoCDDefaultDexOAuthRedirectPath = "/api/dex/callback"

	// ArgoCDDefaultDexGRPCPort is the default GRPC listen port for Dex.
	ArgoCDDefaultDexGRPCPort = 5557

	// ArgoCDDefaultDexHTTPPort is the default HTTP listen port for Dex.
	ArgoCDDefaultDexHTTPPort = 5556

	// ArgoCDDefaultDexMetricsPort is the default Metrics listen port for Dex.
	ArgoCDDefaultDexMetricsPort = 5558

	// ArgoCDDefaultDexServiceAccountName is the default Service Account name for the Dex server.
	ArgoCDDefaultDexServiceAccountName = "argocd-dex-server"

	// ArgoCDDefaultDexVersion is the Dex container image tag to use when not specified.
	ArgoCDDefaultDexVersion = "sha256:d5f887574312f606c61e7e188cfb11ddb33ff3bf4bd9f06e6b1458efca75f604" // v2.30.3

	// ArgoCDDefaultExportJobImage is the export job container image to use when not specified.
	ArgoCDDefaultExportJobImage = "quay.io/argoprojlabs/argocd-operator-util"

	// ArgoCDDefaultExportJobVersion is the export job container image tag to use when not specified.
	ArgoCDDefaultExportJobVersion = "sha256:9b7f3aaf6f34e57171575cdd75fe13d223c4ca9f477598f04fc6147a3eb6a2a4" // 0.8.0

	// ArgoCDDefaultExportLocalCapicity is the default capacity to use for local export.
	ArgoCDDefaultExportLocalCapicity = "2Gi"

	// ArgoCDDefaultGATrackingID is the default Google Analytics tracking ID.
	ArgoCDDefaultGATrackingID = ""

	// ArgoCDDefaultGAAnonymizeUsers is the default value for anonymizing google analytics users.
	ArgoCDDefaultGAAnonymizeUsers = false

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
	ArgoCDDefaultGrafanaVersion = "sha256:afef23a1b4cf159ec3180aac3ad693c10e560657313bfe3ec81f344ace6d2f05" // 6.7.2

	// ArgoCDDefaultHelpChatURL is the default help chat URL.
	ArgoCDDefaultHelpChatURL = ""

	// ArgoCDDefaultHelpChatText is the default help chat text.
	ArgoCDDefaultHelpChatText = ""

	// ArgoCDDefaultIngressPath is the path to use for the Ingress when not specified.
	ArgoCDDefaultIngressPath = "/"

	// ArgoCDDefaultKustomizeBuildOptions is the default kustomize build options.
	ArgoCDDefaultKustomizeBuildOptions = ""

	// ArgoCDKeycloakImage is the default Keycloak Image used for the non-openshift platforms when not specified.
	ArgoCDKeycloakImage = "quay.io/keycloak/keycloak"

	// ArgoCDKeycloakVersion is the default Keycloak version used for the non-openshift platform when not specified.
	// Version: 15.0.2
	ArgoCDKeycloakVersion = "sha256:64fb81886fde61dee55091e6033481fa5ccdac62ae30a4fd29b54eb5e97df6a9"

	// ArgoCDKeycloakImageForOpenShift is the default Keycloak Image used for the OpenShift platform when not specified.
	ArgoCDKeycloakImageForOpenShift = "registry.redhat.io/rh-sso-7/sso76-openshift-rhel8"

	// ArgoCDKeycloakVersionForOpenShift is the default Keycloak version used for the OpenShift platform when not specified.
	// Version: 7.6-32
	ArgoCDKeycloakVersionForOpenShift = "sha256:ec9f60018694dcc5d431ba47d5536b761b71cb3f66684978fe6bb74c157679ac"

	// ArgoCDDefaultOIDCConfig is the default OIDC configuration.
	ArgoCDDefaultOIDCConfig = ""

	// ArgoCDDefaultPrometheusReplicas is the default Prometheus replica count.
	ArgoCDDefaultPrometheusReplicas = int32(1)

	// ArgoCDDefaultRBACPolicy is the default RBAC policy CSV data.
	ArgoCDDefaultRBACPolicy = ""

	// ArgoCDDefaultRBACDefaultPolicy is the default Argo CD RBAC policy.
	ArgoCDDefaultRBACDefaultPolicy = "role:readonly"

	// ArgoCDDefaultRBACScopes is the default Argo CD RBAC scopes.
	ArgoCDDefaultRBACScopes = "[groups]"

	// ArgoCDDefaultRedisConfigPath is the default Redis configuration directory when not specified.
	ArgoCDDefaultRedisConfigPath = "/var/lib/redis"

	// ArgoCDDefaultRedisHAReplicas is the defaul number of replicas for Redis when rinning in HA mode.
	ArgoCDDefaultRedisHAReplicas = int32(3)

	// ArgoCDDefaultRedisHAProxyImage is the default Redis HAProxy image to use when not specified.
	ArgoCDDefaultRedisHAProxyImage = "haproxy"

	// ArgoCDDefaultRedisHAProxyVersion is the default Redis HAProxy image tag to use when not specified.
	ArgoCDDefaultRedisHAProxyVersion = "sha256:7392fbbbb53e9e063ca94891da6656e6062f9d021c0e514888a91535b9f73231" // 2.0.25-alpine

	// ArgoCDDefaultRedisImage is the Redis container image to use when not specified.
	ArgoCDDefaultRedisImage = "redis"

	// ArgoCDDefaultRedisPort is the default listen port for Redis.
	ArgoCDDefaultRedisPort = 6379

	// ArgoCDDefaultRedisSentinelPort is the default listen port for Redis sentinel.
	ArgoCDDefaultRedisSentinelPort = 26379

	//ArgoCDDefaultRedisSuffix is the default suffix to use for Redis resources.
	ArgoCDDefaultRedisSuffix = "redis"

	// ArgoCDDefaultRedisVersion is the Redis container image tag to use when not specified.
	ArgoCDDefaultRedisVersion = "sha256:8061ca607db2a0c80010aeb5fc9bed0253448bc68711eaa14253a392f6c48280" // 6.2.4-alpine

	// ArgoCDDefaultRedisVersionHA is the Redis container image tag to use when not specified in HA mode.
	ArgoCDDefaultRedisVersionHA = "sha256:8061ca607db2a0c80010aeb5fc9bed0253448bc68711eaa14253a392f6c48280" // 6.2.4-alpine

	// ArgoCDDefaultRepoMetricsPort is the default listen port for the Argo CD repo server metrics.
	ArgoCDDefaultRepoMetricsPort = 8084

	// ArgoCDDefaultRepoServerPort is the default listen port for the Argo CD repo server.
	ArgoCDDefaultRepoServerPort = 8081

	// ArgoCDDefaultRepositories is the default repositories.
	ArgoCDDefaultRepositories = ""

	// ArgoCDDefaultRepositoryCredentials is the default repository credentials
	ArgoCDDefaultRepositoryCredentials = ""

	// ArgoCDDefaultResourceExclusions is the default resource exclusions.
	ArgoCDDefaultResourceExclusions = ""

	// ArgoCDDefaultResourceInclusions is the default resource inclusions.
	ArgoCDDefaultResourceInclusions = ""

	// ArgoCDDefaultRSAKeySize is the default RSA key size when not specified.
	ArgoCDDefaultRSAKeySize = 2048

	// ArgoCDDefaultServerOperationProcessors is the number of ArgoCD Server Operation Processors to use when not specified.
	ArgoCDDefaultServerOperationProcessors = int32(10)

	// ArgoCDDefaultServerStatusProcessors is the number of ArgoCD Server Status Processors to use when not specified.
	ArgoCDDefaultServerStatusProcessors = int32(20)

	// ArgoCDDefaultControllerParellelismLimit is the default parallelism limit for application controller
	ArgoCDDefaultControllerParallelismLimit = int32(10)

	// ArgoCDDefaultServerResourceLimitCPU is the default CPU limit when not specified for the Argo CD server contianer.
	ArgoCDDefaultServerResourceLimitCPU = "1000m"

	// ArgoCDDefaultServerResourceLimitMemory is the default memory limit when not specified for the Argo CD server contianer.
	ArgoCDDefaultServerResourceLimitMemory = "128Mi"

	// ArgoCDDefaultServerResourceRequestCPU is the default CPU requested when not specified for the Argo CD server contianer.
	ArgoCDDefaultServerResourceRequestCPU = "250m"

	// ArgoCDDefaultServerResourceRequestMemory is the default memory requested when not specified for the Argo CD server contianer.
	ArgoCDDefaultServerResourceRequestMemory = "64Mi"

	// ArgoCDDefaultServerSessionKeyLength is the length of the generated default server signature key.
	ArgoCDDefaultServerSessionKeyLength = 20

	// ArgoCDDefaultServerSessionKeyNumDigits is the number of digits to use for the generated default server signature key.
	ArgoCDDefaultServerSessionKeyNumDigits = 5

	// ArgoCDDefaultServerSessionKeyNumSymbols is the number of symbols to use for the generated default server signature key.
	ArgoCDDefaultServerSessionKeyNumSymbols = 0

	// ArgoCDDefaultSSHKnownHosts is the default SSH Known hosts data.
	ArgoCDDefaultSSHKnownHosts = `[ssh.github.com]:443 ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg=
[ssh.github.com]:443 ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl
[ssh.github.com]:443 ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCj7ndNxQowgcQnjshcLrqPEiiphnt+VTTvDP6mHBL9j1aNUkY4Ue1gvwnGLVlOhGeYrnZaMgRK6+PKCUXaDbC7qtbW8gIkhL7aGCsOr/C56SJMy/BCZfxd1nWzAOxSDPgVsmerOBYfNqltV9/hWCqBywINIR+5dIg6JTJ72pcEpEjcYgXkE2YEFXV1JHnsKgbLWNlhScqb2UmyRkQyytRLtL+38TGxkxCflmO+5Z8CSSNY7GidjMIZ7Q4zMjA2n1nGrlTDkzwDCsw+wqFPGQA179cnfGWOWRVruj16z6XyvxvjJwbz0wQZ75XK5tKSb7FNyeIEs4TT4jk+S4dhPeAUC5y+bDYirYgM4GC7uEnztnZyaVWQ7B381AK4Qdrwt51ZqExKbQpTUNn+EjqoTwvqNj4kqx5QUCI0ThS/YkOxJCXmPUWZbhjpCg56i+2aB6CmK2JGhn57K5mj0MNdBXA4/WnwH6XoPWJzK5Nyu2zB3nAZp+S5hpQs+p1vN1/wsjk=
bitbucket.org ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBPIQmuzMBuKdWeF4+a2sjSSpBK0iqitSQ+5BM9KhpexuGt20JpTVM7u5BDZngncgrqDMbWdxMWWOGtZ9UgbqgZE=
bitbucket.org ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIazEu89wgQZ4bqs3d63QSMzYVa0MuJ2e2gKTKqu+UUO
bitbucket.org ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAubiN81eDcafrgMeLzaFPsw2kNvEcqTKl/VqLat/MaB33pZy0y3rJZtnqwR2qOOvbwKZYKiEO1O6VqNEBxKvJJelCq0dTXWT5pbO2gDXC6h6QDXCaHo6pOHGPUy+YBaGQRGuSusMEASYiWunYN0vCAI8QaXnWMXNMdFP3jHAJH0eDsoiGnLPBlBp4TNm6rYI74nMzgz3B9IikW4WVK+dc8KZJZWYjAuORU3jc1c/NPskD2ASinf8v3xnfXeukU0sJ5N6m5E8VLjObPEO+mN2t/FZTMZLiFqPWc/ALSqnMnnhwrNi2rbfg/rd/IpL8Le3pSBne8+seeFVBoGqzHM9yXw==
github.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg=
github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl
github.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCj7ndNxQowgcQnjshcLrqPEiiphnt+VTTvDP6mHBL9j1aNUkY4Ue1gvwnGLVlOhGeYrnZaMgRK6+PKCUXaDbC7qtbW8gIkhL7aGCsOr/C56SJMy/BCZfxd1nWzAOxSDPgVsmerOBYfNqltV9/hWCqBywINIR+5dIg6JTJ72pcEpEjcYgXkE2YEFXV1JHnsKgbLWNlhScqb2UmyRkQyytRLtL+38TGxkxCflmO+5Z8CSSNY7GidjMIZ7Q4zMjA2n1nGrlTDkzwDCsw+wqFPGQA179cnfGWOWRVruj16z6XyvxvjJwbz0wQZ75XK5tKSb7FNyeIEs4TT4jk+S4dhPeAUC5y+bDYirYgM4GC7uEnztnZyaVWQ7B381AK4Qdrwt51ZqExKbQpTUNn+EjqoTwvqNj4kqx5QUCI0ThS/YkOxJCXmPUWZbhjpCg56i+2aB6CmK2JGhn57K5mj0MNdBXA4/WnwH6XoPWJzK5Nyu2zB3nAZp+S5hpQs+p1vN1/wsjk=
gitlab.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBFSMqzJeV9rUzU4kWitGjeR4PWSa29SPqJ1fVkhtj3Hw9xjLVXVYrU9QlYWrOLXBpQ6KWjbjTDTdDkoohFzgbEY=
gitlab.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAfuCHKVTjquxvt6CM6tdG4SLp1Btn/nOeHHE5UOzRdf
gitlab.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCsj2bNKTBSpIYDEGk9KxsGh3mySTRgMtXL583qmBpzeQ+jqCMRgBqB98u3z++J1sKlXHWfM9dyhSevkMwSbhoR8XIq/U0tCNyokEi/ueaBMCvbcTHhO7FcwzY92WK4Yt0aGROY5qX2UKSeOvuP4D6TPqKF1onrSzH9bx9XUf2lEdWT/ia1NEKjunUqu1xOB/StKDHMoX4/OKyIzuS0q/T1zOATthvasJFoPrAjkohTyaDUz2LN5JoH839hViyEG82yB+MjcFV5MU3N1l1QL3cVUCh93xSaua1N85qivl+siMkPGbO5xR/En4iEY6K2XPASUEMaieWVNTRCtJ4S8H+9
ssh.dev.azure.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
vs-ssh.visualstudio.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
`
	// RedisDefaultAdminPasswordLength is the length of the generated default redis admin password.
	RedisDefaultAdminPasswordLength = 16

	// RedisDefaultAdminPasswordNumDigits is the number of digits to use for the generated default redis admin password.
	RedisDefaultAdminPasswordNumDigits = 5

	// RedisDefaultAdminPasswordNumSymbols is the number of symbols to use for the generated default redis admin password.
	RedisDefaultAdminPasswordNumSymbols = 0

	// OperatorMetricsPort is the port that is used to expose default controller-runtime metrics for the operator pod.
	OperatorMetricsPort = 8080
)

// DefaultLabels returns the default set of labels for controllers.
func DefaultLabels(name string) map[string]string {
	return map[string]string{
		ArgoCDKeyName:      name,
		ArgoCDKeyPartOf:    ArgoCDAppName,
		ArgoCDKeyManagedBy: name,
	}
}

// DefaultAnnotations returns the default set of annotations for child resources of ArgoCD
func DefaultAnnotations(name string, namespace string) map[string]string {
	return map[string]string{
		AnnotationName:      name,
		AnnotationNamespace: namespace,
	}
}

// DefaultNodeSelector returns the defult nodeSelector for ArgoCD workloads
func DefaultNodeSelector() map[string]string {
	return map[string]string{
		"kubernetes.io/os": "linux",
	}
}
