package redis

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func TestDeleteConfigMap(t *testing.T) {
	tests := []struct {
		name           string
		reconciler     *RedisReconciler
		configMapExist bool
		expectedError  bool
	}{
		{
			name: "ConfigMap exists",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestConfigMap(nil),
			),
			configMapExist: true,
			expectedError:  false,
		},
		{
			name: "ConfigMap does not exist",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
			),
			configMapExist: false,
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := tt.reconciler.deleteConfigMap(test.TestName, test.TestNamespace)

			if tt.configMapExist {
				_, err := workloads.GetConfigMap(test.TestName, test.TestNamespace, tt.reconciler.Client)
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
