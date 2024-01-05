package common

// names
const (
	// ArgoCDOperatorGrafanaComponent is the name of the Grafana control plane component
	ArgoCDOperatorGrafanaComponent = "argocd-grafana"

	// ArgoCDGrafanaConfigMapSuffix is the default suffix for the Grafana configuration ConfigMap.
	ArgoCDGrafanaConfigMapSuffix = "grafana-config"

	// ArgoCDGrafanaDashboardConfigMapSuffix is the default suffix for the Grafana dashboards ConfigMap.
	ArgoCDGrafanaDashboardConfigMapSuffix = "grafana-dashboards"
)

// grafana
const (

	// ArgoCDDefaultGrafanaAdminUsername is the Grafana admin username to use when not specified.
	ArgoCDDefaultGrafanaAdminUsername = "admin"

	// ArgoCDDefaultGrafanaAdminPasswordLength is the length of the generated default Grafana admin password.
	ArgoCDDefaultGrafanaAdminPasswordLength = 32

	// ArgoCDDefaultGrafanaAdminPasswordNumDigits is the number of digits to use for the generated default Grafana admin password.
	ArgoCDDefaultGrafanaAdminPasswordNumDigits = 5

	// ArgoCDDefaultGrafanaAdminPasswordNumSymbols is the number of symbols to use for the generated default Grafana admin password.
	ArgoCDDefaultGrafanaAdminPasswordNumSymbols = 5

	// ArgoCDDefaultGrafanaImage is the Grafana container image to use when not specified.
	ArgoCDDefaultGrafanaImage = "grafana/grafana"

	// ArgoCDDefaultGrafanaReplicas is the default Grafana replica count.
	ArgoCDDefaultGrafanaReplicas = int32(1)

	// ArgoCDDefaultGrafanaSecretKeyLength is the length of the generated default Grafana secret key.
	ArgoCDDefaultGrafanaSecretKeyLength = 20

	// ArgoCDDefaultGrafanaSecretKeyNumDigits is the number of digits to use for the generated default Grafana secret key.
	ArgoCDDefaultGrafanaSecretKeyNumDigits = 5

	// ArgoCDDefaultGrafanaSecretKeyNumSymbols is the number of symbols to use for the generated default Grafana secret key.
	ArgoCDDefaultGrafanaSecretKeyNumSymbols = 0

	// ArgoCDDefaultGrafanaConfigPath is the default Grafana configuration directory when not specified.
	ArgoCDDefaultGrafanaConfigPath = "/var/lib/grafana"

	// ArgoCDDefaultGrafanaVersion is the Grafana container image tag to use when not specified.
	ArgoCDDefaultGrafanaVersion = "sha256:afef23a1b4cf159ec3180aac3ad693c10e560657313bfe3ec81f344ace6d2f05" // 6.7.2
)
