package reposerver

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
)

func TestRepoServerReconciler_reconcileServiceMonitor(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sa := argocdcommon.MakeTestServiceAccount()

	resourceName = argocdcommon.TestArgoCDName

	tests := []struct {
		name        string
		setupClient func() *RepoServerReconciler
		wantErr     bool
	}{
		{
			name: "create a ServiceMonitor",
			setupClient: func() *RepoServerReconciler {
				return makeTestRepoServerReconciler(t, ns, sa)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rsr := tt.setupClient()
			err := rsr.reconcileServiceMonitor()
			if (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}
			currentService := &monitoringv1.ServiceMonitor{}
			err = rsr.Client.Get(context.TODO(), types.NamespacedName{Name: util.GenerateResourceName(rsr.Instance.Name, common.RepoServerMetrics), Namespace: argocdcommon.TestNamespace}, currentService)
			if err != nil {
				t.Fatalf("Could not get current Service: %v", err)
			}
			assert.Equal(t, common.ArgoCDMetrics, currentService.Spec.Endpoints[0].Port)
		})
	}
}

func TestRepoServerReconciler_DeleteServiceMonitor(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sa := argocdcommon.MakeTestServiceAccount()
	resourceName = argocdcommon.TestArgoCDName
	tests := []struct {
		name        string
		setupClient func() *RepoServerReconciler
		wantErr     bool
	}{
		{
			name: "successful delete",
			setupClient: func() *RepoServerReconciler {
				return makeTestRepoServerReconciler(t, ns, sa)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rsr := tt.setupClient()
			if err := rsr.deleteServiceMonitor(resourceName, ns.Name); (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
