package monitoring

import (
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
)

var prometheusAPIFound = false

// IsPrometheusAPIAvailable returns true if the Prometheus API is present.
func IsPrometheusAPIAvailable() bool {
	return prometheusAPIFound
}

// SetPrometheusAPIFound sets the value of prometheusAPIFound to provided input
func SetPrometheusAPIFound(found bool) {
	prometheusAPIFound = found
}

// VerifyPrometheusAPI will verify that the Prometheus API is present.
func VerifyPrometheusAPI() error {
	found, err := argoutil.VerifyAPI(monitoringv1.SchemeGroupVersion.Group, monitoringv1.SchemeGroupVersion.Version)
	if err != nil {
		return err
	}
	prometheusAPIFound = found
	return nil
}
