package notifications

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestNotificationsReconciler_reconcileDeployment(t *testing.T) {
	resourceName = argocdcommon.TestArgoCDName
	resourceLabels = testExpectedLabels
	ns := argocdcommon.MakeTestNamespace()
	nr := makeTestNotificationsReconciler(t, ns)

	existingDeployment := nr.getDesiredDeployment()

	tests := []struct {
		name        string
		setupClient func() *NotificationsReconciler
		wantErr     bool
	}{
		{
			name: "create a deployment",
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, ns)
			},
			wantErr: false,
		},
		{
			name: "update a deployment",
			setupClient: func() *NotificationsReconciler {
				outdatedDeployment := existingDeployment
				outdatedDeployment.ObjectMeta.Labels = argocdcommon.TestKVP
				return makeTestNotificationsReconciler(t, outdatedDeployment, ns)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			err := nr.reconcileDeployment()
			if (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			updatedDeployment := &appsv1.Deployment{}
			err = nr.Client.Get(context.TODO(), types.NamespacedName{Name: resourceName, Namespace: argocdcommon.TestNamespace}, updatedDeployment)
			if err != nil {
				t.Fatalf("Could not get updated Deployment: %v", err)
			}
			assert.Equal(t, testExpectedLabels, updatedDeployment.ObjectMeta.Labels)
		})
	}
}

func TestNotificationsReconciler_DeleteDeployment(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	resourceName = argocdcommon.TestArgoCDName
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
			if err := nr.DeleteDeployment(resourceName, ns.Name); (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
