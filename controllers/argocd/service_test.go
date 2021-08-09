package argocd

import (
	"context"
	"os"
	"testing"

	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestReconcileArgoCD_reconcileDexService_Dex_Enabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	s := newServiceWithSuffix("dex-server", "dex-server", a)

	assert.NilError(t, r.reconcileDexService(a))
	assert.NilError(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s))
}

func TestReconcileArgoCD_reconcileDexService_Dex_Disabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	s := newServiceWithSuffix("dex-server", "dex-server", a)

	// Create Service for Dex
	assert.NilError(t, r.reconcileDexService(a))
	assert.NilError(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s))

	// Disable Dex, existing service should be deleted
	os.Setenv("DISABLE_DEX", "true")
	t.Cleanup(func() {
		os.Unsetenv("DISABLE_DEX")
	})

	assert.NilError(t, r.reconcileDexService(a))
	assert.ErrorContains(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s), "not found")

	// Service for Dex should not be created on reconciliation when disabled
	assert.NilError(t, r.reconcileDexService(a))
	assert.ErrorContains(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s), "not found")
}
