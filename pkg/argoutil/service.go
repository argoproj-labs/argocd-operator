package argoutil

import (
	"fmt"
)

// FqdnServiceRef will return the FQDN referencing a specific service name, as set up by the operator, with the
// given port.
func FqdnServiceRef(serviceName, namespace string, port int) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local:%d", serviceName, namespace, port)
}
