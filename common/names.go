package common

// names
const (
	// ArgoCDAppName is the application name for labels.
	ArgoCDAppName = "argocd"

	// ArgoCDConfigMapName is the upstream hard-coded ArgoCD ConfigMap name.
	ArgoCDConfigMapName = "argocd-cm"

	// ArgoCDGPGKeysConfigMapName is the upstream hard-coded ArgoCD gpg-keys ConfigMap name.
	ArgoCDGPGKeysConfigMapName = "argocd-gpg-keys-cm"

	// ArgoCDKnownHostsConfigMapName is the u i.e default image versions together, defaultpstream hard-coded SSH known hosts data ConfigMap name.
	ArgoCDKnownHostsConfigMapName = "argocd-ssh-known-hosts-cm"

	// ArgoCDRBACConfigMapName is the upstream hard-coded RBAC ConfigMap name.
	ArgoCDRBACConfigMapName = "argocd-rbac-cm"

	// ArgoCDSecretName is the upstream hard-coded ArgoCD Secret name.
	ArgoCDSecretName = "argocd-secret"

	// ArgoCDTLSCertsConfigMapName is the upstream hard-coded TLS certificate data ConfigMap name.
	ArgoCDTLSCertsConfigMapName = "argocd-tls-certs-cm"

	// ArgoCDComponentStatus is the default group name of argocd-component-status-alert prometheusRule
	ArgoCDComponentStatus = "ArgoCDComponentStatus"

	// ArgoCDOperatorName is the name of the operator that manages Argo CD instances and workloads
	ArgoCDOperatorName = "argocd-operator"

	ArogCDComponentStatusAlertRuleName = "argocd-component-status-alert"
)

// suffixes
const (
	// CASuffix is the name suffix for ArgoCD CA resources.
	CASuffix = "ca"

	// TLSSuffix is the name suffix for ArgoCD TLS resources.
	TLSSuffix = "tls"

	// GRPCSuffix is the name suffix for ArgoCD GRPC resources.
	GRPCSuffix = "grpc"

	CLusterSuffix = "cluster"

	MetricsSuffix = "metrics"

	AppMgmtSuffix = "app-mgmt"

	AppsetMgmtSuffix = "appset-mgmt"

	ResourceMgmtSuffix = "resource-mgmt"

	WebhookSuffix = "webhook"
)
