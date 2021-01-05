package argocd

import (
	"context"
	"fmt"
	"testing"

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
	assertNoError(t, err)

	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	reconciledRole := &v1.Role{}
	assertNoError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.DeepEqual(t, expectedRules, reconciledRole.Rules)

	// undersirable change.
	reconciledRole.Rules = policyRuleForRedisHa()
	assertNoError(t, r.client.Update(context.TODO(), reconciledRole))

	// overwrite it.
	_, err = r.reconcileRole(workloadIdentifier, expectedRules, a)
	assertNoError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
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

	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	reconciledClusterRole := &v1.ClusterRole{}
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName}, reconciledClusterRole))
	assert.DeepEqual(t, expectedRules, reconciledClusterRole.Rules)

	// undersirable change.
	reconciledClusterRole.Rules = policyRuleForRedisHa()
	assert.NilError(t, r.client.Update(context.TODO(), reconciledClusterRole))

	// overwrite it.
	_, err = r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName}, reconciledClusterRole))
	assert.DeepEqual(t, expectedRules, reconciledClusterRole.Rules)
}
