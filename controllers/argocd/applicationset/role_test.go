package applicationset

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/tests/test"
)

func TestApplicationSetReconciler_reconcileRole(t *testing.T) {
	ns := test.MakeTestNamespace(nil)
	resourceName = test.TestArgoCDName
	existingRole := &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       common.RoleKind,
			APIVersion: common.APIGroupVersionRbacV1,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      test.TestArgoCDName,
			Namespace: test.TestNamespace,
		},
		Rules: getPolicyRules(),
	}

	tests := []struct {
		name        string
		setupClient func() *ApplicationSetReconciler
		wantErr     bool
	}{
		{
			name: "create a role",
			setupClient: func() *ApplicationSetReconciler {
				return makeTestApplicationSetReconciler(t, false, ns)
			},
			wantErr: false,
		},
		{
			name: "Update a role",
			setupClient: func() *ApplicationSetReconciler {
				outdatedRole := existingRole
				outdatedRole.Rules = []rbacv1.PolicyRule{}
				return makeTestApplicationSetReconciler(t, false, outdatedRole, ns)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			err := nr.reconcileRole()
			if (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			updatedRole := &rbacv1.Role{}
			err = nr.Client.Get(context.TODO(), types.NamespacedName{Name: test.TestArgoCDName, Namespace: test.TestNamespace}, updatedRole)
			if err != nil {
				t.Fatalf("Could not get updated Role: %v", err)
			}
			assert.Equal(t, getPolicyRules(), updatedRole.Rules)
		})
	}
}

func TestApplicationSetReconciler_DeleteRole(t *testing.T) {
	ns := test.MakeTestNamespace(nil)
	resourceName = test.TestArgoCDName
	tests := []struct {
		name        string
		setupClient func() *ApplicationSetReconciler
		wantErr     bool
	}{
		{
			name: "successful delete",
			setupClient: func() *ApplicationSetReconciler {
				return makeTestApplicationSetReconciler(t, false, ns)
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
