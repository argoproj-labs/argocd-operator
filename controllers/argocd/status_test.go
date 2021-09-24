package argocd

import (
	"testing"

	"gotest.tools/assert"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestReconcileArgoCD_reconcileStatusSSOConfig_multi_sso_configured(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCDForKeycloakWithDex()

	templateAPIFound = true
	r := makeTestReconciler(t, a)
	assert.NilError(t, r.reconcileStatusSSOConfig(a))
	assert.Equal(t, a.Status.SSOConfig, "Failed")
}
func TestReconcileArgoCD_reconcileStatusSSOConfig_only_keycloak_configured(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCDForKeycloak()

	templateAPIFound = true
	r := makeTestReconciler(t, a)
	assert.NilError(t, r.reconcileStatusSSOConfig(a))
	assert.Equal(t, a.Status.SSOConfig, "Success")
}
func TestReconcileArgoCD_reconcileStatusSSOConfig_only_dex_configured(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCDWithResources()

	templateAPIFound = true
	r := makeTestReconciler(t, a)
	assert.NilError(t, r.reconcileStatusSSOConfig(a))
	assert.Equal(t, a.Status.SSOConfig, "Success")
}
func TestReconcileArgoCD_reconcileStatusSSOConfig_no_sso_configured(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	templateAPIFound = true
	r := makeTestReconciler(t, a)
	assert.NilError(t, r.reconcileStatusSSOConfig(a))
	assert.Equal(t, a.Status.SSOConfig, "Unknown")
}
