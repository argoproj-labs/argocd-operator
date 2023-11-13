package server

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)


func TestServerReconciler_sourceNamespaceRoles(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	
	sr := makeTestServerReconciler(t, ns)
	if sr.SourceNamespaces == nil {
		sr.SourceNamespaces = make(map[string]string)
	}

	// manually add new ns to source namespaces 
	srcNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test",},}
	err := sr.Client.Create(context.TODO(), srcNS)
	assert.NoError(t, err)
	sr.SourceNamespaces[srcNS.ObjectMeta.Name] = ""
	err = sr.reconcileSourceRoles()
	assert.NoError(t, err)
	
	// role should be created in source namespace
	srcNSRole := &rbacv1.Role{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd_test", Namespace: srcNS.Name,}, srcNSRole)
	assert.NoError(t, err)
	assert.Equal(t, getPolicyRulesForSourceNamespace(), srcNSRole.Rules)
}

func TestServerReconciler_managedNamespaceRoles(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sr := makeTestServerReconciler(t, ns)
	if sr.ManagedNamespaces == nil {
		sr.ManagedNamespaces = make(map[string]string)
	}

	// manually add argocd ns to managed namespaces
	sr.ManagedNamespaces[ns.ObjectMeta.Name] = ""

	err := sr.reconcileManagedRoles()
	assert.NoError(t, err)

	// role should be created in ArgoCD's namespace
	argoCDNSRole := &rbacv1.Role{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: "argocd",}, argoCDNSRole)
	assert.NoError(t, err)
	assert.Equal(t, getPolicyRulesForArgoCDNamespace(), argoCDNSRole.Rules)

	// create a new namespace for ArgoCD to manage
	mngNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			Labels: map[string]string { "argocd.argoproj.io/managed-by" : "argocd"},
		},
	}
	err = sr.Client.Create(context.TODO(), mngNS)
	assert.NoError(t, err)
	// manually add new ns to managed namespaces 
	sr.ManagedNamespaces[mngNS.ObjectMeta.Name] = ""
	err = sr.reconcileManagedRoles()
	assert.NoError(t, err)
	
	// role should be created in managed namespace
	mngNSRole := &rbacv1.Role{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: mngNS.Name,}, mngNSRole)
	assert.NoError(t, err)
	assert.Equal(t, getPolicyRulesForManagedNamespace(), mngNSRole.Rules)
}

func TestServerReconciler_deleteRoles(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	
	sr := makeTestServerReconciler(t, ns)

	// create new namespaces for argocd to manage
	if sr.SourceNamespaces == nil {
		sr.SourceNamespaces = make(map[string]string)
	}
	if sr.ManagedNamespaces == nil {
		sr.ManagedNamespaces = make(map[string]string)
	}

	srcNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-src",},}
	err := sr.Client.Create(context.TODO(), srcNS)
	assert.NoError(t, err)

	mngNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mgn",
			Labels: map[string]string { "argocd.argoproj.io/managed-by" : "argocd"},
		},
	}
	err = sr.Client.Create(context.TODO(), mngNS)
	assert.NoError(t, err)
	
	sr.SourceNamespaces[srcNS.ObjectMeta.Name] = ""
	sr.ManagedNamespaces[mngNS.ObjectMeta.Name] = ""

	// create roles in source & managed ns
	err = sr.reconcileRoles()
	assert.NoError(t, err)

	// delete roles from source & managed ns
	err = sr.deleteRoles(sr.Instance.Name, sr.Instance.Namespace)
	assert.NoError(t, err)

	// role shouldn't exist in source & managed namespace

	srcNSRole := &rbacv1.Role{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd_test-src", Namespace: srcNS.Name,}, srcNSRole)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))

	mngNSRole := &rbacv1.Role{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: mngNS.Name,}, mngNSRole)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestServerReconciler_customRole(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	
	sr := makeTestServerReconciler(t, ns)

	if sr.ManagedNamespaces == nil {
		sr.ManagedNamespaces = make(map[string]string)
	}

	// manually add argocd ns for role reconciliation 
	sr.ManagedNamespaces[sr.Instance.Namespace] = ""

	// create argocd roles in ns
	err := sr.reconcileRoles()
	assert.NoError(t, err)

	// argocd default role should be created in argoCD ns
	argoCDRole := &rbacv1.Role{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: sr.Instance.Namespace,}, argoCDRole)
	assert.NoError(t, err)
	assert.Equal(t, getPolicyRulesForArgoCDNamespace(), argoCDRole.Rules)

	// use custom role for argocd server
	t.Setenv("SERVER_CLUSTER_ROLE", "my-role")

	// remove default role
	err = sr.reconcileRoles()
	assert.NoError(t, err)

	// default role shouldn't exists
	argoCDRole = &rbacv1.Role{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: sr.Instance.Namespace,}, argoCDRole)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))

}

