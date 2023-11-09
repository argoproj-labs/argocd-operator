package server

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)


func TestServerReconciler_managedNamespaceRoles(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sr := makeTestServerReconciler(t, ns)
	if sr.ManagedNamespaces == nil {
		sr.ManagedNamespaces = make(map[string]string)
	}

	// add argocd ns to managed namespaces manually
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
	// add new ns to managed namespaces manually
	sr.ManagedNamespaces[mngNS.ObjectMeta.Name] = ""
	err = sr.reconcileManagedRoles()
	assert.NoError(t, err)
	
	// role should be created in managed namespace
	mngNSRole := &rbacv1.Role{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: mngNS.Name,}, mngNSRole)
	assert.NoError(t, err)
	assert.Equal(t, getPolicyRulesForManagedNamespace(), mngNSRole.Rules)
}

func TestServerReconciler_sourceNamespaceRoles(t *testing.T) {}

//func TestServerReconciler_createResetAndDeleteRole(t *testing.T) {
//	ns := argocdcommon.MakeTestNamespace()
//	sr := makeTestServerReconciler(t, ns)
//
//	resourceName = testResourceName
//	resourceLabels = testResourceLabels
//
//	// create role
//	err := sr.reconcileRole()
//	assert.NoError(t, err)
//
//	// role should be created
//	currentRole := &rbacv1.Role{}
//	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: testResourceName, Namespace: argocdcommon.TestNamespace}, currentRole)
//	assert.NoError(t, err)
//	assert.Equal(t, testResourceLabels, currentRole.Labels)
//	//assert.Equal(t, getPolicyRules(), currentRole.Rules)
//
//	// modify role
//	currentRole.Rules = []rbacv1.PolicyRule{}
//	err = sr.Client.Update(context.TODO(), currentRole)
//	assert.NoError(t, err)
//
//	err = sr.reconcileRole()
//	assert.NoError(t, err)
//
//	// role should reset to default
//	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: testResourceName, Namespace: argocdcommon.TestNamespace}, currentRole)
//	assert.NoError(t, err)
//	//assert.Equal(t, getPolicyRules(), currentRole.Rules)
//
//	// delete role
//	err = sr.deleteRole(testResourceName, sr.Instance.Namespace)
//	assert.NoError(t, err)
//
//	// role should not exist
//	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: testResourceName, Namespace: argocdcommon.TestNamespace}, currentRole)
//	assert.Equal(t, true, errors.IsNotFound(err))
//}

