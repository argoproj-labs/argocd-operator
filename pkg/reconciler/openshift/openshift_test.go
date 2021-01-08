package openshift

import (
	"fmt"
	"testing"

	"github.com/argoproj-labs/argocd-operator/pkg/controller/argocd"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gotest.tools/assert"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestReconcileArgoCD_reconcileClusterRoles_with_extensions(t *testing.T) {

	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCDForClusterConfig()
	testClusterRole := &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", a.Name, testApplicationController),
		},
		Rules: argocd.PolicyRuleForApplicationController(),
	}

	argocd.Register(reconcilerHook)
	argocd.ApplyReconcilerHook(a, testClusterRole)
	want := append(argocd.PolicyRuleForApplicationController(), policyRulesForClusterConfig()...)
	assert.DeepEqual(t, want, testClusterRole.Rules)
}
