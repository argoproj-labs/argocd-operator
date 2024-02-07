package common

// keys
const (
	// ArgoCDKeyPrometheus is the resource prometheus key for labels.
	ArgoCDKeyPrometheus = "prometheus"

	// PrometheusReleaseKey is the prometheus release key for labels.
	PrometheusReleaseKey = "release"
)

// defaults
const (
	// ArgoCDDefaultPrometheusReplicas is the default Prometheus replica count.
	ArgoCDDefaultPrometheusReplicas = int32(1)
)
