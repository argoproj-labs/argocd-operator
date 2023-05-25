package argocd

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	ActiveInstances = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "active_argocd_instances",
			Help: "Number of active argocd instances",
		},
		[]string{"state"},
	)
)

func init() {

	opsQueued := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "our_company",
			Subsystem: "blob_storage",
			Name:      "ops_queued",
			Help:      "Number of blob storage operations waiting to be processed, partitioned by user and type.",
		},
		[]string{
			// Which user has requested the operation?
			"user",
			// Of what type is the operation?
			"type",
		},
	)
	prometheus.MustRegister(opsQueued)

	// Increase a value using compact (but order-sensitive!) WithLabelValues().
	opsQueued.WithLabelValues("bob", "put").Add(4)
	// Increase a value with a map using WithLabels. More verbose, but order
	// doesn't matter anymore.
	opsQueued.With(prometheus.Labels{"type": "delete", "user": "alice"}).Inc()

	// Register custom metrics with the global prometheus registry
	prometheus.MustRegister(ActiveInstances)

	ActiveInstances.WithLabelValues("test").Add(1)
}
