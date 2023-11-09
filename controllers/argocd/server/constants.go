package server

// Values
const (
	ServerControllerComponent = "server"
)

// Env
const (
	// ArgoCDServerClusterRoleEnvVar is an environment variable to specify a custom cluster role for Argo CD server
	ArgoCDServerClusterRoleEnvVar = "SERVER_CLUSTER_ROLE"
)

// suffixes
const (
	// ArgoCDServerSuffix is the name suffix for ArgoCD Server RBAC resources.
	ArgoCDServerSuffix = "argocd-server"

	// ServerSuffix is the name suffix for ArgoCD Server Deployment resources.
	ServerSuffix = "server"
)

// ingress
const (
	// ArgoCDKeyIngressBackendProtocol is the backend-protocol key for labels.
	ArgoCDKeyIngressBackendProtocol = "nginx.ingress.kubernetes.io/backend-protocol"

	// ArgoCDKeyIngressClass is the ingress class key for labels.
	ArgoCDKeyIngressClass = "kubernetes.io/ingress.class"

	// ArgoCDKeyIngressSSLRedirect is the ssl force-redirect key for labels.
	ArgoCDKeyIngressSSLRedirect = "nginx.ingress.kubernetes.io/force-ssl-redirect"

	// ArgoCDKeyIngressSSLPassthrough is the ssl passthrough key for labels.
	ArgoCDKeyIngressSSLPassthrough = "nginx.ingress.kubernetes.io/ssl-passthrough"
)