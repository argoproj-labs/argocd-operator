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

func TestNotificationsReconciler_reconcileConfigMap(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	existingConfigMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       ConfigMapKind,
			APIVersion: APIVersionV1,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      argocdcommon.TestArgoCDName,
			Namespace: argocdcommon.TestNamespace,
		},
		Data: testConfigMapData,
	}

	tests := []struct {
		name         string
		resourceName string
		setupClient  func() *NotificationsReconciler
		wantErr      bool
	}{
		{
			name:         "configMap doesn't exist",
			resourceName: argocdcommon.TestArgoCDName,
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, ns)
			},
			wantErr: false,
		},
		{
			name:         "configMap exists",
			resourceName: argocdcommon.TestArgoCDName,
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, existingConfigMap, ns)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			originalResourceName := resourceName
			resourceName = argocdcommon.TestArgoCDName
			err := nr.reconcileConfigMap()
			if (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.reconcileConfigMap() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.name == "configMap exists" {
				currentConfigMap := &corev1.ConfigMap{}
				err := nr.Client.Get(context.TODO(), types.NamespacedName{Name: argocdcommon.TestArgoCDName, Namespace: argocdcommon.TestNamespace}, currentConfigMap)
				if err != nil {
					t.Fatalf("Could not get current ConfigMap: %v", err)
				}
				assert.Equal(t, existingConfigMap, currentConfigMap)
			}

			resourceName = originalResourceName
		})
	}
}

func TestNotificationsReconciler_DeleteConfigMap(t *testing.T) {
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
			if err := nr.DeleteConfigMap(tt.resourceName, ns.Name); (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.DeleteConfigMap() error = %v, wantErr %v", err, tt.wantErr)
			}
			resourceName = originalResourceName
		})
	}
}
