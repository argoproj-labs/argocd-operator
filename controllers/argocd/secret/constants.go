package secret

// names
const (
	// ArgoCDSecretName is the upstream hard-coded ArgoCD Secret name.
	ArgoCDSecretName = "argocd-secret"

	InClusterSecretName = "in-cluster"

	SecretsControllerName = "secrets-controller"
)

// suffixes
const (
	DefaultClusterConfigSuffix      = "default-cluster-config"
	DefaultClusterCredentialsSuffix = "cluster"
)

// values
const (
	ClusterSecretType = "cluster"
)
