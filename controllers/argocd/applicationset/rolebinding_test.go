package applicationset

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/tests/test"
)

func TestApplicationSetReconciler_DeleteRoleBinding(t *testing.T) {
	ns := test.MakeTestNamespace(nil)
	sa := test.MakeTestServiceAccount()
	resourceName = test.TestArgoCDName
	tests := []struct {
		name        string
		setupClient func() *ApplicationSetReconciler
		wantErr     bool
	}{
		{
			name: "successful delete",
			setupClient: func() *ApplicationSetReconciler {
				return makeTestApplicationSetReconciler(t, false, ns, sa)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			if err := nr.deleteRoleBinding(resourceName, ns.Name); (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
