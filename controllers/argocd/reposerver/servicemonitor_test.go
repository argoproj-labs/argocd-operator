package reposerver

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/pkg/monitoring"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeleteServiceMonitor(t *testing.T) {
	tests := []struct {
		name                   string
		reconciler             *RepoServerReconciler
		prometheusAPIAvailable bool
		svcMonitorExist        bool
		expectedError          bool
	}{
		{
			name: "Prometheus API absent",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
			),
			prometheusAPIAvailable: false,
			svcMonitorExist:        false,
			expectedError:          false,
		},
		{
			name: "ServiceMonitor not found",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
			),
			prometheusAPIAvailable: true,
			svcMonitorExist:        false,
			expectedError:          false,
		},
		{
			name: "ServiceMonitor exists",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestServiceMonitor(nil),
			),
			prometheusAPIAvailable: true,
			svcMonitorExist:        true,
			expectedError:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitoring.SetPrometheusAPIFound(tt.prometheusAPIAvailable)
			defer monitoring.SetPrometheusAPIFound(false)

			err := tt.reconciler.deleteServiceMonitor(test.TestName, test.TestNamespace)

			if tt.svcMonitorExist {
				_, err := monitoring.GetServiceMonitor(test.TestName, test.TestNamespace, tt.reconciler.Client)
				assert.True(t, apierrors.IsNotFound(err))
			}

			if tt.expectedError {
				assert.Error(t, err, "Expected an error but got none.")
			} else {
				assert.NoError(t, err, "Expected no error but got one.")
			}
		})
	}
}

func getDesiredSvcMonitor() *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-argocd-repo-server-metrics",
			Namespace: "test-ns",
			Labels: map[string]string{
				"app.kubernetes.io/name":       "test-argocd-repo-server-metrics",
				"app.kubernetes.io/part-of":    "argocd",
				"app.kubernetes.io/instance":   "test-argocd",
				"app.kubernetes.io/managed-by": "argocd-operator",
				"app.kubernetes.io/component":  "repo-server",
				"release":                      "prometheus-operator",
			},
			Annotations: map[string]string{
				"argocds.argoproj.io/name":      "test-argocd",
				"argocds.argoproj.io/namespace": "test-ns",
			},
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "test-argocd-repo-server",
				},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port: "metrics",
				},
			},
		},
	}
}
