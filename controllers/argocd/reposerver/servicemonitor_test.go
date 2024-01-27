package reposerver

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
)

// func TestRepoServerReconciler_reconcileServiceMonitor(t *testing.T) {
// 	ns := argocdcommon.MakeTestNamespace()
// 	sa := argocdcommon.MakeTestServiceAccount()

// 	resourceName = argocdcommon.TestArgoCDName

// 	tests := []struct {
// 		name        string
// 		setupClient func() *RepoServerReconciler
// 		wantErr     bool
// 	}{
// 		{
// 			name: "create a ServiceMonitor",
// 			setupClient: func() *RepoServerReconciler {
// 				return makeTestRepoServerReconciler(t, ns, sa)
// 			},
// 			wantErr: false,
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			rsr := tt.setupClient()
// 			err := rsr.reconcileServiceMonitor()
// 			if (err != nil) != tt.wantErr {
// 				if tt.wantErr {
// 					t.Errorf("Expected error but did not get one")
// 				} else {
// 					t.Errorf("Unexpected error: %v", err)
// 				}
// 			}
// 			currentService := &monitoringv1.ServiceMonitor{}
// 			err = rsr.Client.Get(context.TODO(), types.NamespacedName{Name: argoutil.GenerateResourceName(rsr.Instance.Name, common.RepoServerMetrics), Namespace: argocdcommon.TestNamespace}, currentService)
// 			if err != nil {
// 				t.Fatalf("Could not get current Service: %v", err)
// 			}
// 			assert.Equal(t, common.ArgoCDMetrics, currentService.Spec.Endpoints[0].Port)
// 		})
// 	}
// }

func TestDeleteServiceMonitor(t *testing.T) {
	tests := []struct {
		name            string
		namespace       string
		reconciler      *RepoServerReconciler
		prometheusExist bool
		svcMonitorExist bool
		expectedError   bool
	}{
		{
			name:            "Prometheus API absent",
			namespace:       test.TestNamespace,
			prometheusExist: false,
			svcMonitorExist: true,
			expectedError:   false,
		},
		{
			name:            "Prometheus API present, ServiceMonitor exists",
			namespace:       test.TestNamespace,
			prometheusExist: true,
			svcMonitorExist: true,
			expectedError:   false,
		},
		{
			name:            "Prometheus API present, ServiceMonitor not found",
			namespace:       test.TestNamespace,
			prometheusExist: true,
			svcMonitorExist: false,
			expectedError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rsr.deleteServiceMonitor("test-svc-monitor", tt.namespace)

			if tt.expectedError {
				assert.Error(t, err, "Expected an error but got none.")
			} else {
				assert.NoError(t, err, "Expected no error but got one.")
			}
		})
	}
}
