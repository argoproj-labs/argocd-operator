package notifications

import (
	"reflect"
	"testing"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNotificationsReconciler_reconcileRole(t *testing.T) {
	testScheme := runtime.NewScheme()
	testClient := fake.NewClientBuilder().WithScheme(testScheme).Build()
	type fields struct {
		Client   client.Client
		Scheme   *runtime.Scheme
		Instance *v1alpha1.ArgoCD
		Logger   logr.Logger
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "role doesn't exist",
			fields: fields{
				Client:   testClient,
				Scheme:   testScheme,
				Instance: makeTestArgoCD(),
				Logger:   makeTestNotificationsLogger(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := &NotificationsReconciler{
				Client:   tt.fields.Client,
				Scheme:   tt.fields.Scheme,
				Instance: tt.fields.Instance,
				Logger:   tt.fields.Logger,
			}
			if err := nr.reconcileRole(); (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.reconcileRole() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
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

func Test_getPolicyRules(t *testing.T) {
	tests := []struct {
		name string
		want []rbacv1.PolicyRule
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPolicyRules(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPolicyRules() = %v, want %v", got, tt.want)
			}
		})
	}
}
