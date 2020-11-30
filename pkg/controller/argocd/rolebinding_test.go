package argocd

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestReconcileArgoCD_reconcileRoleBinding(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	assertNoError(t, r.reconcileRoleBinding("x", &v1.Role{}, &corev1.ServiceAccount{}, a))

	// TODO: Compare contents of the object
}

func TestReconcileArgoCD_reconcileClusterRoleBinding(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	assertNoError(t, r.reconcileClusterRoleBinding("x", &v1.ClusterRole{}, &corev1.ServiceAccount{}, a))

	// TODO: Compare contents of the object
}
