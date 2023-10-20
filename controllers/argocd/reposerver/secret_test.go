package reposerver

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
)

// reconcileTLSSecret() will be tested in e2e tests
func TestRepoServerReconciler_DeleteSecret(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	tests := []struct {
		name        string
		setupClient func() *RepoServerReconciler
		wantErr     bool
	}{
		{
			name: "successful delete",
			setupClient: func() *RepoServerReconciler {
				return makeTestRepoServerReconciler(t, ns)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			if err := nr.deleteTLSSecret(ns.Name); (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
