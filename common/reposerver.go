package common

// names
const (
	RepoServerController = "repo-server-controller"

	// RepoServerComponent is the repo-server control plane component
	RepoServerComponent = "repo-server"

	// ArgoCDRepoServerTLSSecretName is the name of the TLS secret for the repo-server
	ArgoCDRepoServerTLSSecretName = "argocd-repo-server-tls"
)

// suffixes
const (
	RepoServerSuffix = "repo-server"

	RepoServerMetricsSuffix = "repo-server-metrics"
)

// values
const (
	// ArgoCDRepoServerTLS is the argocd repo server tls value.
	ArgoCDRepoServerTLS = "argocd-repo-server-tls"
)

// defaults
const (
	// DefaultRepoServerMetricsPort is the default listen port for the Argo CD repo server metrics.
	DefaultRepoServerMetricsPort = 8084

	// DefaultRepoServerPort is the default listen port for the Argo CD repo server.
	DefaultRepoServerPort = 8081
)

// keys
const (
	RepoTLSCertChangedKey = "repo.tls.cert.changed"
)

// commands
const (
	RepoServerCmd = "argocd-repo-server"
)
