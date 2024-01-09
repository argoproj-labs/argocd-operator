package server

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"

	rbacv1 "k8s.io/api/rbac/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestServerReconciler_createAndDeleteClusterRole(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sr := makeTestServerReconciler(t, ns)

	err := sr.reconcileClusterRole()
	assert.NoError(t, err)

	// cluster role should not be created as ArgoCD in not cluster scoped
	cr := &rbacv1.ClusterRole{}
	err = sr.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: "argocd-argocd-argocd-server"}, cr)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))

	// make ArgoCD cluster scope
	sr.ClusterScoped = true
	err = sr.reconcileClusterRole()
	assert.NoError(t, err)

	// cluster role should be created as ArgoCD is cluster scoped
	cr = &rbacv1.ClusterRole{}
	err = sr.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: "argocd-argocd-argocd-server"}, cr)
	assert.NoError(t, err)

	// disable cluster ArgoCD
	sr.ClusterScoped = false
	err = sr.reconcileClusterRole()
	assert.NoError(t, err)

	// cluster role should be deleted as ArgoCD is changed to namespace scoped
	cr = &rbacv1.ClusterRole{}
	err = sr.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: "argocd-argocd-argocd-server"}, cr)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}
