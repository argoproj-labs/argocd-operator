package argocd

import (
	"testing"

	v1 "k8s.io/api/rbac/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestReconcileArgoCD_reconcileRole(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	_, err := r.reconcileRole("x", []v1.PolicyRule{}, a)
	assertNoError(t, err)
}
