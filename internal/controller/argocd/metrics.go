package argocd

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	ActiveInstancesByPhase = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "active_argocd_instances_by_phase",
			Help: "Number of active argocd instances by phase",
		},
		[]string{"phase"},
	)

	ActiveInstancesTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_argocd_instances_total",
			Help: "Total number of active argocd instances",
		},
	)

	ActiveInstanceReconciliationCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "active_argocd_instance_reconciliation_count",
			Help: "Number of reconciliations performed for a given instance",
		},
		[]string{"namespace"},
	)

	// ReconcileTime is a prometheus metric which keeps track of the duration
	// of reconciliations for a given instance
	ReconcileTime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "controller_runtime_reconcile_time_seconds_per_instance",
		Help:    "Length of time per reconciliation per instance",
		Buckets: []float64{0.05, 0.075, 0.1, 0.15, 0.2, 0.22, 0.24, 0.26, 0.28, 0.3, 0.32, 0.34, 0.37, 0.4, 0.42, 0.44, 0.48, 0.5, 0.55, 0.6, 0.75, 0.9, 1.00},
	}, []string{"namespace"})
)

func init() {
	metrics.Registry.MustRegister(ActiveInstancesTotal, ActiveInstancesByPhase, ActiveInstanceReconciliationCount, ReconcileTime)
}
