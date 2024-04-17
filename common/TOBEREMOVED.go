package common

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	// ArgoCDKeyName is the resource name key for labels.
	ArgoCDKeyName = "app.kubernetes.io/name"

	// ArgoCDKeyPartOf is the resource part-of key for labels.
	ArgoCDKeyPartOf = "app.kubernetes.io/part-of"

	// ArgoCDKeyComponent is the resource component key for labels.
	ArgoCDKeyComponent = "app.kubernetes.io/component"

	// ArgoCDManagedByLabel is needed to identify namespace managed by an instance on ArgoCD
	ArgoCDManagedByLabel = "argocd.argoproj.io/managed-by"

	// ArgoCDKeyStatefulSetPodName is the resource StatefulSet Pod Name key for labels.
	ArgoCDKeyStatefulSetPodName = "statefulset.kubernetes.io/pod-name"

	// ArgoCDKeyHostname is the resource hostname key for labels.
	ArgoCDKeyHostname = "kubernetes.io/hostname"

	// ArgoCDKeyIngressClass is the ingress class key for labels.
	ArgoCDKeyIngressClass = "kubernetes.io/ingress.class"

	// ArgoCDKeyIngressBackendProtocol is the backend-protocol key for labels.
	ArgoCDKeyIngressBackendProtocol = "nginx.ingress.kubernetes.io/backend-protocol"

	// ArgoCDKeyIngressSSLRedirect is the ssl force-redirect key for labels.
	ArgoCDKeyIngressSSLRedirect = "nginx.ingress.kubernetes.io/force-ssl-redirect"

	// ArgoCDKeyIngressSSLPassthrough is the ssl passthrough key for labels.
	ArgoCDKeyIngressSSLPassthrough = "nginx.ingress.kubernetes.io/ssl-passthrough"

	// ArgoCDKeyTolerateUnreadyEndpounts is the resource tolerate unready endpoints key for labels.
	ArgoCDKeyTolerateUnreadyEndpounts = "service.alpha.kubernetes.io/tolerate-unready-endpoints"

	// ArgoCDKeyFailureDomainZone is the failure-domain zone key for labels.
	ArgoCDKeyFailureDomainZone = "failure-domain.beta.kubernetes.io/zone"

	// ArgoCDKeyDexOAuthRedirectURI is the key for the OAuth Redirect URI annotation.
	ArgoCDKeyDexOAuthRedirectURI = "serviceaccounts.openshift.io/oauth-redirecturi.argocd"

	// AnnotationOpenShiftServiceCA is the annotation on services used to
	// request a TLS certificate from OpenShift's Service CA for AutoTLS
	AnnotationOpenShiftServiceCA = "service.beta.openshift.io/serving-cert-secret-name"

	// AnnotationName is the annotation on child resources that specifies which ArgoCD instance
	// name a specific object is associated with
	AnnotationName = "argocds.argoproj.io/name"

	// AnnotationNamespace is the annotation on child resources that specifies which ArgoCD instance
	// namespace a specific object is associated with
	AnnotationNamespace = "argocds.argoproj.io/namespace"

	// ArgoCDDeletionFinalizer is a finalizer to implement pre-delete hooks
	ArgoCDDeletionFinalizer = "argoproj.io/finalizer"

	// ArgoCDSecretTypeLabel is needed for cluster secrets
	ArgoCDSecretTypeLabel = "argocd.argoproj.io/secret-type"

	// ArgoCDKeyManagedBy is the managed-by key for labels.
	ArgoCDKeyManagedBy = "app.kubernetes.io/managed-by"

	// ArgoCDManagedByClusterArgoCDLabel is needed to identify namespace mentioned as sourceNamespace on ArgoCD
	ArgoCDManagedByClusterArgoCDLabel = "argocd.argoproj.io/managed-by-cluster-argocd"

	// ArgoCDDexImageEnvName is the environment variable used to get the image
	// to used for the Dex container.
	ArgoCDDexImageEnvName = "ARGOCD_DEX_IMAGE"

	// ArgoCDImageEnvName is the environment variable used to get the image
	// to used for the argocd container.
	ArgoCDImageEnvName = "ARGOCD_IMAGE"

	// ArgoCDKeycloakImageEnvName is the environment variable used to get the image
	// to used for the Keycloak container.
	ArgoCDKeycloakImageEnvName = "ARGOCD_KEYCLOAK_IMAGE"

	// ArgoCDRedisHAProxyImageEnvName is the environment variable used to get the image
	// to used for the Redis HA Proxy container.
	ArgoCDRedisHAProxyImageEnvName = "ARGOCD_REDIS_HA_PROXY_IMAGE"

	// ArgoCDRedisHAImageEnvName is the environment variable used to get the image
	// to used for the the Redis container in HA mode.
	ArgoCDRedisHAImageEnvName = "ARGOCD_REDIS_HA_IMAGE"

	// ArgoCDRedisImageEnvName is the environment variable used to get the image
	// to used for the Redis container.
	ArgoCDRedisImageEnvName = "ARGOCD_REDIS_IMAGE"

	// ArgoCDGrafanaImageEnvName is the environment variable used to get the image
	// to used for the Grafana container.
	ArgoCDGrafanaImageEnvName = "ARGOCD_GRAFANA_IMAGE"

	// ArgoCDControllerClusterRoleEnvName is an environment variable to specify a custom cluster role for Argo CD application controller
	ArgoCDControllerClusterRoleEnvName = "CONTROLLER_CLUSTER_ROLE"

	// ArgoCDServerClusterRoleEnvName is an environment variable to specify a custom cluster role for Argo CD server
	ArgoCDServerClusterRoleEnvName = "SERVER_CLUSTER_ROLE"

	// ArgoCDRepoImageEnvName is the environment variable used to get the image
	// to used for the Dex container.
	ArgoCDRepoImageEnvName = "ARGOCD_REPOSERVER_IMAGE"

	// Label Selector is an env variable for ArgoCD instance reconcilliation.
	ArgoCDLabelSelectorKey = "ARGOCD_LABEL_SELECTOR"

	// ArgoCDKeyTLSCert is the key for TLS certificates.
	ArgoCDKeyTLSCert = corev1.TLSCertKey

	// ArgoCDKeyTLSPrivateKey is the key for TLS private keys.
	ArgoCDKeyTLSPrivateKey = corev1.TLSPrivateKeyKey

	// ArgoCDNotificationsControllerComponent is the name of the Notifications controller control plane component
	ArgoCDNotificationsControllerComponent = "argocd-notifications-controller"

	ArgoCDDefaultRedisSuffix = "redis"

	// ArgoCDRedisComponent is the name of the Redis control plane component
	ArgoCDRedisComponent = "argocd-redis"

	// ArgoCDRedisHAComponent is the name of the Redis HA control plane component
	ArgoCDRedisHAComponent = "argocd-redis-ha"

	// ArgoCDDefaultRedisPort is the default listen port for Redis.
	ArgoCDDefaultRedisPort = 6379

	// ArgoCDDefaultRedisImage is the Redis container image to use when not specified.
	ArgoCDDefaultRedisImage = "redis"

	// ArgoCDDefaultRedisSentinelPort is the default listen port for Redis sentinel.
	ArgoCDDefaultRedisSentinelPort = 26379

	// ArgoCDDefaultRedisVersion is the Redis container image tag to use when not specified.
	ArgoCDDefaultRedisVersion = "sha256:8061ca607db2a0c80010aeb5fc9bed0253448bc68711eaa14253a392f6c48280" // 6.2.4-alpine

	// ArgoCDDefaultRedisVersionHA is the Redis container image tag to use when not specified in HA mode.
	ArgoCDDefaultRedisVersionHA = "sha256:8061ca607db2a0c80010aeb5fc9bed0253448bc68711eaa14253a392f6c48280" // 6.2.4-alpine

	// ArgoCDDefaultRedisConfigPath is the default Redis configuration directory when not specified.
	ArgoCDDefaultRedisConfigPath = "/var/lib/redis"

	// ArgoCDDefaultRedisHAReplicas is the defaul number of replicas for Redis when rinning in HA mode.
	ArgoCDDefaultRedisHAReplicas = int32(3)

	// ArgoCDDefaultRedisHAProxyImage is the default Redis HAProxy image to use when not specified.
	ArgoCDDefaultRedisHAProxyImage = "haproxy"

	// ArgoCDDefaultRedisHAProxyVersion is the default Redis HAProxy image tag to use when not specified.
	ArgoCDDefaultRedisHAProxyVersion = "sha256:7392fbbbb53e9e063ca94891da6656e6062f9d021c0e514888a91535b9f73231" // 2.0.25-alpine

	// ArgoCDRedisHAProxyImageEnvVar is the environment variable used to get the image
	// to used for the Redis HA Proxy container.
	ArgoCDRedisHAProxyImageEnvVar = "ARGOCD_REDIS_HA_PROXY_IMAGE"

	// ArgoCDRedisHAImageEnvVar is the environment variable used to get the image
	// to used for the the Redis container in HA mode.
	ArgoCDRedisHAImageEnvVar = "ARGOCD_REDIS_HA_IMAGE"

	// ArgoCDPolicyMatcherMode is the key for matchers function for casbin.
	// There are two options for this, 'glob' for glob matcher or 'regex' for regex matcher.
	ArgoCDPolicyMatcherMode = "policy.matchMode"

	// ArgoCDDefaultRepoMetricsPort is the default listen port for the Argo CD repo server metrics.
	ArgoCDDefaultRepoMetricsPort = 8084

	// ArgoCDDefaultRepoServerPort is the default listen port for the Argo CD repo server.
	ArgoCDDefaultRepoServerPort = 8081

	// ArgoCDKeyRelease is the prometheus release key for labels.
	ArgoCDKeyRelease = "release"

	// ArgoCDArgoprojKeyManagedByClusterArgoCD is needed to identify namespace mentioned as sourceNamespace on ArgoCD
	ArgoCDArgoprojKeyManagedByClusterArgoCD = "argocd.argoproj.io/managed-by-cluster-argocd"

	// ArgoCDApplicationControllerComponent is the name of the application controller control plane component
	ArgoCDApplicationControllerComponent = "argocd-application-controller"

	// ArgoCDServerComponent is the name of the Dex server control plane component
	ArgoCDServerComponent = "argocd-server"

	//ApplicationSetServiceNameSuffix is the suffix for Apllication Set Controller Service
	ApplicationSetServiceNameSuffix = "applicationset-controller"
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

const (
	// ArgoCDCASuffix is the name suffix for ArgoCD CA resources.
	ArgoCDCASuffix = "ca"
)

// names
const (
	// ArgoCDDexServerComponent is the name of the Dex server control plane component
	ArgoCDDexServerComponent = "argocd-dex-server"

	// ArgoCDDefaultDexServiceAccountName is the default Service Account name for the Dex server.
	ArgoCDDefaultDexServiceAccountName = "argocd-dex-server"
)

// keys
const (
	// ArgoCDKeyDexConfig is the key for dex configuration.
	ArgoCDKeyDexConfig = "dex.config"
)

// defaults
const (

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

	// ArgoCDDefaultDexVersion is the Dex container image tag to use when not specified.
	ArgoCDDefaultDexVersion = "sha256:d5f887574312f606c61e7e188cfb11ddb33ff3bf4bd9f06e6b1458efca75f604" // v2.30.3
)
