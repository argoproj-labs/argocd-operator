package server

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func TestServerReconciler_deleteRole(t *testing.T) {
	sr := MakeTestServerReconciler(
		test.MakeTestArgoCD(nil),
	)
	sr.varSetter()

	// create role
	err := sr.reconcileRole()
	assert.NoError(t, err)

	// role should exist in ArgoCD namespace
	role := &rbacv1.Role{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "test-argocd-server", Namespace: test.TestNamespace}, role)
	assert.NoError(t, err)

	// delete role
	err = sr.deleteRole(resourceName, sr.Instance.Namespace)
	assert.NoError(t, err)

	// role shouldn't exist in ArgoCD namespace
	role = &rbacv1.Role{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "test-argocd-server", Namespace: test.TestNamespace}, role)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestServerReconciler_customRole(t *testing.T) {
	sr := MakeTestServerReconciler(
		test.MakeTestArgoCD(nil),
	)
	sr.varSetter()

	// create argocd roles in ns
	err := sr.reconcileRole()
	assert.NoError(t, err)

	// argocd default role should be created in ArgoCD's ns
	argoCDRole := &rbacv1.Role{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "test-argocd-server", Namespace: sr.Instance.Namespace}, argoCDRole)
	assert.NoError(t, err)
	assert.Equal(t, getPolicyRules(), argoCDRole.Rules)

	// use custom role for argocd server
	t.Setenv("SERVER_CLUSTER_ROLE", "my-role")

	// remove default role
	err = sr.reconcileRole()
	assert.NoError(t, err)

	// default role shouldn't exists
	argoCDRole = &rbacv1.Role{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "test-argocd-server", Namespace: sr.Instance.Namespace}, argoCDRole)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}
