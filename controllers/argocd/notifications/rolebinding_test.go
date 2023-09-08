package notifications

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestNotificationsReconciler_reconcileRoleBinding(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sa := argocdcommon.MakeTestServiceAccount()
	existingRoleBinding := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       RoleBindingKind,
			APIVersion: APIVersionRbacV1,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      argocdcommon.TestArgoCDName,
			Namespace: argocdcommon.TestNamespace,
		},
		RoleRef:  testRoleRef,
		Subjects: testSubjects,
	}

	tests := []struct {
		name         string
		resourceName string
		setupClient  func() *NotificationsReconciler
		wantErr      bool
	}{
		{
			name:         "rolebinding doesn't exist",
			resourceName: argocdcommon.TestArgoCDName,
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, ns, sa)
			},
			wantErr: false,
		},
		{
			name:         "rolebinding exists and is correct",
			resourceName: argocdcommon.TestArgoCDName,
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, existingRoleBinding, ns, sa)
			},
			wantErr: false,
		},
		{
			name:         "rolebinding exists but outdated",
			resourceName: argocdcommon.TestArgoCDName,
			setupClient: func() *NotificationsReconciler {
				outdatedRoleBinding := existingRoleBinding
				outdatedRoleBinding.RoleRef = rbacv1.RoleRef{}
				outdatedRoleBinding.Subjects = []rbacv1.Subject{}
				return makeTestNotificationsReconciler(t, outdatedRoleBinding, ns, sa)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			originalResourceName := resourceName
			resourceName = argocdcommon.TestArgoCDName
			err := nr.reconcileRoleBinding()
			if (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.reconcileRoleBinding() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.name == "rolebinding exists and is correct" {
				updatedRoleBinding := &rbacv1.RoleBinding{}
				err := nr.Client.Get(context.TODO(), types.NamespacedName{Name: argocdcommon.TestArgoCDName, Namespace: argocdcommon.TestNamespace}, updatedRoleBinding)
				if err != nil {
					t.Fatalf("Could not get updated RoleBinding: %v", err)
				}
				assert.Equal(t, existingRoleBinding, updatedRoleBinding)
			}

			if tt.name == "rolebinding exists but outdated" {
				updatedRoleBinding := &rbacv1.RoleBinding{}
				err := nr.Client.Get(context.TODO(), types.NamespacedName{Name: argocdcommon.TestArgoCDName, Namespace: argocdcommon.TestNamespace}, updatedRoleBinding)
				if err != nil {
					t.Fatalf("Could not get updated RoleBinding: %v", err)
				}
				assert.Equal(t, testRoleRef, updatedRoleBinding.RoleRef)
				assert.Equal(t, testSubjects, updatedRoleBinding.Subjects)
			}
			resourceName = originalResourceName
		})
	}
}

func TestNotificationsReconciler_DeleteRoleBinding(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sa := argocdcommon.MakeTestServiceAccount()
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
				return makeTestNotificationsReconciler(t, ns, sa)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			originalResourceName := resourceName
			resourceName = argocdcommon.TestArgoCDName
			if err := nr.DeleteRoleBinding(tt.resourceName, ns.Name); (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.DeleteRoleBinding() error = %v, wantErr %v", err, tt.wantErr)
			}
			resourceName = originalResourceName
		})
	}
}
