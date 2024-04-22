package notifications

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/tests/test"
)

// func TestNotificationsReconciler_reconcileRoleBinding(t *testing.T) {
// 	resourceName = test.TestArgoCDName
// 	ns := test.MakeTestNamespace(nil)
// 	sa := test.MakeTestServiceAccount(func(sa *corev1.ServiceAccount) {
// 		sa.Name = resourceName
// 	})

// 	tests := []struct {
// 		name        string
// 		setupClient func() *NotificationsReconciler
// 		wantErr     bool
// 	}{
// 		{
// 			name: "create a rolebinding",
// 			setupClient: func() *NotificationsReconciler {
// 				return makeTestNotificationsReconciler(t, ns, sa)
// 			},
// 			wantErr: false,
// 		},
// 		{
// 			name: "update a rolebinding",
// 			setupClient: func() *NotificationsReconciler {
// 				outdatedRoleBinding := &rbacv1.RoleBinding{
// 					TypeMeta: metav1.TypeMeta{
// 						Kind:       common.RoleBindingKind,
// 						APIVersion: common.APIGroupVersionRbacV1,
// 					},
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      test.TestArgoCDName,
// 						Namespace: test.TestNamespace,
// 					},
// 					RoleRef:  rbacv1.RoleRef{},
// 					Subjects: []rbacv1.Subject{},
// 				}
// 				return makeTestNotificationsReconciler(t, outdatedRoleBinding, ns, sa)
// 			},
// 			wantErr: false,
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			nr := tt.setupClient()
// 			err := nr.reconcileRoleBinding()
// 			if (err != nil) != tt.wantErr {
// 				if tt.wantErr {
// 					t.Errorf("Expected error but did not get one")
// 				} else {
// 					t.Errorf("Unexpected error: %v", err)
// 				}
// 			}
// 			updatedRoleBinding := &rbacv1.RoleBinding{}
// 			err = nr.Client.Get(context.TODO(), types.NamespacedName{Name: test.TestArgoCDName, Namespace: test.TestNamespace}, updatedRoleBinding)
// 			if err != nil {
// 				t.Fatalf("Could not get updated RoleBinding: %v", err)
// 			}
// 			assert.Equal(t, test.MakeTestRoleRef(resourceName), updatedRoleBinding.RoleRef)
// 			assert.Equal(t, test.MakeTestSubjects(types.NamespacedName{Name: resourceName, Namespace: test.TestNamespace}), updatedRoleBinding.Subjects)
// 		})
// 	}
// }

func TestNotificationsReconciler_DeleteRoleBinding(t *testing.T) {
	ns := test.MakeTestNamespace(nil)
	sa := test.MakeTestServiceAccount()
	resourceName = test.TestArgoCDName
	tests := []struct {
		name        string
		setupClient func() *NotificationsReconciler
		wantErr     bool
	}{
		{
			name: "successful delete",
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, ns, sa)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			if err := nr.deleteRoleBinding(resourceName, ns.Name); (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
