package common

// names
const (
	// ArgoCDExportName is the export name for labels.
	ArgoCDExportName = "argocd.export"
)

// defaults
const (
	// ArgoCDDefaultExportJobImage is the export job container image to use when not specified.
	ArgoCDDefaultExportJobImage = "quay.io/argoprojlabs/argocd-operator-util"

	// ArgoCDDefaultExportJobVersion is the export job container image tag to use when not specified.
	ArgoCDDefaultExportJobVersion = "sha256:6f80965a2bef1c80875be0995b18d9be5a6ad4af841cbc170ed3c60101a7deb2" // 0.5.0

	// ArgoCDDefaultExportLocalCapicity is the default capacity to use for local export.
	ArgoCDDefaultExportLocalCapicity = "2Gi"
)
