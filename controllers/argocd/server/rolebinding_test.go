package server

import (
	"context"
	"testing"

	appctrl "github.com/argoproj-labs/argocd-operator/controllers/argocd/appcontroller"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestServerReconciler_createAndDeleteRoleBindings(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	
	sr := makeTestServerReconciler(t, ns)

	// create new namespaces for argocd to manage
	if sr.SourceNamespaces == nil {
		sr.SourceNamespaces = make(map[string]string)
	}
	if sr.ManagedNamespaces == nil {
		sr.ManagedNamespaces = make(map[string]string)
	}

	sr.ManagedNamespaces[sr.Instance.Namespace] = ""

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

	// create server SA & roles 
	err = sr.reconcileServiceAccount()
	assert.NoError(t, err)
	err = sr.reconcileRoles()
	assert.NoError(t, err)

	// create dummy appcontroller service account
	appCtrlSA := &corev1.ServiceAccount{ 
		ObjectMeta: metav1.ObjectMeta{
			Name: appctrl.GetAppControllerName(sr.Instance.Name),
			Namespace: sr.Instance.Namespace,
		},
	}
	err = sr.Client.Create(context.TODO(), appCtrlSA)
	assert.NoError(t, err)

	// create role bindings
	err = sr.reconcileRoleBindings()
	assert.NoError(t, err)

	// rolebindings should be created in argocd, managed & source namespaces
	argoCDNSRole := &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: sr.Instance.Namespace,}, argoCDNSRole)
	assert.NoError(t, err)

	mngNSRole := &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: mngNS.Name,}, mngNSRole)
	assert.NoError(t, err)

	srcNSRole := &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd_test-src", Namespace: srcNS.Name,}, srcNSRole)
	assert.NoError(t, err)

	// delete roles from argocd, source & managed ns
	err = sr.deleteRoleBindings(sr.Instance.Name, sr.Instance.Namespace)
	assert.NoError(t, err)

	// rolebindings shouldn't exist in argocd, source & managed namespace
	argoCDNSRole = &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: sr.Instance.Namespace,}, argoCDNSRole)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))

	srcNSRole = &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd_test-src", Namespace: srcNS.Name,}, srcNSRole)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))

	mngNSRole = &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: mngNS.Name,}, mngNSRole)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestServerReconciler_roleBindingWithCustomRole(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	
	sr := makeTestServerReconciler(t, ns)

	if sr.ManagedNamespaces == nil {
		sr.ManagedNamespaces = make(map[string]string)
	}

	// manually add argocd ns for rolebinding reconciliation 
	sr.ManagedNamespaces[sr.Instance.Namespace] = ""

	// create server SA & roles 
	err := sr.reconcileServiceAccount()
	assert.NoError(t, err)
	err = sr.reconcileRoles()
	assert.NoError(t, err)

	// create dummy appcontroller service account
	appCtrlSA := &corev1.ServiceAccount{ 
		ObjectMeta: metav1.ObjectMeta{
			Name: appctrl.GetAppControllerName(sr.Instance.Name),
			Namespace: sr.Instance.Namespace,
		},
	}
	err = sr.Client.Create(context.TODO(), appCtrlSA)
	assert.NoError(t, err)

	// create argocd rolebinding
	err = sr.reconcileRoleBindings()
	assert.NoError(t, err)

	// argocd default rolebinding with default role ref should be created in argoCD ns 
	argoCDRB := &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: sr.Instance.Namespace,}, argoCDRB)
	assert.NoError(t, err)
	assert.Equal(t, "argocd-argocd-server", argoCDRB.RoleRef.Name)

	// use custom role for argocd server
	t.Setenv("SERVER_CLUSTER_ROLE", "my-role")

	// update rolebinding
	err = sr.reconcileRoleBindings()
	assert.NoError(t, err)

	// rolebinding should reference custom role
	argoCDRB = &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: sr.Instance.Namespace,}, argoCDRB)
	assert.NoError(t, err)
	assert.Equal(t, "my-role", argoCDRB.RoleRef.Name)

}