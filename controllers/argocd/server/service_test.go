package server

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
)

func TestServerReconciler_createAndUpdateService(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sr := makeTestServerReconciler(t, ns)

	err := sr.reconcileService()
	assert.NoError(t, err)

	// service should be created with default service type
	svc := &corev1.Service{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-server", Namespace: "argocd",}, svc)
	assert.NoError(t, err)
	assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)

	// modify svc resource in ArgoCD
	sr.Instance.Spec.Server.Service.Type = corev1.ServiceTypeLoadBalancer
	err = sr.reconcileService()
	assert.NoError(t, err)
	
	// service type should be updated
	svc = &corev1.Service{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-server", Namespace: "argocd",}, svc)
	assert.NoError(t, err)
	assert.Equal(t, corev1.ServiceTypeLoadBalancer, svc.Spec.Type)
}