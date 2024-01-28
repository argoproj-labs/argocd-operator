package reposerver

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func TestDeleteDeployment(t *testing.T) {
	tests := []struct {
		name            string
		reconciler      *RepoServerReconciler
		deploymentExist bool
		expectedError   bool
	}{
		{
			name: "Deployment exists",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(),
				test.MakeTestDeployment(),
			),
			deploymentExist: true,
			expectedError:   false,
		},
		{
			name: "Deployment does not exist",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(),
			),
			deploymentExist: false,
			expectedError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := tt.reconciler.deleteDeployment(test.TestName, test.TestNamespace)

			if tt.deploymentExist {
				_, err := workloads.GetDeployment(test.TestName, test.TestNamespace, tt.reconciler.Client)
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
