package argocd

import (
	"context"
	"os"
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestReconcileArgoCD_reconcileDexService_Dex_Enabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	s := newServiceWithSuffix("dex-server", "dex-server", a)

	assert.NoError(t, r.reconcileDexService(a))
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s))
}

func TestReconcileArgoCD_reconcileDexService_Dex_Disabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	s := newServiceWithSuffix("dex-server", "dex-server", a)

	// Create Service for Dex
	assert.NoError(t, r.reconcileDexService(a))
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s))

	// Disable Dex, existing service should be deleted
	os.Setenv("DISABLE_DEX", "true")
	t.Cleanup(func() {
		os.Unsetenv("DISABLE_DEX")
	})

	assert.NoError(t, r.reconcileDexService(a))
	//assert.ErrorContains(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s), "not found")
	//TODO: https://github.com/stretchr/testify/pull/1022 introduced ErrorContains, but is not yet available in a tagged release. Revert to ErrorContains once this becomes available
	assert.Error(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s))
	assert.Contains(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s).Error(), "not found")

	// Service for Dex should not be created on reconciliation when disabled
	assert.NoError(t, r.reconcileDexService(a))
	//assert.ErrorContains(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s), "not found")
	//TODO: https://github.com/stretchr/testify/pull/1022 introduced ErrorContains, but is not yet available in a tagged release. Revert to ErrorContains once this becomes available
	assert.Error(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s))
	assert.Contains(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s).Error(), "not found")
}

func TestEnsureAutoTLSAnnotation(t *testing.T) {
	a := makeTestArgoCD()
	t.Run("Ensure annotation will be set for OpenShift", func(t *testing.T) {
		routeAPIFound = true
		svc := newService(a)

		// Annotation is inserted, update is required
		needUpdate := ensureAutoTLSAnnotation(svc, "some-secret", true)
		assert.Equal(t, needUpdate, true)
		atls, ok := svc.Annotations[common.AnnotationOpenShiftServiceCA]
		assert.Equal(t, ok, true)
		assert.Equal(t, atls, "some-secret")

		// Annotation already set, doesn't need update
		needUpdate = ensureAutoTLSAnnotation(svc, "some-secret", true)
		assert.Equal(t, needUpdate, false)
	})
	t.Run("Ensure annotation will be unset for OpenShift", func(t *testing.T) {
		routeAPIFound = true
		svc := newService(a)
		svc.Annotations = make(map[string]string)
		svc.Annotations[common.AnnotationOpenShiftServiceCA] = "some-secret"

		// Annotation getting removed, update required
		needUpdate := ensureAutoTLSAnnotation(svc, "some-secret", false)
		assert.Equal(t, needUpdate, true)
		_, ok := svc.Annotations[common.AnnotationOpenShiftServiceCA]
		assert.Equal(t, ok, false)

		// Annotation does not exist, no update required
		needUpdate = ensureAutoTLSAnnotation(svc, "some-secret", false)
		assert.Equal(t, needUpdate, false)
	})
	t.Run("Ensure annotation will not be set for non-OpenShift", func(t *testing.T) {
		routeAPIFound = false
		svc := newService(a)
		needUpdate := ensureAutoTLSAnnotation(svc, "some-secret", true)
		assert.Equal(t, needUpdate, false)
		_, ok := svc.Annotations[common.AnnotationOpenShiftServiceCA]
		assert.Equal(t, ok, false)
	})
}
