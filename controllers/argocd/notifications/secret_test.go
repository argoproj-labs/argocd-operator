package notifications

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/tests/test"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestNotificationsReconciler_reconcileSecret(t *testing.T) {
	ns := test.MakeTestNamespace()
	resourceLabels = testExpectedLabels
	tests := []struct {
		name        string
		setupClient func() *NotificationsReconciler
		wantErr     bool
	}{
		{
			name: "create a secret",
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, ns)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			err := nr.reconcileSecret()
			if (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			currentSecret := &corev1.Secret{}
			err = nr.Client.Get(context.TODO(), types.NamespacedName{Name: common.NotificationsSecretName, Namespace: test.TestNamespace}, currentSecret)
			if err != nil {
				t.Fatalf("Could not get current Secret: %v", err)
			}
			assert.Equal(t, testExpectedLabels, currentSecret.ObjectMeta.Labels)
		})
	}
}

func TestNotificationsReconciler_DeleteSecret(t *testing.T) {
	ns := test.MakeTestNamespace()
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
			if err := nr.deleteSecret(ns.Name); (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
