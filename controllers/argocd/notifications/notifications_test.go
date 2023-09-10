package notifications

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
)

func TestNotificationsReconciler_Reconcile(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	tests := []struct {
		name         string
		resourceName string
		setupClient  func() *NotificationsReconciler
		wantErr      bool
	}{
		{
			name:         "successful reconcile",
			resourceName: argocdcommon.TestArgoCDName,
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, ns)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		nr := tt.setupClient()
		originalResourceName := resourceName
		resourceName = argocdcommon.TestArgoCDName
		err := nr.Reconcile()
		assert.NoError(t, err)
		if (err != nil) != tt.wantErr {
			t.Errorf("NotificationsReconciler.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
		}
		resourceName = originalResourceName
	}
}

func TestNotificationsReconciler_DeleteResources(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		setupClient  func() *NotificationsReconciler
		wantErr      bool
	}{
		{
			name:         "successful delete",
			resourceName: argocdcommon.TestArgoCDName,
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			originalResourceName := resourceName
			resourceName = argocdcommon.TestArgoCDName
			if err := nr.DeleteResources(); (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.DeleteResources() error = %v, wantErr %v", err, tt.wantErr)
			}
			resourceName = originalResourceName
		})
	}
}
