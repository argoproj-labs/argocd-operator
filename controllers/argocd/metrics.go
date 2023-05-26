package argocd

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	ActiveInstances = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "active_argocd_instances",
			Help: "Number of active argocd instances",
		},
		[]string{"phase"},
	)
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

// func init() {

// 	// opsQueued := prometheus.NewGaugeVec(
// 	// 	prometheus.GaugeOpts{
// 	// 		Namespace: "our_company",
// 	// 		Subsystem: "blob_storage",
// 	// 		Name:      "ops_queued",
// 	// 		Help:      "Number of blob storage operations waiting to be processed, partitioned by user and type.",
// 	// 	},
// 	// 	[]string{
// 	// 		// Which user has requested the operation?
// 	// 		"user",
// 	// 		// Of what type is the operation?
// 	// 		"type",
// 	// 	},
// 	// )
// 	err := prometheus.Register(ActiveInstances)
// 	if err != nil && err.Error() != "duplicate metrics collector registration attempted" {
// 		//do nothing
// 	}

// 	// // Increase a value using compact (but order-sensitive!) WithLabelValues().
// 	// opsQueued.WithLabelValues("bob", "put").Add(4)
// 	// // Increase a value with a map using WithLabels. More verbose, but order
// 	// // doesn't matter anymore.
// 	// opsQueued.With(prometheus.Labels{"type": "delete", "user": "alice"}).Inc()

// 	// // Register custom metrics with the global prometheus registry
// 	// prometheus.MustRegister(ActiveInstances)

// 	// ActiveInstances.WithLabelValues("test").Add(1)
// }
