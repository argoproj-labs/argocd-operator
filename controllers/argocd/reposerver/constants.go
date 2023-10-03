package reposerver

const (
	// Values
	RepoServerControllerComponent = "repo-server"
	RepoServerController          = "argocd-repo-server"
	RepoServerMetrics             = "repo-server-metrics"
	RepoServerTLSSecretName       = "argocd-repo-server-tls"
	RedisHAProxyServiceName       = "redis-ha-haproxy"
	CopyUtil                      = "copyutil"
	// RepoServerSecretName                = "argocd-repo-server-secret"
	// RepoServerConfigMapName             = "argocd-repo-server-cm"

	// Commands
	UidEntryPointSh            = "uid_entrypoint.sh"
	LogLevel                   = "--loglevel"
	LogFormat                  = "--logformat"
	ArgoCDRepoServer           = "--argocd-repo-server"
	RepoServerTLSRedisCertPath = "/app/config/reposerver/tls/redis/tls.crt"
)
