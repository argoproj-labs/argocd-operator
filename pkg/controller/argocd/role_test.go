package argocd

import (
	"context"
	"fmt"
	"testing"

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

func TestGetClusterRole(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	createClusterRoles(t, r.client)

	serverClusterRole := "argocd-server"
	clusterRole, err := r.getClusterRole(serverClusterRole)
	assertNoError(t, err)
	assert.Equal(t, serverClusterRole, clusterRole.Name)

	controllerClusterRole := "argocd-application-controller"
	clusterRole, err = r.getClusterRole(controllerClusterRole)
	assertNoError(t, err)
	assert.Equal(t, controllerClusterRole, clusterRole.Name)
}
