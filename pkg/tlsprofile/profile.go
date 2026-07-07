package tlsprofile

import configv1 "github.com/openshift/api/config/v1"

type TLSConfigProfile struct {
	DisableClusterTLSProfile bool
	// MinVersion specifies the minimum TLS version configured in cluster.
	MinVersion configv1.TLSProtocolVersion
	// Ciphers specifies the list of supported TLS cipher suites in cluster.
	Ciphers []string
}
