package notifications

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestNotificationsReconciler_reconcileConfigMap(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()

	tests := []struct {
		name        string
		setupClient func() *NotificationsReconciler
		wantErr     bool
	}{
		{
			name: "create a configMap",
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, ns)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			err := nr.reconcileConfigMap()
			if (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			currentConfigMap := &corev1.ConfigMap{}
			err = nr.Client.Get(context.TODO(), types.NamespacedName{Name: common.NotificationsConfigMapName, Namespace: argocdcommon.TestNamespace}, currentConfigMap)
			if err != nil {
				t.Fatalf("Could not get current ConfigMap: %v", err)
			}
			assert.Equal(t, GetDefaultNotificationsConfig(), currentConfigMap.Data)
		})
	}
}

func TestNotificationsReconciler_DeleteConfigMap(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
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
			if err := nr.deleteConfigMap(ns.Name); (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
