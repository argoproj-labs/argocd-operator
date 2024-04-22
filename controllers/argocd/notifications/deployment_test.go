package notifications

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/tests/test"
)

// func TestNotificationsReconciler_reconcileDeployment(t *testing.T) {
// 	resourceName = test.TestArgoCDName

// 	ns := test.MakeTestNamespace(nil)
// 	nr := makeTestNotificationsReconciler(t, ns)

// 	existingDeployment := nr.getdesired()

// 	tests := []struct {
// 		name        string
// 		setupClient func() *NotificationsReconciler
// 		wantErr     bool
// 	}{
// 		{
// 			name: "create a deployment",
// 			setupClient: func() *NotificationsReconciler {
// 				return makeTestNotificationsReconciler(t, ns)
// 			},
// 			wantErr: false,
// 		},
// 		{
// 			name: "update a deployment",
// 			setupClient: func() *NotificationsReconciler {
// 				outdatedDeployment := existingDeployment
// 				outdatedDeployment.ObjectMeta.Labels = test.TestKVP
// 				return makeTestNotificationsReconciler(t, outdatedDeployment, ns)
// 			},
// 			wantErr: false,
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			nr := tt.setupClient()
// 			err := nr.reconcileDeployment()
// 			if (err != nil) != tt.wantErr {
// 				if tt.wantErr {
// 					t.Errorf("Expected error but did not get one")
// 				} else {
// 					t.Errorf("Unexpected error: %v", err)
// 				}
// 			}

// 			updatedDeployment := &appsv1.Deployment{}
// 			err = nr.Client.Get(context.TODO(), types.NamespacedName{Name: resourceName, Namespace: test.TestNamespace}, updatedDeployment)
// 			if err != nil {
// 				t.Fatalf("Could not get updated Deployment: %v", err)
// 			}
// 			assert.Equal(t, testExpectedLabels, updatedDeployment.ObjectMeta.Labels)
// 		})
// 	}
// }

func TestNotificationsReconciler_DeleteDeployment(t *testing.T) {
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
			if err := nr.deleteDeployment(resourceName, ns.Name); (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
