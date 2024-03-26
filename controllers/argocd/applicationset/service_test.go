package applicationset

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/tests/test"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestApplicationSetReconciler_reconcileService(t *testing.T) {
	ns := test.MakeTestNamespace(nil)
	sa := test.MakeTestServiceAccount()
	resourceName = test.TestArgoCDName

	tests := []struct {
		name        string
		setupClient func() *ApplicationSetReconciler
		wantErr     bool
	}{
		{
			name: "create a Service",
			setupClient: func() *ApplicationSetReconciler {
				return makeTestApplicationSetReconciler(t, false, ns, sa)
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
			err = nr.Client.Get(context.TODO(), types.NamespacedName{Name: test.TestArgoCDName, Namespace: test.TestNamespace}, currentService)
			if err != nil {
				t.Fatalf("Could not get current Service: %v", err)
			}
			// assert.Equal(t, GetServiceSpec().Ports, currentService.Spec.Ports)
		})
	}
}

func TestApplicationSetReconciler_DeleteService(t *testing.T) {
	ns := test.MakeTestNamespace(nil)
	sa := test.MakeTestServiceAccount()
	resourceName = test.TestArgoCDName
	tests := []struct {
		name        string
		setupClient func() *ApplicationSetReconciler
		wantErr     bool
	}{
		{
			name: "successful delete",
			setupClient: func() *ApplicationSetReconciler {
				return makeTestApplicationSetReconciler(t, false, ns, sa)
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
