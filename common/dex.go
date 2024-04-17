package common

// names
const (
	// DexServerComponent is the name of the Dex server control plane component
	DexServerComponent = "dex-server"

	// DefaultDexServiceAccountName is the default Service Account name for the Dex server.
	DefaultDexServiceAccountName = "argocd-dex-server"

	// DexComponent is the dex control plane component
	DexComponent = "dex-server"
)

// suffix
const (
	DexSuffix = "dex-server"
)

// keys
const (
	// KeyDexConfig is the key for dex configuration.
	KeyDexConfig = "dex.config"

	DexConfigChangedKey = "dex.config.changed"
)

// defaults
const (

	// DefaultDexConfig is the default dex configuration.
	DefaultDexConfig = ""

	// DefaultDexImage is the Dex container image to use when not specified.
	DefaultDexImage = "ghcr.io/dexidp/dex"

	// DefaultDexOAuthRedirectPath is the default path to use for the OAuth Redirect URI.
	DefaultDexOAuthRedirectPath = "/api/dex/callback"

	// DefaultDexGRPCPort is the default GRPC listen port for Dex.
	DefaultDexGRPCPort = 5557

	// DefaultDexHTTPPort is the default HTTP listen port for Dex.
	DefaultDexHTTPPort = 5556

	// DefaultDexMetricsPort is the default Metrics listen port for Dex.
	DefaultDexMetricsPort = 5558

	// DefaultDexVersion is the Dex container image tag to use when not specified.
	DefaultDexVersion = "sha256:d5f887574312f606c61e7e188cfb11ddb33ff3bf4bd9f06e6b1458efca75f604" // v2.30.3
)
