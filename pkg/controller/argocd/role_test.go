package argocd

import (
	"context"
	"fmt"
	syslog "log"
	"testing"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"gotest.tools/assert"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestReconcileArgoCD_reconcileRole(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	workloadIdentifier := "x"
	expectedRules := policyRuleForApplicationController()
	_, err := r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, err)

	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	reconciledRole := &v1.Role{}
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.DeepEqual(t, expectedRules, reconciledRole.Rules)

	// undersirable change.
	reconciledRole.Rules = policyRuleForRedisHa()
	assert.NilError(t, r.client.Update(context.TODO(), reconciledRole))

	// overwrite it.
	_, err = r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.DeepEqual(t, expectedRules, reconciledRole.Rules)
}

func TestReconcileArgoCD_reconcileClusterRole(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	workloadIdentifier := common.ArgoCDApplicationControllerComponent
	expectedRules := policyRuleForApplicationControllerClusterRole()
	_, err := r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, err)

	reconciledClusterRole := &v1.ClusterRole{}
	clusterRoleName := generateResourceName(workloadIdentifier, a)
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))
	assert.DeepEqual(t, expectedRules, reconciledClusterRole.Rules)

	// undersirable change.
	reconciledClusterRole.Rules = policyRuleForRedisHa()
	assert.NilError(t, r.client.Update(context.TODO(), reconciledClusterRole))

	// overwrite it.
	_, err = r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))
	assert.DeepEqual(t, expectedRules, reconciledClusterRole.Rules)
}

func TestReconcileArgoCDClusterConfig_reconcileClusterRole(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCDForClusterConfig()
	r := makeTestReconciler(t, a)

	workloadIdentifier := common.ArgoCDApplicationControllerComponent
	expectedRules := policyRulesForClusterConfig()
	_, err := r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, err)

	reconciledClusterRole := &v1.ClusterRole{}
	clusterRoleName := generateResourceName(workloadIdentifier, a)
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))
	assert.DeepEqual(t, expectedRules, reconciledClusterRole.Rules)

	// undersirable change.
	reconciledClusterRole.Rules = policyRuleForRedisHa()
	assert.NilError(t, r.client.Update(context.TODO(), reconciledClusterRole))

	// overwrite it.
	_, err = r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))
	assert.DeepEqual(t, expectedRules, reconciledClusterRole.Rules)
}

func TestReconcileArgoCD_reconcileRoles_with_extensions(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	extension := v1.PolicyRule{
		Verbs:     []string{"get"},
		APIGroups: []string{"demo.example.com"},
		Resources: []string{"demos"},
	}

	mod := testModifier{
		role: func(cr *argoprojv1alpha1.ArgoCD, role *v1.Role) error {
			if role.ObjectMeta.Name == cr.ObjectMeta.Name+"-argocd-server" {
				role.Rules = append(role.Rules, extension)
			}
			return nil
		},
	}
	Register(mod)

	_, err := r.reconcileRoles(a)
	assert.NilError(t, err)

	reconciledRole := &v1.Role{}
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: "argocd"}, reconciledRole))

	want := []v1.PolicyRule{
		{
			Verbs:     []string{"create", "get", "list", "watch", "update", "patch", "delete"},
			APIGroups: []string{""},
			Resources: []string{"secrets", "configmaps"},
		},
		{
			Verbs:     []string{"create", "get", "list", "watch", "update", "delete", "patch"},
			APIGroups: []string{"argoproj.io"},
			Resources: []string{"applications", "appprojects"},
		},
		{
			Verbs:     []string{"create", "list"},
			APIGroups: []string{""},
			Resources: []string{"events"},
		},
		{
			Verbs:     []string{"get"},
			APIGroups: []string{"demo.example.com"},
			Resources: []string{"demos"},
		},
	}
	assert.DeepEqual(t, want, reconciledRole.Rules)
}

type testModifier struct {
	role func(cr *argoprojv1alpha1.ArgoCD, role *v1.Role) error
}

func (m testModifier) Role(cr *argoprojv1alpha1.ArgoCD, role *v1.Role) error {
	return m.role(cr, role)
}
