package common

// env vars
const (
	// ArgoCDDexImageEnvVar is the environment variable used to get the image
	// to used for the Dex container.
	ArgoCDDexImageEnvVar = "ARGOCD_DEX_IMAGE"

	// ArgoCDImageEnvVar is the environment variable used to get the image
	// to used for the argocd container.
	ArgoCDImageEnvVar = "ARGOCD_IMAGE"

	// ArgoCDKeycloakImageEnvVar is the environment variable used to get the image
	// to used for the Keycloak container.
	ArgoCDKeycloakImageEnvVar = "ARGOCD_KEYCLOAK_IMAGE"

	// ArgoCDGrafanaImageEnvVar is the environment variable used to get the image
	// to used for the Grafana container.
	ArgoCDGrafanaImageEnvVar = "ARGOCD_GRAFANA_IMAGE"

	// ArgoCDClusterConfigNamespacesEnvVar is the environment variable that contains the list of namespaces allowed to host cluster config
	// instances
	ArgoCDClusterConfigNamespacesEnvVar = "ARGOCD_CLUSTER_CONFIG_NAMESPACES"

	// ArgoCDLabelSelectorEnvVar is an environment variable that contains the labels used for selective instance reconilliation.
	ArgoCDLabelSelectorEnvVar = "ARGOCD_LABEL_SELECTOR"

	ArgoCDExecTimeoutEnvVar = "ARGOCD_EXEC_TIMEOUT"

	ArgoCDOperatorLogLevelEnvVar = "LOG_LEVEL"

	ArgoCDReconciliationTImeOutEnvVar = "ARGOCD_RECONCILIATION_TIMEOUT"

	ArgoCDRemoveManagedByLabelOnDeletionEnvVar = "REMOVE_MANAGED_BY_LABEL_ON_ARGOCD_DELETION"
)
