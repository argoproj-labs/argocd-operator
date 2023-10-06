package reposerver

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestRepoServerReconciler_reconcileTLSService(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sa := argocdcommon.MakeTestServiceAccount()
	resourceName = argocdcommon.TestArgoCDName

	tests := []struct {
		name        string
		setupClient func() *RepoServerReconciler
		wantErr     bool
	}{
		{
			name: "create a Service",
			setupClient: func() *RepoServerReconciler {
				return makeTestRepoServerReconciler(t, ns, sa)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			err := nr.reconcileService()
			if (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}
			currentService := &corev1.Service{}
			err = nr.Client.Get(context.TODO(), types.NamespacedName{Name: argocdcommon.TestArgoCDName, Namespace: argocdcommon.TestNamespace}, currentService)
			if err != nil {
				t.Fatalf("Could not get current Service: %v", err)
			}
			assert.Equal(t, GetServiceSpec().Ports, currentService.Spec.Ports)
		})
	}
}

func TestRepoServerReconciler_DeleteService(t *testing.T) {
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
			nr := tt.setupClient()
			if err := nr.deleteService(resourceName, ns.Name); (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
