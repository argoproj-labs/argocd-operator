package argocd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

func hasPodsExecRule(rules []v1.PolicyRule) bool {
	for _, r := range rules {
		if len(r.Resources) == 1 &&
			r.Resources[0] == "pods/exec" {
			return true
		}
	}
	return false
}

func TestPolicyRuleForServer_WebTerminal(t *testing.T) {
	webTerminalEnabled := true
	tests := []struct {
		name string
		cr   *argoproj.ArgoCD
		want bool
	}{
		{
			name: "default",
			cr: &argoproj.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example",
					Namespace: "default",
				},
				Spec: argoproj.ArgoCDSpec{},
			},
			want: false,
		},
		{
			name: "disabled",
			cr:   &argoproj.ArgoCD{},
			want: false,
		},
		{
			name: "enabled",
			cr: &argoproj.ArgoCD{
				Spec: argoproj.ArgoCDSpec{
					WebTerminalEnabled: &webTerminalEnabled,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := policyRuleForServer(tt.cr)
			assert.Equal(t, tt.want, hasPodsExecRule(rules))
		})
	}
}
