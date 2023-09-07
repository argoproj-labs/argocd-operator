package notifications

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNotificationsReconciler_reconcileRoleBinding(t *testing.T) {
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := &NotificationsReconciler{
				Client:   tt.fields.Client,
				Scheme:   tt.fields.Scheme,
				Instance: tt.fields.Instance,
				Logger:   tt.fields.Logger,
			}
			if err := nr.reconcileRoleBinding(); (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.reconcileRoleBinding() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNotificationsReconciler_DeleteRoleBinding(t *testing.T) {
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
			if err := nr.DeleteRoleBinding(tt.args.name, tt.args.namespace); (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.DeleteRoleBinding() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
