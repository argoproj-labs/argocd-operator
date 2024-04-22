package notifications

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/tests/test"
)

// func TestNotificationsReconciler_reconcileRole(t *testing.T) {
// 	ns := test.MakeTestNamespace(nil)
// 	resourceName = test.TestArgoCDName
// 	existingRole := &rbacv1.Role{
// 		TypeMeta: metav1.TypeMeta{
// 			Kind:       common.RoleKind,
// 			APIVersion: common.APIGroupVersionRbacV1,
// 		},
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      test.TestArgoCDName,
// 			Namespace: test.TestNamespace,
// 		},
// 		Rules: getPolicyRules(),
// 	}

// 	tests := []struct {
// 		name        string
// 		setupClient func() *NotificationsReconciler
// 		wantErr     bool
// 	}{
// 		{
// 			name: "create a role",
// 			setupClient: func() *NotificationsReconciler {
// 				return makeTestNotificationsReconciler(t, ns)
// 			},
// 			wantErr: false,
// 		},
// 		{
// 			name: "Update a role",
// 			setupClient: func() *NotificationsReconciler {
// 				outdatedRole := existingRole
// 				outdatedRole.Rules = []rbacv1.PolicyRule{}
// 				return makeTestNotificationsReconciler(t, outdatedRole, ns)
// 			},
// 			wantErr: false,
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			nr := tt.setupClient()
// 			err := nr.reconcileRole()
// 			if (err != nil) != tt.wantErr {
// 				if tt.wantErr {
// 					t.Errorf("Expected error but did not get one")
// 				} else {
// 					t.Errorf("Unexpected error: %v", err)
// 				}
// 			}

// 			updatedRole := &rbacv1.Role{}
// 			err = nr.Client.Get(context.TODO(), types.NamespacedName{Name: test.TestArgoCDName, Namespace: test.TestNamespace}, updatedRole)
// 			if err != nil {
// 				t.Fatalf("Could not get updated Role: %v", err)
// 			}
// 			assert.Equal(t, getPolicyRules(), updatedRole.Rules)
// 		})
// 	}
// }

func TestNotificationsReconciler_DeleteRole(t *testing.T) {
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
			if err := nr.deleteRole(resourceName, ns.Name); (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
