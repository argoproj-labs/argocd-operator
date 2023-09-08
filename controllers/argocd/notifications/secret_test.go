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

func TestNotificationsReconciler_reconcileSecret(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	existingSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       SecretKind,
			APIVersion: APIVersionV1,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      argocdcommon.TestArgoCDName,
			Namespace: argocdcommon.TestNamespace,
		},
		StringData: testKVP,
	}

	tests := []struct {
		name         string
		resourceName string
		setupClient  func() *NotificationsReconciler
		wantErr      bool
	}{
		{
			name:         "secret doesn't exist",
			resourceName: argocdcommon.TestArgoCDName,
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, ns)
			},
			wantErr: false,
		},
		{
			name:         "secret exists",
			resourceName: argocdcommon.TestArgoCDName,
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, existingSecret, ns)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			originalResourceName := resourceName
			resourceName = argocdcommon.TestArgoCDName
			err := nr.reconcileSecret()
			if (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.reconcileSecret() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.name == "secret exists" {
				currentSecret := &corev1.Secret{}
				err := nr.Client.Get(context.TODO(), types.NamespacedName{Name: argocdcommon.TestArgoCDName, Namespace: argocdcommon.TestNamespace}, currentSecret)
				if err != nil {
					t.Fatalf("Could not get current Secret: %v", err)
				}
				assert.Equal(t, existingSecret, currentSecret)
			}

			resourceName = originalResourceName
		})
	}
}

func TestNotificationsReconciler_DeleteSecret(t *testing.T) {
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
			if err := nr.DeleteSecret(tt.resourceName, ns.Name); (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.DeleteSecret() error = %v, wantErr %v", err, tt.wantErr)
			}
			resourceName = originalResourceName
		})
	}
}
