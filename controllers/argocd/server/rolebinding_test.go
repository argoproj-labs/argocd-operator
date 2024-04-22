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
