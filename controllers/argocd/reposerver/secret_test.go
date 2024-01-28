package reposerver

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func TestDeleteSecret(t *testing.T) {
	tests := []struct {
		name          string
		reconciler    *RepoServerReconciler
		secretExist   bool
		expectedError bool
	}{
		{
			name: "Secret exists",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(),
				test.MakeTestSecret(),
			),
			secretExist:   true,
			expectedError: false,
		},
		{
			name: "Secret does not exist",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(),
			),
			secretExist:   false,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := tt.reconciler.deleteSecret(test.TestName, test.TestNamespace)

			if tt.secretExist {
				_, err := workloads.GetSecret(test.TestName, test.TestNamespace, tt.reconciler.Client)
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
