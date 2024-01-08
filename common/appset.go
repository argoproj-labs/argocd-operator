package common

// names
const (
	// ArgoCDAppSetGitlabSCMTLSCertsConfigMapName is the hard-coded ApplicationSet Gitlab SCM TLS certificate data ConfigMap name.
	ArgoCDAppSetGitlabSCMTLSCertsConfigMapName = "argocd-appset-gitlab-scm-tls-certs-cm"

	//ApplicationSetServiceNameSuffix is the suffix for Apllication Set Controller Service
	ApplicationSetServiceNameSuffix = "applicationset-controller"

	AppSetControllerComponent  = "applicationset-controller"
	AppSetController           = "argocd-applicationset-controller"
	AppSetGitlabSCMTlsCert     = "appset-gitlab-scm-tls-cert"
	AppSetGitlabSCMTlsCertPath = "/app/tls/scm/cert"
	AppSetWebhookRouteName     = "applicationset-controller-webhook"
)

// commands
const (
	EntryPointSh = "entrypoint.sh"
)
