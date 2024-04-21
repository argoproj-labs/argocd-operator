package notifications

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestNotificationsReconciler_reconcileServiceAccount(t *testing.T) {
	ns := test.MakeTestNamespace(nil)
	resourceName = test.TestArgoCDName

	tests := []struct {
		name        string
		setupClient func() *NotificationsReconciler
		wantErr     bool
	}{
		{
			name: "create a serviceAccount",
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, ns)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			err := nr.reconcileServiceAccount()
			if (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			currentServiceAccount := &corev1.ServiceAccount{}
			err = nr.Client.Get(context.TODO(), types.NamespacedName{Name: test.TestArgoCDName, Namespace: test.TestNamespace}, currentServiceAccount)
			if err != nil {
				t.Fatalf("Could not get current ServiceAccount: %v", err)
			}
			assert.Equal(t, testExpectedLabels, currentServiceAccount.Labels)
		})
	}
}

func TestNotificationsReconciler_DeleteServiceAccount(t *testing.T) {
	ns := test.MakeTestNamespace(nil)
	resourceName = test.TestArgoCDName
	tests := []struct {
		name        string
		setupClient func() *NotificationsReconciler
		wantErr     bool
	}{
		{
			name: "successful delete",
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, ns)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			if err := nr.deleteServiceAccount(resourceName, ns.Name); (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
