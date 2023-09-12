package notifications

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestNotificationsReconciler_reconcileDeployment(t *testing.T) {
	resourceName = argocdcommon.TestArgoCDName
	resourceLabels = testLabels
	ns := argocdcommon.MakeTestNamespace()
	existingDeployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       DeploymentKind,
			APIVersion: APIVersionAppsV1,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      argocdcommon.TestArgoCDName,
			Namespace: argocdcommon.TestNamespace,
			Labels:    resourceLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: resourceLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: argocdcommon.TestArgoCDName,
						},
					},
					Volumes: []corev1.Volume{},
				},
			},
			Selector: &metav1.LabelSelector{},
			Replicas: &testReplicas,
		},
	}

	tests := []struct {
		name        string
		setupClient func() *NotificationsReconciler
		wantErr     bool
	}{
		{
			name: "deployment doesn't exist",
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, ns)
			},
			wantErr: false,
		},
		{
			name: "deployment exists and is correct",
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, existingDeployment, ns)
			},
			wantErr: false,
		},
		{
			name: "deployment exists but outdated",
			setupClient: func() *NotificationsReconciler {
				outdatedDeployment := existingDeployment
				outdatedDeployment.ObjectMeta.Labels = testKVP
				return makeTestNotificationsReconciler(t, outdatedDeployment, ns)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			err := nr.reconcileDeployment()
			if (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.reconcileDeployment() error = %v, wantErr %v", err, tt.wantErr)
			}

			updatedDeployment := &appsv1.Deployment{}
			err = nr.Client.Get(context.TODO(), types.NamespacedName{Name: argocdcommon.TestArgoCDName, Namespace: argocdcommon.TestNamespace}, updatedDeployment)
			if err != nil {
				t.Fatalf("Could not get updated Deployment: %v", err)
			}
			assert.Equal(t, testLabels, updatedDeployment.ObjectMeta.Labels)
		})
	}
}

func TestNotificationsReconciler_DeleteDeployment(t *testing.T) {
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
			if err := nr.DeleteDeployment(resourceName, ns.Name); (err != nil) != tt.wantErr {
				t.Errorf("NotificationsReconciler.DeleteDeployment() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
