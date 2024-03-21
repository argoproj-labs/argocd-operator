package common

// names
const (
	ServerController = "server-controller"

	// ServerComponent is the name of the Argo CD server control plane component
	ServerComponent = "server"

	//ArgoCDServerName = "argocd-server"

	// ArgoCDServerTLSSecretName is the name of the TLS secret for the argocd-server
	ArgoCDServerTLSSecretName = "argocd-server-tls"
)

// suffixes
const (
	// ArgoCDServerSuffix is the name suffix for ArgoCD Server RBAC resources.
	//ArgoCDServerSuffix = "argocd-server"

	// ServerSuffix is the name suffix for ArgoCD Server resources.
	ServerSuffix = "server"
)

// values
const (
	// VolumeMountPathArgoCDServerTLS is the path to mount argocd-server tls certs
	VolumeMountPathArgoCDServerTLS = "/app/config/server/tls"

	// VolumeMountPathRedisServerTLS is the path to mount redis tls certs
	VolumeMountPathRedisServerTLS = "/app/config/server/tls/redis"

	ServerMetricsPort = 8083

	ServerPort = 8080
)

// defaults
const (

	// ArgoCDDefaultServerOperationProcessors is the number of ArgoCD Server Operation Processors to use when not specified.
	ArgoCDDefaultServerOperationProcessors = int32(10)

	// ArgoCDDefaultServerStatusProcessors is the number of ArgoCD Server Status Processors to use when not specified.
	ArgoCDDefaultServerStatusProcessors = int32(20)

	// ArgoCDDefaultServerResourceLimitCPU is the default CPU limit when not specified for the Argo CD server contianer.
	ArgoCDDefaultServerResourceLimitCPU = "1000m"

	// ArgoCDDefaultServerResourceLimitMemory is the default memory limit when not specified for the Argo CD server contianer.
	ArgoCDDefaultServerResourceLimitMemory = "128Mi"

	// ArgoCDDefaultServerResourceRequestCPU is the default CPU requested when not specified for the Argo CD server contianer.
	ArgoCDDefaultServerResourceRequestCPU = "250m"

	// ArgoCDDefaultServerResourceRequestMemory is the default memory requested when not specified for the Argo CD server contianer.
	ArgoCDDefaultServerResourceRequestMemory = "64Mi"

	// ArgoCDDefaultServerSessionKeyLength is the length of the generated default server signature key.
	ArgoCDDefaultServerSessionKeyLength = 20

	// ArgoCDDefaultServerSessionKeyNumDigits is the number of digits to use for the generated default server signature key.
	ArgoCDDefaultServerSessionKeyNumDigits = 5

	// ArgoCDDefaultServerSessionKeyNumSymbols is the number of symbols to use for the generated default server signature key.
	ArgoCDDefaultServerSessionKeyNumSymbols = 0
)
