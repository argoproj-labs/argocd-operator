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

	// ArgoCDApplicationControllerComponentView is the name of aggregated ClusterRole to configure view permissions for the application controller control plane component
	ArgoCDApplicationControllerComponentView = "argocd-application-controller-view"

	// ArgoCDApplicationControllerComponentAdmin is the name of aggregated ClusterRole to configure admin permissions for the application controller control plane component
	ArgoCDApplicationControllerComponentAdmin = "argocd-application-controller-admin"

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

	// ArgoCDApplicationSetControllerComponent is the name of the ApplictionSet controller control plane component
	ArgoCDApplicationSetControllerComponent = "argocd-applicationset-controller"

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
	ArgoCDDefaultArgoVersion = "sha256:a8532a23ed5f6e65afaf2a082b65fc74614549e54774f6b25fe3c993faa7bea3" // v3.2.1

	// ArgoCDDefaultBackupKeyLength is the length of the generated default backup key.
	ArgoCDDefaultBackupKeyLength = 32

	// ArgoCDDefaultBackupKeyNumDigits is the number of digits to use for the generated default backup key.
	ArgoCDDefaultBackupKeyNumDigits = 5

	// ArgoCDDefaultBackupKeyNumSymbols is the number of symbols to use for the generated default backup key.
	ArgoCDDefaultBackupKeyNumSymbols = 5

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
	ArgoCDDefaultDexVersion = "sha256:b08a58c9731c693b8db02154d7afda798e1888dc76db30d34c4a0d0b8a26d913" // v2.43.0

	// ArgoCDDefaultExportJobImage is the export job container image to use when not specified.
	ArgoCDDefaultExportJobImage = "quay.io/argoprojlabs/argocd-operator-util"

	// ArgoCDDefaultExportJobVersion is the export job container image tag to use when not specified.
	ArgoCDDefaultExportJobVersion = "sha256:0745934cb55d95c266daa5423ece9c149bb67db99eb2b3d9215597903724c636" // 0.13.0

	// ArgoCDDefaultExportLocalCapicity is the default capacity to use for local export.
	ArgoCDDefaultExportLocalCapicity = "2Gi"

	// ArgoCDDefaultGATrackingID is the default Google Analytics tracking ID.
	ArgoCDDefaultGATrackingID = ""

	// ArgoCDDefaultGAAnonymizeUsers is the default value for anonymizing google analytics users.
	ArgoCDDefaultGAAnonymizeUsers = false

	// ArgoCDDefaultHelpChatURL is the default help chat URL.
	ArgoCDDefaultHelpChatURL = ""

	// ArgoCDDefaultHelpChatText is the default help chat text.
	ArgoCDDefaultHelpChatText = ""

	// ArgoCDDefaultIngressPath is the path to use for the Ingress when not specified.
	ArgoCDDefaultIngressPath = "/"

	// ArgoCDDefaultKustomizeBuildOptions is the default kustomize build options.
	ArgoCDDefaultKustomizeBuildOptions = ""

	// ArgoCDDefaultLabelSelector is the default Label Selector which will reconcile all ArgoCD instances.
	ArgoCDDefaultLabelSelector = ""

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
	ArgoCDDefaultRedisHAProxyImage = "public.ecr.aws/docker/library/haproxy"

	// ArgoCDDefaultRedisHAProxyVersion is the default Redis HAProxy image tag to use when not specified.
	ArgoCDDefaultRedisHAProxyVersion = "sha256:e11f034e651603f10a365e5ad5a0321825e18eded9620e40c4f4d6ae58419bfe" // 3.0.8-alpine

	// ArgoCDDefaultRedisImage is the Redis container image to use when not specified.
	ArgoCDDefaultRedisImage = "public.ecr.aws/docker/library/redis"

	// ArgoCDDefaultRedisPort is the default listen port for Redis.
	ArgoCDDefaultRedisPort = 6379

	// ArgoCDDefaultRedisSentinelPort is the default listen port for Redis sentinel.
	ArgoCDDefaultRedisSentinelPort = 26379

	//ArgoCDDefaultRedisSuffix is the default suffix to use for Redis resources.
	ArgoCDDefaultRedisSuffix = "redis"

	// ArgoCDDefaultRedisVersion is the Redis container image tag to use when not specified.
	ArgoCDDefaultRedisVersion = "sha256:59b6e694653476de2c992937ebe1c64182af4728e54bb49e9b7a6c26614d8933" // 8.2.2-alpine

	// ArgoCDDefaultRedisVersionHA is the Redis container image tag to use when not specified in HA mode.
	ArgoCDDefaultRedisVersionHA = "sha256:59b6e694653476de2c992937ebe1c64182af4728e54bb49e9b7a6c26614d8933" // 8.2.2-alpine

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

	// ArgoCDExtensionInstallerImage is the default image for ArgoCD Extension Installer that can be used to install UI extensions like Rollouts extension.
	ArgoCDExtensionInstallerImage = "quay.io/argoprojlabs/argocd-extension-installer:v0.0.8"

	// ArgoRolloutsExtensionURL is the URL used to download the extension.js file from the latest rollout-extension tar release
	ArgoRolloutsExtensionURL = "https://github.com/argoproj-labs/rollout-extension/releases/download/v0.3.6/extension.tar"

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

	// NotificationsControllerMetricsPort is the port that is used to expose notifications controller metrics.
	NotificationsControllerMetricsPort = 9001

	// ArgoCDCmdParamsConfigMapName is the upstream hard-coded ArgoCD command params ConfigMap name.
	ArgoCDCmdParamsConfigMapName = "argocd-cmd-params-cm"

	// ArgoCDAgentPrincipalDefaultImageName is the default image name for the ArgoCD agent's principal component.
	ArgoCDAgentPrincipalDefaultImageName = "quay.io/argoprojlabs/argocd-agent:v0.5.2"

	// ArgoCDAgentAgentDefaultImageName is the default image name for the ArgoCD agent's agent component.
	ArgoCDAgentAgentDefaultImageName = "quay.io/argoprojlabs/argocd-agent:v0.5.2"

	// ArgoCDImageUpdaterControllerComponent is the name of the Image Updater controller control plane component
	ArgoCDImageUpdaterControllerComponent = "argocd-image-updater-controller"

	// DefaultImagePullPolicy is the default image pull policy to use when not specified.
	DefaultImagePullPolicy = "IfNotPresent"
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
