package argocd

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	ActiveInstancesByPhase = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "active_argocd_instances_by_phase",
			Help: "Number of active argocd instances by phase",
		},
		[]string{"phase"},
	)

	ActiveInstancesTotal = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_argocd_instances_total",
			Help: "Total number of active argocd instances",
		},
	)

	// ReconcileTime is a prometheus metric which keeps track of the duration
	// of reconciliations for a given instance
	ReconcileTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "controller_runtime_reconcile_time_seconds",
		Help: "Length of time per reconciliation per instance",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.15, 0.2, 0.25, 0.3, 0.35, 0.4, 0.45, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0,
			1.25, 1.5, 1.75, 2.0, 2.5, 3.0, 3.5, 4.0, 4.5, 5, 6, 7, 8, 9, 10, 15, 20, 25, 30, 40, 50, 60},
	}, []string{"namespace"})
)

// StartMetricsServer starts a new HTTP server for metrics on given port
func StartMetricsServer(port int) chan error {
	errCh := make(chan error)
	go func() {
		sm := http.NewServeMux()
		sm.Handle("/metrics", promhttp.Handler())
		errCh <- http.ListenAndServe(fmt.Sprintf(":%d", port), sm)
	}()
	return errCh
}
