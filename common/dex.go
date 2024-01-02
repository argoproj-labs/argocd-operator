package common

// names
const (
	// ArgoCDDexServerComponent is the name of the Dex server control plane component
	ArgoCDDexServerComponent = "argocd-dex-server"

	// ArgoCDDefaultDexServiceAccountName is the default Service Account name for the Dex server.
	ArgoCDDefaultDexServiceAccountName = "argocd-dex-server"
)

// keys
const (
	// ArgoCDKeyDexConfig is the key for dex configuration.
	ArgoCDKeyDexConfig = "dex.config"
)

// defaults
const (

	// ArgoCDDefaultDexConfig is the default dex configuration.
	ArgoCDDefaultDexConfig = ""

	// ArgoCDDefaultDexImage is the Dex container image to use when not specified.
	ArgoCDDefaultDexImage = "ghcr.io/dexidp/dex"

	// ArgoCDDefaultDexOAuthRedirectPath is the default path to use for the OAuth Redirect URI.
	ArgoCDDefaultDexOAuthRedirectPath = "/api/dex/callback"

	// ArgoCDDefaultDexGRPCPort is the default GRPC listen port for Dex.
	ArgoCDDefaultDexGRPCPort = 5557

	// ArgoCDDefaultDexHTTPPort is the default HTTP listen port for Dex.
	ArgoCDDefaultDexHTTPPort = 5556

	// ArgoCDDefaultDexMetricsPort is the default Metrics listen port for Dex.
	ArgoCDDefaultDexMetricsPort = 5558

	// ArgoCDDefaultDexVersion is the Dex container image tag to use when not specified.
	ArgoCDDefaultDexVersion = "sha256:d5f887574312f606c61e7e188cfb11ddb33ff3bf4bd9f06e6b1458efca75f604" // v2.30.3
)
