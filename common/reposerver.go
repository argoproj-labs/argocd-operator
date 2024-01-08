package common

// names
const (
	// ArgoCDRepoServerTLSSecretName is the name of the TLS secret for the repo-server
	ArgoCDRepoServerTLSSecretName = "argocd-repo-server-tls"

	RepoServerControllerComponent = "repo-server"
	RepoServerController          = "argocd-repo-server"
	RepoServerMetrics             = "repo-server-metrics"
	RepoServerTLSSecretName       = "argocd-repo-server-tls"
	CopyUtil                      = "copyutil"
)

// values
const (
	// ArgoCDRepoServerTLS is the argocd repo server tls value.
	ArgoCDRepoServerTLS = "argocd-repo-server-tls"
)

// defaults
const (
	// ArgoCDDefaultRepoMetricsPort is the default listen port for the Argo CD repo server metrics.
	ArgoCDDefaultRepoMetricsPort = 8084

	// ArgoCDDefaultRepoServerPort is the default listen port for the Argo CD repo server.
	ArgoCDDefaultRepoServerPort = 8081
)

// commands
const (
	UidEntryPointSh            = "uid_entrypoint.sh"
	ArgoCDRepoServer           = "--argocd-repo-server"
	RepoServerTLSRedisCertPath = "/app/config/reposerver/tls/redis/tls.crt"
)
