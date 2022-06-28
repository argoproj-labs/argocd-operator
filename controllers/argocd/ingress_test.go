package argocd

import (
	"context"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/stretchr/testify/assert"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
)

// Test when '.spec.server.host' in ArgoCD CR is changed.
// Expect hostname in Rule and TLS of Ingress resource to change.
func TestReconcileArgoCD_reconcileIngress_changeHost(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	var (
		beforeHost = "before"
		afterHost  = "after"
	)
	a := makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.Server.Host = beforeHost
		a.Spec.Server.Ingress.Enabled = true
	})
	r := makeTestReconciler(t, a)

	err := r.reconcileArgoServerIngress(a)
	assert.NoError(t, err)

	a = makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.Server.Host = afterHost
		a.Spec.Server.Ingress.Enabled = true
	})
	err = r.reconcileArgoServerIngress(a)
	assert.NoError(t, err)

	ingress := &networkingv1.Ingress{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-server",
		Namespace: testNamespace,
	}, ingress)
	assert.NoError(t, err)

	assert.Equal(t, afterHost, ingress.Spec.Rules[0].Host)
	assert.Equal(t, afterHost, ingress.Spec.TLS[0].Hosts[0])
}

// Test when '.spec.server.ingress.enabled' is change from true to false.
// Expect ingress will be removed.
func TestReconcileArgoCD_reconcileIngress_disabledIngress(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	a := makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.Server.Ingress.Enabled = true
	})
	r := makeTestReconciler(t, a)

	err := r.reconcileArgoServerIngress(a)
	assert.NoError(t, err)

	a = makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.Server.Ingress.Enabled = false
	})
	err = r.reconcileArgoServerIngress(a)
	assert.NoError(t, err)

	ingress := &networkingv1.Ingress{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-server",
		Namespace: testNamespace,
	}, ingress)
	assertNotFound(t, err)
}
