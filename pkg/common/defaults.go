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
	// ArgoCDDefaultAdminPasswordLength is the length of the generated default admin password.
	ArgoCDDefaultAdminPasswordLength = 32

	// ArgoCDDefaultAdminPasswordNumDigits is the number of digits to use for the generated default admin password.
	ArgoCDDefaultAdminPasswordNumDigits = 5

	// ArgoCDDefaultAdminPasswordNumSymbols is the number of symbols to use for the generated default admin password.
	ArgoCDDefaultAdminPasswordNumSymbols = 0

	// ArgoCDDefaultApplicationInstanceLabelKey is the default app name as a tracking label.
	ArgoCDDefaultApplicationInstanceLabelKey = "mycompany.com/appname"

	// ArgoCDDefaultArgoImage is the ArgoCD container image to use when not specified.
	ArgoCDDefaultArgoImage = "argoproj/argocd"

	// ArgoCDDefaultArgoVersion is the Argo CD container image digest to use when version not specified.
	ArgoCDDefaultArgoVersion = "sha256:b835999eb5cf75d01a2678cd971095926d9c2566c9ffe746d04b83a6a0a2849f" // v1.7.7

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
	ArgoCDDefaultDexImage = "quay.io/dexidp/dex"

	// ArgoCDDefaultDexOAuthRedirectPath is the default path to use for the OAuth Redirect URI.
	ArgoCDDefaultDexOAuthRedirectPath = "/api/dex/callback"

	// ArgoCDDefaultDexGRPCPort is the default GRPC listen port for Dex.
	ArgoCDDefaultDexGRPCPort = 5557

	// ArgoCDDefaultDexHTTPPort is the default HTTP listen port for Dex.
	ArgoCDDefaultDexHTTPPort = 5556

	// ArgoCDDefaultDexServiceAccountName is the default Service Account name for the Dex server.
	ArgoCDDefaultDexServiceAccountName = "argocd-dex-server"

	// ArgoCDDefaultDexVersion is the Dex container image tag to use when not specified.
	ArgoCDDefaultDexVersion = "sha256:01e996b4b60edcc5cc042227c6965dd63ba68764c25d86b481b0d65f6e4da308" // v2.22.0

	// ArgoCDDefaultExportJobImage is the export job container image to use when not specified.
	ArgoCDDefaultExportJobImage = "quay.io/jmckind/argocd-operator-util"

	// ArgoCDDefaultExportJobVersion is the export job container image tag to use when not specified.
	ArgoCDDefaultExportJobVersion = "sha256:f987796f3b4be500516689b24988f5c27f8d4f6e537b5c5d862090451de90794" // v0.0.14

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
	ArgoCDDefaultHelpChatURL = "https://mycorp.slack.com/argo-cd"

	// ArgoCDDefaultHelpChatText is the default help chat text.
	ArgoCDDefaultHelpChatText = "Chat now!"

	// ArgoCDDefaultIngressPath is the path to use for the Ingress when not specified.
	ArgoCDDefaultIngressPath = "/"

	// ArgoCDDefaultKustomizeBuildOptions is the default kustomize build options.
	ArgoCDDefaultKustomizeBuildOptions = ""

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
	ArgoCDDefaultRedisHAProxyVersion = "sha256:cd4b3d4d27ae5931dc96b9632188590b7a6880469bcf07f478a3280dd0955336" // 2.0.4

	// ArgoCDDefaultRedisImage is the Redis container image to use when not specified.
	ArgoCDDefaultRedisImage = "redis"

	// ArgoCDDefaultRedisPort is the default listen port for Redis.
	ArgoCDDefaultRedisPort = 6379

	// ArgoCDDefaultRedisSentinelPort is the default listen port for Redis sentinel.
	ArgoCDDefaultRedisSentinelPort = 26379

	//ArgoCDDefaultRedisSuffix is the default suffix to use for Redis resources.
	ArgoCDDefaultRedisSuffix = "redis"

	// ArgoCDDefaultRedisVersion is the Redis container image tag to use when not specified.
	ArgoCDDefaultRedisVersion = "sha256:4be7fdb131e76a6c6231e820c60b8b12938cf1ff3d437da4871b9b2440f4e385" // 5.0.3

	// ArgoCDDefaultRedisVersionHA is the Redis container image tag to use when not specified in HA mode.
	ArgoCDDefaultRedisVersionHA = "sha256:27e139dd0476133961d36e5abdbbb9edf9f596f80cc2f9c2e8f37b20b91d610d" // 5.0.6-alpine

	// ArgoCDDefaultRepoMetricsPort is the default listen port for the Argo CD repo server metrics.
	ArgoCDDefaultRepoMetricsPort = 8084

	// ArgoCDDefaultRepoServerPort is the default listen port for the Argo CD repo server.
	ArgoCDDefaultRepoServerPort = 8081

	// ArgoCDDefaultRepositories is the default repositories.
	ArgoCDDefaultRepositories = ""

	// ArgoCDDefaultRepositoryCredentials is the default repository credentials
	ArgoCDDefaultRepositoryCredentials = ""

	// ArgoCDDefaultResourceCustomizations is the default resource customizations.
	ArgoCDDefaultResourceCustomizations = ""

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
	ArgoCDDefaultSSHKnownHosts = `bitbucket.org ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAubiN81eDcafrgMeLzaFPsw2kNvEcqTKl/VqLat/MaB33pZy0y3rJZtnqwR2qOOvbwKZYKiEO1O6VqNEBxKvJJelCq0dTXWT5pbO2gDXC6h6QDXCaHo6pOHGPUy+YBaGQRGuSusMEASYiWunYN0vCAI8QaXnWMXNMdFP3jHAJH0eDsoiGnLPBlBp4TNm6rYI74nMzgz3B9IikW4WVK+dc8KZJZWYjAuORU3jc1c/NPskD2ASinf8v3xnfXeukU0sJ5N6m5E8VLjObPEO+mN2t/FZTMZLiFqPWc/ALSqnMnnhwrNi2rbfg/rd/IpL8Le3pSBne8+seeFVBoGqzHM9yXw==
github.com ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ==
gitlab.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBFSMqzJeV9rUzU4kWitGjeR4PWSa29SPqJ1fVkhtj3Hw9xjLVXVYrU9QlYWrOLXBpQ6KWjbjTDTdDkoohFzgbEY=
gitlab.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAfuCHKVTjquxvt6CM6tdG4SLp1Btn/nOeHHE5UOzRdf
gitlab.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCsj2bNKTBSpIYDEGk9KxsGh3mySTRgMtXL583qmBpzeQ+jqCMRgBqB98u3z++J1sKlXHWfM9dyhSevkMwSbhoR8XIq/U0tCNyokEi/ueaBMCvbcTHhO7FcwzY92WK4Yt0aGROY5qX2UKSeOvuP4D6TPqKF1onrSzH9bx9XUf2lEdWT/ia1NEKjunUqu1xOB/StKDHMoX4/OKyIzuS0q/T1zOATthvasJFoPrAjkohTyaDUz2LN5JoH839hViyEG82yB+MjcFV5MU3N1l1QL3cVUCh93xSaua1N85qivl+siMkPGbO5xR/En4iEY6K2XPASUEMaieWVNTRCtJ4S8H+9
ssh.dev.azure.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
vs-ssh.visualstudio.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
`
)
