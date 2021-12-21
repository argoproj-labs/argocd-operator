package argocd

import (
	"context"
	"testing"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestReconcileArgoCD_reconcileDexService_Dex_Enabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.SSO = &argoprojv1alpha1.ArgoCDSSOSpec{
			Provider: argoprojv1alpha1.SSOProviderTypeDex,
			Dex: argoprojv1alpha1.ArgoCDDexSpec{
				OpenShiftOAuth: true,
			},
		}
	})
	r := makeTestReconciler(t, a)

	s := newServiceWithSuffix("dex-server", "dex-server", a)

	assert.NilError(t, r.reconcileDexService(a))
	assert.NilError(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s))
}

func TestReconcileArgoCD_reconcileDexService_dex_configruation_and_removal(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.SSO = &argoprojv1alpha1.ArgoCDSSOSpec{
			Provider: argoprojv1alpha1.SSOProviderTypeDex,
			Dex: argoprojv1alpha1.ArgoCDDexSpec{
				OpenShiftOAuth: true,
			},
		}
	})
	r := makeTestReconciler(t, a)

	s := newServiceWithSuffix("dex-server", "dex-server", a)

	// Dex configured, Create Service for Dex
	assert.NilError(t, r.reconcileDexService(a))
	assert.NilError(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s))

	// Dex configuration removed, existing service should be deleted
	a.Spec.SSO = nil

	assert.NilError(t, r.reconcileDexService(a))
	assert.ErrorContains(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s), "not found")

	// Service for Dex should not be created on reconciliation when disabled
	assert.NilError(t, r.reconcileDexService(a))
	assert.ErrorContains(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s), "not found")
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
