package v1alpha1

import (
	"testing"

	"gotest.tools/assert"

	"github.com/argoproj-labs/argocd-operator/common"
)

func Test_ArgoCD_ApplicationInstanceLabelKey(t *testing.T) {
	cr := &ArgoCD{}
	cr.Spec.ApplicationInstanceLabelKey = "my.corp/instance"
	assert.Equal(t, cr.ApplicationInstanceLabelKey(), "my.corp/instance")
	cr = &ArgoCD{}
	assert.Equal(t, cr.ApplicationInstanceLabelKey(), common.ArgoCDDefaultApplicationInstanceLabelKey)
}
