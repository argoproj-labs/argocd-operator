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
	existingServiceAccount := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       ServiceAccountKind,
			APIVersion: APIVersionV1,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      argocdcommon.TestArgoCDName,
			Namespace: argocdcommon.TestNamespace,
		},
	}

	tests := []struct {
		name         string
		resourceName string
		setupClient  func() *NotificationsReconciler
		wantErr      bool
	}{
		{
			name:         "serviceAccount doesn't exist",
			resourceName: argocdcommon.TestArgoCDName,
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, ns)
			},
			wantErr: false,
		},
		{
			name:         "serviceAccount exists",
			resourceName: argocdcommon.TestArgoCDName,
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, existingServiceAccount, ns)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			originalResourceName := resourceName
			resourceName = argocdcommon.TestArgoCDName
			err := nr.reconcileServiceAccount()
			if (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.reconcileServiceAccount() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.name == "serviceAccount exists" {
				currentServiceAccount := &corev1.ServiceAccount{}
				err := nr.Client.Get(context.TODO(), types.NamespacedName{Name: argocdcommon.TestArgoCDName, Namespace: argocdcommon.TestNamespace}, currentServiceAccount)
				if err != nil {
					t.Fatalf("Could not get current ServiceAccount: %v", err)
				}
				assert.Equal(t, existingServiceAccount, currentServiceAccount)
			}

			resourceName = originalResourceName
		})
	}
}

func TestNotificationsReconciler_DeleteServiceAccount(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
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
				return makeTestNotificationsReconciler(t, ns)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			originalResourceName := resourceName
			resourceName = argocdcommon.TestArgoCDName
			if err := nr.DeleteServiceAccount(tt.resourceName, ns.Name); (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.DeleteServiceAccount() error = %v, wantErr %v", err, tt.wantErr)
			}
			resourceName = originalResourceName
		})
	}
}
