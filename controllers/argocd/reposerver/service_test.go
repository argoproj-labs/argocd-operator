package reposerver

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/pkg/networking"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestReconcileService_create(t *testing.T) {
	tests := []struct {
		name            string
		reconciler      *RepoServerReconciler
		expectedError   bool
		expectedService *corev1.Service
	}{
		{
			name: "Service does not exist",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
			),
			expectedError:   false,
			expectedService: getDesiredSvc(),
		},
		{
			name: "Service exists",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestService(nil,
					func(svc *corev1.Service) {
						svc.Name = "test-argocd-repo-server"
					},
				),
			),
			expectedError:   false,
			expectedService: getDesiredSvc(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.reconciler.varSetter()

			err := tt.reconciler.reconcileService()
			assert.NoError(t, err)

			_, err = networking.GetService("test-argocd-repo-server", test.TestNamespace, tt.reconciler.Client)

			if tt.expectedError {
				assert.Error(t, err, "Expected an error but got none.")
			} else {
				assert.NoError(t, err, "Expected no error but got one.")
			}

		})
	}
}

func TestDeleteService(t *testing.T) {
	tests := []struct {
		name          string
		reconciler    *RepoServerReconciler
		serviceExist  bool
		expectedError bool
	}{
		{
			name: "Service exists",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestService(nil),
			),
			serviceExist:  true,
			expectedError: false,
		},
		{
			name: "Service does not exist",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
			),
			serviceExist:  false,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := tt.reconciler.deleteService(test.TestName, test.TestNamespace)

			if tt.serviceExist {
				_, err := networking.GetService(test.TestName, test.TestNamespace, tt.reconciler.Client)
				assert.True(t, apierrors.IsNotFound(err))
			}

			if tt.expectedError {
				assert.Error(t, err, "Expected an error but got none.")
			} else {
				assert.NoError(t, err, "Expected no error but got one.")
			}
		})
	}
}

func getDesiredSvc() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-argocd-repo-server",
			Namespace: "test-ns",
			Labels: map[string]string{
				"app.kubernetes.io/name":       "test-argocd-repo-server",
				"app.kubernetes.io/part-of":    "argocd",
				"app.kubernetes.io/instance":   "test-argocd",
				"app.kubernetes.io/managed-by": "argocd-operator",
				"app.kubernetes.io/component":  "repo-server",
			},
			Annotations: map[string]string{
				"argocds.argoproj.io/name":      "test-argocd",
				"argocds.argoproj.io/namespace": "test-ns",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app.kubernetes.io/name": "test-argocd-repo-server",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "server",
					Port:       8081,
					Protocol:   "TCP",
					TargetPort: intstr.FromInt(8081),
				},
				{
					Name:       "metrics",
					Port:       8084,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(8084),
				},
			},
		},
	}
}
