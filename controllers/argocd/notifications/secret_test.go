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
			Name:      NotificationsSecretName,
			Namespace: argocdcommon.TestNamespace,
			Labels:    resourceLabels,
		},
	}

	tests := []struct {
		name        string
		setupClient func() *NotificationsReconciler
		wantErr     bool
	}{
		{
			name: "secret doesn't exist",
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, ns)
			},
			wantErr: false,
		},
		{
			name: "secret exists",
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, existingSecret, ns)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			err := nr.reconcileSecret()
			if (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.reconcileSecret() error = %v, wantErr %v", err, tt.wantErr)
			}

			currentSecret := &corev1.Secret{}
			err = nr.Client.Get(context.TODO(), types.NamespacedName{Name: NotificationsSecretName, Namespace: argocdcommon.TestNamespace}, currentSecret)
			if err != nil {
				t.Fatalf("Could not get current Secret: %v", err)
			}
			assert.Equal(t, NotificationsSecretName, currentSecret.ObjectMeta.Name)
			assert.Equal(t, argocdcommon.TestNamespace, currentSecret.ObjectMeta.Namespace)
			assert.Equal(t, resourceLabels, currentSecret.ObjectMeta.Labels)
		})
	}
}

func TestNotificationsReconciler_DeleteSecret(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
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
			if err := nr.DeleteSecret(ns.Name); (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.DeleteSecret() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
