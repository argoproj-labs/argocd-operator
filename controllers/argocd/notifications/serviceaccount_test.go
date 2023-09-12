package notifications

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestNotificationsReconciler_reconcileServiceAccount(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	resourceName = argocdcommon.TestArgoCDName
	existingServiceAccount := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       ServiceAccountKind,
			APIVersion: APIVersionV1,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        argocdcommon.TestArgoCDName,
			Namespace:   argocdcommon.TestNamespace,
			Labels:      resourceLabels,
			Annotations: map[string]string{},
		},
	}

	tests := []struct {
		name        string
		setupClient func() *NotificationsReconciler
		wantErr     bool
	}{
		{
			name: "serviceAccount doesn't exist",
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, ns)
			},
			wantErr: false,
		},
		{
			name: "serviceAccount exists",
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, existingServiceAccount, ns)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			err := nr.reconcileServiceAccount()
			if (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.reconcileServiceAccount() error = %v, wantErr %v", err, tt.wantErr)
			}

			currentServiceAccount := &corev1.ServiceAccount{}
			err = nr.Client.Get(context.TODO(), types.NamespacedName{Name: argocdcommon.TestArgoCDName, Namespace: argocdcommon.TestNamespace}, currentServiceAccount)
			if err != nil {
				t.Fatalf("Could not get current ServiceAccount: %v", err)
			}
			assert.Equal(t, resourceLabels, currentServiceAccount.Labels)
		})
	}
}

func TestNotificationsReconciler_DeleteServiceAccount(t *testing.T) {
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
			if err := nr.DeleteServiceAccount(resourceName, ns.Name); (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.DeleteServiceAccount() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
