package applicationset

const (
	// Values
	AppSetControllerComponent  = "applicationset-controller"
	AppSetController           = "argocd-applicationset-controller"
	AppSetGitlabSCMTlsCert     = "appset-gitlab-scm-tls-cert"
	AppSetGitlabSCMTlsCertPath = "/app/tls/scm/cert"
	AppSetWebhookRouteName     = "applicationset-controller-webhook"

	// Commands
	EntryPointSh     = "entrypoint.sh"
	ArgoCDRepoServer = "--argocd-repo-server"
)
