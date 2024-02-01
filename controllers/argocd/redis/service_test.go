package redis

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/pkg/networking"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func TestDeleteService(t *testing.T) {
	tests := []struct {
		name          string
		reconciler    *RedisReconciler
		serviceExist  bool
		expectedError bool
	}{
		{
			name: "Service exists",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestService(nil),
			),
			serviceExist:  true,
			expectedError: false,
		},
		{
			name: "Service does not exist",
			reconciler: makeTestRedisReconciler(
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
