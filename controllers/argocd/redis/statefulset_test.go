package redis

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func TestDeleteStatefulSet(t *testing.T) {
	tests := []struct {
		name             string
		reconciler       *RedisReconciler
		statefulSetExist bool
		expectedError    bool
	}{
		{
			name: "StatefulSet exists",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestStatefulSet(nil),
			),
			statefulSetExist: true,
			expectedError:    false,
		},
		{
			name: "StatefulSet does not exist",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
			),
			statefulSetExist: false,
			expectedError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := tt.reconciler.deleteStatefulSet(test.TestName, test.TestNamespace)

			if tt.statefulSetExist {
				_, err := workloads.GetStatefulSet(test.TestName, test.TestNamespace, tt.reconciler.Client)
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
