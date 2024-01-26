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

func TestServerReconciler_createAndDeleteRoleBindings(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sr := makeTestServerReconciler(t, ns)
	setTestResourceNameAndLabels(sr)

	// create new namespaces for argocd to manage
	if sr.SourceNamespaces == nil {
		sr.SourceNamespaces = make(map[string]string)
	}
	if sr.ManagedNamespaces == nil {
		sr.ManagedNamespaces = make(map[string]string)
	}

	sr.ManagedNamespaces[sr.Instance.Namespace] = ""

	srcNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-src"}}
	err := sr.Client.Create(context.TODO(), srcNS)
	assert.NoError(t, err)

	mngNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name:   "test-mgn"}}
	err = sr.Client.Create(context.TODO(), mngNS)
	assert.NoError(t, err)

	sr.SourceNamespaces[srcNS.ObjectMeta.Name] = ""
	sr.ManagedNamespaces[mngNS.ObjectMeta.Name] = ""

	// create role bindings
	err = sr.reconcileRoleBindings()
	assert.NoError(t, err)

	// rolebindings should be created in argocd, managed & source namespaces
	argoCDNsRb := &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: sr.Instance.Namespace}, argoCDNsRb)
	assert.NoError(t, err)

	mngNsRb := &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: mngNS.Name}, mngNsRb)
	assert.NoError(t, err)

	srcNsRb := &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-argocd-server", Namespace: srcNS.Name}, srcNsRb)
	assert.NoError(t, err)

	// delete rolebindings from argocd, source & managed ns
	err = sr.deleteRoleBindings(resourceName, uniqueResourceName)
	assert.NoError(t, err)

	// rolebindings shouldn't exist in argocd, source & managed namespace
	argoCDNsRb = &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: sr.Instance.Namespace}, argoCDNsRb)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))

	mngNsRb = &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: mngNS.Name}, mngNsRb)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))

	srcNsRb = &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-argocd-server", Namespace: srcNS.Name}, srcNsRb)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestServerReconciler_roleBindingWithCustomRole(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sr := makeTestServerReconciler(t, ns)
	setTestResourceNameAndLabels(sr)

	if sr.ManagedNamespaces == nil {
		sr.ManagedNamespaces = make(map[string]string)
	}

	// manually add argocd ns for rolebinding reconciliation
	sr.ManagedNamespaces[sr.Instance.Namespace] = ""

	// create argocd rolebinding
	err := sr.reconcileRoleBindings()
	assert.NoError(t, err)

	// argocd default rolebinding with default role ref should be created in argoCD ns
	argoCDRb := &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: sr.Instance.Namespace}, argoCDRb)
	assert.NoError(t, err)
	assert.Equal(t, "argocd-argocd-server", argoCDRb.RoleRef.Name)

	// use custom role for argocd server
	t.Setenv("SERVER_CLUSTER_ROLE", "my-role")

	// update rolebinding
	err = sr.reconcileRoleBindings()
	assert.NoError(t, err)

	// rolebinding should reference custom role
	argoCDRb = &rbacv1.RoleBinding{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: sr.Instance.Namespace}, argoCDRb)
	assert.NoError(t, err)
	assert.Equal(t, "my-role", argoCDRb.RoleRef.Name)

}
