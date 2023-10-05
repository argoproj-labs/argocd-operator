package applicationset

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestApplicationSetReconciler_reconcileRoleBinding(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sa := argocdcommon.MakeTestServiceAccount()
	resourceName = argocdcommon.TestArgoCDName

	tests := []struct {
		name        string
		setupClient func() *ApplicationSetReconciler
		wantErr     bool
	}{
		{
			name: "create a rolebinding",
			setupClient: func() *ApplicationSetReconciler {
				return makeTestApplicationSetReconciler(t, false, ns, sa)
			},
			wantErr: false,
		},
		{
			name: "update a rolebinding",
			setupClient: func() *ApplicationSetReconciler {
				outdatedRoleBinding := &rbacv1.RoleBinding{
					TypeMeta: metav1.TypeMeta{
						Kind:       common.RoleBindingKind,
						APIVersion: common.APIGroupVersionRbacV1,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      argocdcommon.TestArgoCDName,
						Namespace: argocdcommon.TestNamespace,
					},
					RoleRef:  rbacv1.RoleRef{},
					Subjects: []rbacv1.Subject{},
				}
				return makeTestApplicationSetReconciler(t, false, outdatedRoleBinding, ns, sa)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			err := nr.reconcileRoleBinding()
			if (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}
			updatedRoleBinding := &rbacv1.RoleBinding{}
			err = nr.Client.Get(context.TODO(), types.NamespacedName{Name: argocdcommon.TestArgoCDName, Namespace: argocdcommon.TestNamespace}, updatedRoleBinding)
			if err != nil {
				t.Fatalf("Could not get updated RoleBinding: %v", err)
			}
			assert.Equal(t, argocdcommon.TestRoleRef, updatedRoleBinding.RoleRef)
			assert.Equal(t, argocdcommon.TestSubjects, updatedRoleBinding.Subjects)
		})
	}
}

func TestApplicationSetReconciler_DeleteRoleBinding(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sa := argocdcommon.MakeTestServiceAccount()
	resourceName = argocdcommon.TestArgoCDName
	tests := []struct {
		name        string
		setupClient func() *ApplicationSetReconciler
		wantErr     bool
	}{
		{
			name: "successful delete",
			setupClient: func() *ApplicationSetReconciler {
				return makeTestApplicationSetReconciler(t, false, ns, sa)
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
