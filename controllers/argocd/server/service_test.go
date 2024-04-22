package server

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/openshift"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"

	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestServerReconciler_createAndUpdateService(t *testing.T) {
	sr := MakeTestServerReconciler(
		test.MakeTestArgoCD(nil),
	)
	sr.varSetter()

	err := sr.reconcileService()
	assert.NoError(t, err)

	// service should be created with default service type
	svc := &corev1.Service{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "test-argocd-server", Namespace: test.TestNamespace}, svc)
	assert.NoError(t, err)
	assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)

	// modify svc resource in ArgoCD
	sr.Instance.Spec.Server.Service.Type = corev1.ServiceTypeLoadBalancer
	err = sr.reconcileService()
	assert.NoError(t, err)

	// service type should be updated
	svc = &corev1.Service{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "test-argocd-server", Namespace: test.TestNamespace}, svc)
	assert.NoError(t, err)
	assert.Equal(t, corev1.ServiceTypeLoadBalancer, svc.Spec.Type)
}

func TestServerReconciler_autoTLSService(t *testing.T) {
	openshift.SetVersionAPIFound(true)
	sr := MakeTestServerReconciler(
		test.MakeTestArgoCD(nil),
	)
	sr.varSetter()

	err := sr.reconcileService()
	assert.NoError(t, err)

	// service should be created without openshift tls annotations
	svc := &corev1.Service{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "test-argocd-server", Namespace: test.TestNamespace}, svc)
	assert.NoError(t, err)
	assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)

	// enable autoTLS
	sr.Instance.Spec.Server.Route = v1beta1.ArgoCDRouteSpec{
		TLS: &routev1.TLSConfig{
			Termination: routev1.TLSTerminationReencrypt,
		},
	}
	err = sr.reconcileService()
	assert.NoError(t, err)

	// service type should be updated with openshift tls annotations
	svc = &corev1.Service{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "test-argocd-server", Namespace: test.TestNamespace}, svc)
	assert.NoError(t, err)
	v, found := svc.ObjectMeta.Annotations["service.beta.openshift.io/serving-cert-secret-name"]
	assert.True(t, found)
	assert.Equal(t, "argocd-server-tls", v)

	// disable autoTLS
	sr.Instance.Spec.Server.Route = v1beta1.ArgoCDRouteSpec{}
	err = sr.reconcileService()
	assert.NoError(t, err)

	// annotation should be removed
	svc = &corev1.Service{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "test-argocd-server", Namespace: test.TestNamespace}, svc)
	assert.NoError(t, err)
	_, found = svc.ObjectMeta.Annotations["service.beta.openshift.io/serving-cert-secret-name"]
	assert.False(t, found)
}
