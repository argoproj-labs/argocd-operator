package common

// names
const (
	// ArgoCDAppName is the application name for labels.
	ArgoCDAppName = "argocd"

	// ArgoCDConfigMapName is the upstream hard-coded ArgoCD ConfigMap name.
	ArgoCDConfigMapName = "argocd-cm"

	// ArgoCDGPGKeysConfigMapName is the upstream hard-coded ArgoCD gpg-keys ConfigMap name.
	ArgoCDGPGKeysConfigMapName = "argocd-gpg-keys-cm"

	// ArgoCDExportName is the export name for labels.
	ArgoCDExportName = "argocd.export"

	// ArgoCDKnownHostsConfigMapName is the u i.e default image versions together, defaultpstream hard-coded SSH known hosts data ConfigMap name.
	ArgoCDKnownHostsConfigMapName = "argocd-ssh-known-hosts-cm"

	// ArgoCDRedisHAConfigMapName is the upstream ArgoCD Redis HA ConfigMap name.
	ArgoCDRedisHAConfigMapName = "argocd-redis-ha-configmap"

	// ArgoCDRedisHAHealthConfigMapName is the upstream ArgoCD Redis HA Health ConfigMap name.
	ArgoCDRedisHAHealthConfigMapName = "argocd-redis-ha-health-configmap"

	// ArgoCDRedisProbesConfigMapName is the upstream ArgoCD Redis Probes ConfigMap name.
	ArgoCDRedisProbesConfigMapName = "argocd-redis-ha-probes"

	// ArgoCDRBACConfigMapName is the upstream hard-coded RBAC ConfigMap name.
	ArgoCDRBACConfigMapName = "argocd-rbac-cm"

	// ArgoCDSecretName is the upstream hard-coded ArgoCD Secret name.
	ArgoCDSecretName = "argocd-secret"

	// ArgoCDTLSCertsConfigMapName is the upstream hard-coded TLS certificate data ConfigMap name.
	ArgoCDTLSCertsConfigMapName = "argocd-tls-certs-cm"

	// ArgoCDRedisServerTLSSecretName is the name of the TLS secret for the redis-server
	ArgoCDRedisServerTLSSecretName = "argocd-operator-redis-tls"

	// ArgoCDRepoServerTLSSecretName is the name of the TLS secret for the repo-server
	ArgoCDRepoServerTLSSecretName = "argocd-repo-server-tls"

	// ArgoCDServerTLSSecretName is the name of the TLS secret for the argocd-server
	ArgoCDServerTLSSecretName = "argocd-server-tls"

	// ArgoCDOperatorName is the name of the operator that manages Argo CD instances and workloads
	ArgoCDOperatorName = "argocd-operator"
)

// suffixes
const (
	// ArgoCDCASuffix is the name suffix for ArgoCD CA resources.
	ArgoCDCASuffix = "ca"

	// ArgoCDGrafanaConfigMapSuffix is the default suffix for the Grafana configuration ConfigMap.
	ArgoCDGrafanaConfigMapSuffix = "grafana-config"

	// ArgoCDGrafanaDashboardConfigMapSuffix is the default suffix for the Grafana dashboards ConfigMap.
	ArgoCDGrafanaDashboardConfigMapSuffix = "grafana-dashboards"

	//ApplicationSetServiceNameSuffix is the suffix for Apllication Set Controller Service
	ApplicationSetServiceNameSuffix = "applicationset-controller"
)
