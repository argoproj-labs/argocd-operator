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
	resourceName = argocdcommon.TestArgoCDName
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
		name        string
		setupClient func() *NotificationsReconciler
		wantErr     bool
	}{
		{
			name: "rolebinding doesn't exist",
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, ns, sa)
			},
			wantErr: false,
		},
		{
			name: "rolebinding exists and is correct",
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, existingRoleBinding, ns, sa)
			},
			wantErr: false,
		},
		{
			name: "rolebinding exists but outdated",
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
			err := nr.reconcileRoleBinding()
			if (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.reconcileRoleBinding() error = %v, wantErr %v", err, tt.wantErr)
			}
			updatedRoleBinding := &rbacv1.RoleBinding{}
			err = nr.Client.Get(context.TODO(), types.NamespacedName{Name: argocdcommon.TestArgoCDName, Namespace: argocdcommon.TestNamespace}, updatedRoleBinding)
			if err != nil {
				t.Fatalf("Could not get updated RoleBinding: %v", err)
			}
			assert.Equal(t, testRoleRef, updatedRoleBinding.RoleRef)
			assert.Equal(t, testSubjects, updatedRoleBinding.Subjects)
		})
	}
}

func TestNotificationsReconciler_DeleteRoleBinding(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sa := argocdcommon.MakeTestServiceAccount()
	resourceName = argocdcommon.TestArgoCDName
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
			if err := nr.DeleteRoleBinding(resourceName, ns.Name); (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.DeleteRoleBinding() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
