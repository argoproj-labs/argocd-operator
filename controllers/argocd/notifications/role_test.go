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

func TestNotificationsReconciler_reconcileRole(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	resourceName = argocdcommon.TestArgoCDName
	existingRole := &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       RoleKind,
			APIVersion: APIVersionRbacV1,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      argocdcommon.TestArgoCDName,
			Namespace: argocdcommon.TestNamespace,
		},
		Rules: getPolicyRules(),
	}

	tests := []struct {
		name        string
		setupClient func() *NotificationsReconciler
		wantErr     bool
	}{
		{
			name: "role doesn't exist",
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, ns)
			},
			wantErr: false,
		},
		{
			name: "role exists and is correct",
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, existingRole, ns)
			},
			wantErr: false,
		},
		{
			name: "role exists but outdated",
			setupClient: func() *NotificationsReconciler {
				outdatedRole := existingRole
				outdatedRole.Rules = []rbacv1.PolicyRule{}
				return makeTestNotificationsReconciler(t, outdatedRole, ns)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			err := nr.reconcileRole()
			if (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.reconcileRole() error = %v, wantErr %v", err, tt.wantErr)
			}

			updatedRole := &rbacv1.Role{}
			err = nr.Client.Get(context.TODO(), types.NamespacedName{Name: argocdcommon.TestArgoCDName, Namespace: argocdcommon.TestNamespace}, updatedRole)
			if err != nil {
				t.Fatalf("Could not get updated Role: %v", err)
			}
			assert.Equal(t, getPolicyRules(), updatedRole.Rules)
		})
	}
}

func TestNotificationsReconciler_DeleteRole(t *testing.T) {
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
			if err := nr.DeleteRole(resourceName, ns.Name); (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.DeleteRole() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
