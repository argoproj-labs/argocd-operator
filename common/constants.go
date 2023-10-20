package common

const (
	// ApplicationController
	ApplicationControllerComponent = "application-controller"

	// Notifications
	NotificationsControllerComponent = "notifications-controller"
	NotificationsSecretName          = "argocd-notifications-secret"
	NotificationsConfigMapName       = "argocd-notifications-cm"

	// RepoServer
	RepoServerControllerComponent = "repo-server"
	RepoServerController          = "argocd-repo-server"
	RepoServerMetrics             = "repo-server-metrics"
	RepoServerTLSSecretName       = "argocd-repo-server-tls"
	CopyUtil                      = "copyutil"
	// Commands
	UidEntryPointSh            = "uid_entrypoint.sh"
	ArgoCDRepoServer           = "--argocd-repo-server"
	RepoServerTLSRedisCertPath = "/app/config/reposerver/tls/redis/tls.crt"

	// Server
	ServerControllerComponent = "server"

	// Redis
	RedisControllerComponent = "redis"
	RedisHAProxyServiceName  = "redis-ha-haproxy"
	// Commands
	Redis                      = "--redis"
	RedisUseTLS                = "--redis-use-tls"
	RedisInsecureSkipTLSVerify = "--redis-insecure-skip-tls-verify"
	RedisCACertificate         = "--redis-ca-certificate"

	// ApplicationSet
	AppSetControllerComponent  = "applicationset-controller"
	AppSetController           = "argocd-applicationset-controller"
	AppSetGitlabSCMTlsCert     = "appset-gitlab-scm-tls-cert"
	AppSetGitlabSCMTlsCertPath = "/app/tls/scm/cert"
	AppSetWebhookRouteName     = "applicationset-controller-webhook"
	// Commands
	EntryPointSh = "entrypoint.sh"
)
