package notifications

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var existingRole = &rbacv1.Role{
	TypeMeta: metav1.TypeMeta{
		Kind:       RoleKind,
		APIVersion: "rbac.authorization.k8s.io/v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      argocdcommon.TestArgoCDName,
		Namespace: argocdcommon.TestNamespace,
	},
	Rules: getPolicyRules(),
}

func TestNotificationsReconciler_reconcileRole(t *testing.T) {
	originalResourceName := resourceName
	resourceName = argocdcommon.TestArgoCDName

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: argocdcommon.TestNamespace,
		},
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
		// Add role exists but outdated tests
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			err := nr.reconcileRole()
			if (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.reconcileRole() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.name == "role exists and is correct" {
				updatedRole := &rbacv1.Role{}
				err := nr.Client.Get(context.TODO(), types.NamespacedName{Name: argocdcommon.TestArgoCDName, Namespace: argocdcommon.TestNamespace}, updatedRole)
				if err != nil {
					t.Fatalf("Could not get updated Role: %v", err)
				}
				assert.Equal(t, existingRole, updatedRole)
			}
		})
	}

	resourceName = originalResourceName
}

func TestNotificationsReconciler_DeleteRole(t *testing.T) {
	type fields struct {
		Client   client.Client
		Scheme   *runtime.Scheme
		Instance *v1alpha1.ArgoCD
		Logger   logr.Logger
	}
	type args struct {
		name      string
		namespace string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := &NotificationsReconciler{
				Client:   tt.fields.Client,
				Scheme:   tt.fields.Scheme,
				Instance: tt.fields.Instance,
				Logger:   tt.fields.Logger,
			}
			if err := nr.DeleteRole(tt.args.name, tt.args.namespace); (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.DeleteRole() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
