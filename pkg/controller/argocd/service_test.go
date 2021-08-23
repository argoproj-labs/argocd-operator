package argocd

import (
	"context"
	"os"
	"testing"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestReconcileArgoCD_reconcileDexService_Dex_Enabled(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	s := newServiceWithSuffix("dex-server", "dex-server", a)

	assert.NilError(t, r.reconcileDexService(a))
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s))
}

func TestReconcileArgoCD_reconcileDexService_Dex_Disabled(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	s := newServiceWithSuffix("dex-server", "dex-server", a)

	// Create Service for Dex
	assert.NilError(t, r.reconcileDexService(a))
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s))

	// Disable Dex, existing service should be deleted
	os.Setenv("DISABLE_DEX", "true")
	t.Cleanup(func() {
		os.Unsetenv("DISABLE_DEX")
	})

	assert.NilError(t, r.reconcileDexService(a))
	assert.ErrorContains(t, r.client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s), "not found")

	// Service for Dex should not be created on reconciliation when disabled
	assert.NilError(t, r.reconcileDexService(a))
	assert.ErrorContains(t, r.client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s), "not found")
}

func TestReconcileArgoCD_reconcileServerServiceHooks(t *testing.T) {
	defer resetHooks()()
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)
	certAnn := "cert.kubernetes"
	Register(func(cr *argoprojv1alpha1.ArgoCD, v interface{}, s string) error {
		switch o := v.(type) {
		case *corev1.Service:
			if o.Name == cr.Name+"-server" {
				if o.Annotations == nil {
					o.Annotations = map[string]string{}
				}
				o.Annotations[certAnn] = o.Name
			}
		}
		return nil
	})

	err := r.reconcileServerService(a)
	assert.NilError(t, err)

	t.Run("annotation added", func(t *testing.T) {
		svc := newServiceWithSuffix("server", "server", a)
		assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}, svc))
		value, found := svc.Annotations[certAnn]
		assert.Equal(t, found, true)
		assert.Equal(t, value, svc.Name)
	})

	t.Run("other annotations are persisted", func(t *testing.T) {
		// add new annotations to service
		anns := map[string]string{"a": "1", "b": "2"}
		svc := newServiceWithSuffix("server", "server", a)
		assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}, svc))
		svc.Annotations = anns
		assert.NilError(t, r.client.Update(context.TODO(), svc))

		err := r.reconcileServerService(a)
		assert.NilError(t, err)

		// check if the old annotations are still present
		assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}, svc))
		anns[certAnn] = svc.Name
		assert.DeepEqual(t, anns, svc.Annotations)
	})

	t.Run("duplicate annotations are overwritten", func(t *testing.T) {
		// add new annotations to service
		anns := map[string]string{certAnn: "test-value", "b": "2"}
		svc := newServiceWithSuffix("server", "server", a)
		assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}, svc))
		svc.Annotations = anns
		assert.NilError(t, r.client.Update(context.TODO(), svc))

		err := r.reconcileServerService(a)
		assert.NilError(t, err)

		// check if the annotation with the duplicate key is overwritten
		assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}, svc))
		anns[certAnn] = svc.Name
		assert.DeepEqual(t, anns, svc.Annotations)
	})
}
