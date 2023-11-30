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
