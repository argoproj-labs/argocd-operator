package reposerver

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestRepoServerReconciler_reconcileSecret(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	resourceLabels = testExpectedLabels
	tests := []struct {
		name        string
		setupClient func() *RepoServerReconciler
		wantErr     bool
	}{
		{
			name: "create a secret",
			setupClient: func() *RepoServerReconciler {
				return makeTestRepoServerReconciler(t, ns)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			err := nr.reconcileTLSSecret()
			if (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			currentSecret := &corev1.Secret{}
			err = nr.Client.Get(context.TODO(), types.NamespacedName{Name: RepoServerTLSSecretName, Namespace: argocdcommon.TestNamespace}, currentSecret)
			if err != nil {
				t.Fatalf("Could not get current Secret: %v", err)
			}
			assert.Equal(t, testExpectedLabels, currentSecret.ObjectMeta.Labels)
		})
	}
}

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
