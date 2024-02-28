package redis

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func TestReconcileServiceAccount(t *testing.T) {
	tests := []struct {
		name                     string
		reconciler               *RedisReconciler
		expectedError            bool
		expectedCreateLogMessage string
	}{
		{
			name: "ServiceAccount does not exist",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
			),
			expectedError:            false,
			expectedCreateLogMessage: "serviceaccount created",
		},
		{
			name: "ServiceAccount exists",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestServiceAccount(
					func(sa *corev1.ServiceAccount) {
						sa.Name = "test-argocd-redis"
					},
				),
			),
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.reconciler.varSetter()
			err := tt.reconciler.reconcileServiceAccount()
			assert.NoError(t, err)

			_, err = permissions.GetServiceAccount("test-argocd-redis", test.TestNamespace, tt.reconciler.Client)

			// Validate the error condition
			if tt.expectedError {
				assert.Error(t, err, "Expected an error but got none.")
			} else {
				assert.NoError(t, err, "Expected no error but got one.")
			}

		})
	}
}

func TestDeleteServiceAccount(t *testing.T) {
	tests := []struct {
		name                string
		reconciler          *RedisReconciler
		serviceAccountExist bool
		expectedError       bool
	}{
		{
			name: "ServiceAccount exists",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestServiceAccount(),
			),
			serviceAccountExist: true,
			expectedError:       false,
		},
		{
			name: "ServiceAccount does not exist",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
			),
			serviceAccountExist: false,
			expectedError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := tt.reconciler.deleteServiceAccount(test.TestName, test.TestNamespace)

			if tt.serviceAccountExist {
				_, err := permissions.GetServiceAccount(test.TestName, test.TestNamespace, tt.reconciler.Client)
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
