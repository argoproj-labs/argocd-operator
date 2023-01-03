package argocd

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// workloadState is a named int type that is used to characterize the state (unknown/pending/running/available) a workload is in
type workloadState int

// numerical representation of workloadStates to encode this information into prometheus
const (
	WorkloadUnknownState   workloadState = 0
	WorkloadFailedState    workloadState = 1
	WorkloadPendingState   workloadState = 2
	WorkloadRunningState   workloadState = 3
	WorkloadAvailableState workloadState = 4

	// MetricsPath is the endpoint to collect instance level metrics
	MetricsPath = "/metrics"
)

type WorkloadStatusTrackerMetrics struct {
	applicationControllerStatus    *prometheus.GaugeVec
	applicationSetControllerStatus *prometheus.GaugeVec
	dexStatus                      *prometheus.GaugeVec
	notificationsControllerStatus  *prometheus.GaugeVec
	serverStatus                   *prometheus.GaugeVec
	repoServerStatus               *prometheus.GaugeVec
	redisStatus                    *prometheus.GaugeVec
	argoCDPhase                    *prometheus.GaugeVec
}

var instanceStatusTracker *WorkloadStatusTrackerMetrics

// StartMetricsServer starts a new HTTP server for metrics on given port
func StartMetricsServer(port int) chan error {
	errCh := make(chan error)
	go func() {
		sm := http.NewServeMux()
		sm.Handle(MetricsPath, promhttp.Handler())
		errCh <- http.ListenAndServe(fmt.Sprintf(":%d", port), sm)
	}()
	return errCh
}

// NewWorkloadStatusTrackerMetrics returns a new WorkloadStatustrackerMetrics object
func NewWorkloadStatusTrackerMetrics() *WorkloadStatusTrackerMetrics {
	workloadStatusTrackers := &WorkloadStatusTrackerMetrics{}

	workloadStatusTrackers.applicationControllerStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "argocd_application_controller_status",
		Help: "Describes the status of the application controller workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']",
	}, []string{"namespace"})

	workloadStatusTrackers.applicationSetControllerStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "argocd_applicationset_controller_status",
		Help: "Describes the status of the applicationSet controller workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']",
	}, []string{"namespace"})

	workloadStatusTrackers.dexStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "argocd_dex_status",
		Help: "Describes the status of the dex workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']",
	}, []string{"namespace"})

	workloadStatusTrackers.notificationsControllerStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "argocd_notifications_controller_status",
		Help: "Describes the status of the notifications controller workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']",
	}, []string{"namespace"})

	workloadStatusTrackers.serverStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "argocd_server_status",
		Help: "Describes the status of the argo-cd server workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']",
	}, []string{"namespace"})

	workloadStatusTrackers.repoServerStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "argocd_repo_server_status",
		Help: "Describes the status of the repo server workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']",
	}, []string{"namespace"})

	workloadStatusTrackers.redisStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "argocd_redis_status",
		Help: "Describes the status of the redis workload [0='Unknown', 1='Failed', 2='Pending', 3='Running']",
	}, []string{"namespace"})

	workloadStatusTrackers.argoCDPhase = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "argocd_phase",
		Help: "Describes the phase of argo-cd instance [2='Pending', 4='Available']",
	}, []string{"namespace"})

	return workloadStatusTrackers
}

// InstanceStatus returns the global instance status tracker object. This stores the various workload status metrics partitioned by instance namespace
func InstanceStatusTracker() *WorkloadStatusTrackerMetrics {
	return instanceStatusTracker
}

// SetApplicationControllerStatus sets the workloadStatus of the application controller workload for the instance present in the supplied namespace
func (wstm *WorkloadStatusTrackerMetrics) SetApplicationControllerStatus(namespace string, state workloadState) {
	wstm.applicationControllerStatus.WithLabelValues(namespace).Set(float64(state))
}

// SetApplicationSetControllerStatus sets the workloadStatus of the applicationSet controller workload for the instance present in the supplied namespace
func (wstm *WorkloadStatusTrackerMetrics) SetApplicationSetControllerStatus(namespace string, state workloadState) {
	wstm.applicationSetControllerStatus.WithLabelValues(namespace).Set(float64(state))
}

// SetDexStatus sets the workloadStatus of the dex workload for the instance present in the supplied namespace
func (wstm *WorkloadStatusTrackerMetrics) SetDexStatus(namespace string, state workloadState) {
	wstm.dexStatus.WithLabelValues(namespace).Set(float64(state))
}

// SetNotificationsControllerStatus sets the workloadStatus of the notifications controller workload for the instance present in the supplied namespace
func (wstm *WorkloadStatusTrackerMetrics) SetNotificationsControllerStatus(namespace string, state workloadState) {
	wstm.notificationsControllerStatus.WithLabelValues(namespace).Set(float64(state))
}

// SetServerStatus sets the workloadStatus of the server workload for the instance present in the supplied namespace
func (wstm *WorkloadStatusTrackerMetrics) SetServerStatus(namespace string, state workloadState) {
	wstm.serverStatus.WithLabelValues(namespace).Set(float64(state))
}

// SetRepoServerStatus sets the workloadStatus of the repo-server controller workload for the instance present in the supplied namespace
func (wstm *WorkloadStatusTrackerMetrics) SetRepoServerStatus(namespace string, state workloadState) {
	wstm.repoServerStatus.WithLabelValues(namespace).Set(float64(state))
}

// SetRedisStatus sets the workloadStatus of the application controller workload for the instance present in the supplied namespace
func (wstm *WorkloadStatusTrackerMetrics) SetRedisStatus(namespace string, state workloadState) {
	wstm.redisStatus.WithLabelValues(namespace).Set(float64(state))
}

// SetArgoCDPhase sets the phase of the instance present in the supplied namespace
func (wstm *WorkloadStatusTrackerMetrics) SetArgoCDPhase(namespace string, state workloadState) {
	wstm.argoCDPhase.WithLabelValues(namespace).Set(float64(state))
}

func init() {
	instanceStatusTracker = NewWorkloadStatusTrackerMetrics()
}
