package argocd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

func TestShouldDeployCommitServer(t *testing.T) {
	meta := metav1.ObjectMeta{Name: testArgoCDName, Namespace: testNamespace}
	tests := []struct {
		name string
		cr   *argoproj.ArgoCD
		want bool
	}{
		{
			name: "disabled by default when source hydrator is not configured",
			cr: &argoproj.ArgoCD{
				ObjectMeta: meta,
				Spec: argoproj.ArgoCDSpec{
					ApplicationSet: &argoproj.ArgoCDApplicationSet{
						Enabled: ptr.To(true),
					},
				},
			},
			want: false,
		},
		{
			name: "disabled when commit server is configured but source hydrator is not enabled",
			cr: &argoproj.ArgoCD{
				ObjectMeta: meta,
				Spec: argoproj.ArgoCDSpec{
					CommitServer: argoproj.ArgoCDCommitServerSpec{
						LogLevel: "debug",
					},
				},
			},
			want: false,
		},
		{
			name: "disabled when source hydrator is explicitly disabled",
			cr: &argoproj.ArgoCD{
				ObjectMeta: meta,
				Spec: argoproj.ArgoCDSpec{
					SourceHydrator: argoproj.ArgoCDSourceHydratorSpec{
						Enabled: ptr.To(false),
					},
				},
			},
			want: false,
		},
		{
			name: "enabled when source hydrator is explicitly enabled",
			cr: &argoproj.ArgoCD{
				ObjectMeta: meta,
				Spec: argoproj.ArgoCDSpec{
					SourceHydrator: argoproj.ArgoCDSourceHydratorSpec{
						Enabled: ptr.To(true),
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, UseCommitServer(tt.cr))
		})
	}
}
