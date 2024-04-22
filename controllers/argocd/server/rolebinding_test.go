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

func TestServerReconciler_createAndDeleteRoleBindings(t *testing.T) {
	sr := MakeTestServerReconciler(
		test.MakeTestArgoCD(nil),
	)
	sr.varSetter()

	// create role bindings
	err := sr.reconcileRoleBindings()
	assert.NoError(t, err)

	// rolebindings should be created in argocd namespace
	argoCDNsRb := &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "test-argocd-server", Namespace: sr.Instance.Namespace}, argoCDNsRb)
	assert.NoError(t, err)

	// delete rolebindings from argocd ns
	err = sr.deleteRoleBinding(resourceName, sr.Instance.Namespace)
	assert.NoError(t, err)

	// rolebindings shouldn't exist in argocd, source & managed namespace
	argoCDNsRb = &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "test-argocd-server", Namespace: sr.Instance.Namespace}, argoCDNsRb)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestServerReconciler_roleBindingWithCustomRole(t *testing.T) {
	sr := MakeTestServerReconciler(
		test.MakeTestArgoCD(nil),
	)
	sr.varSetter()

	// create argocd rolebinding
	err := sr.reconcileRoleBindings()
	assert.NoError(t, err)

	// argocd default rolebinding with default role ref should be created in argoCD ns
	argoCDRb := &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "test-argocd-server", Namespace: sr.Instance.Namespace}, argoCDRb)
	assert.NoError(t, err)
	assert.Equal(t, "test-argocd-server", argoCDRb.RoleRef.Name)

	// use custom role for argocd server
	t.Setenv("SERVER_CLUSTER_ROLE", "my-role")

	// update rolebinding
	err = sr.reconcileRoleBindings()
	// expect error as the roleref is updated so rolebinding is deleted and recreated again on next reconciliation
	assert.Error(t, err)
	err = sr.reconcileRoleBindings()
	assert.NoError(t, err)

	// rolebinding should reference custom role
	argoCDRb = &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "test-argocd-server", Namespace: sr.Instance.Namespace}, argoCDRb)
	assert.NoError(t, err)
	assert.Equal(t, "my-role", argoCDRb.RoleRef.Name)
}
