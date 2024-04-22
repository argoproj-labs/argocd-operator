package server

import (
	"context"
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	routev1 "github.com/openshift/api/route/v1"
)

func TestServerReconciler_routeTLS(t *testing.T) {
	sr := MakeTestServerReconciler(
		test.MakeTestArgoCD(nil),
	)
	sr.varSetter()
	routev1.Install(sr.Scheme)

	secureTLSConfig := &routev1.TLSConfig{
		Termination:                   routev1.TLSTerminationPassthrough,
		InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
	}
	secureRoutePort := &routev1.RoutePort{
		TargetPort: intstr.FromString("https"),
	}

	insecureTLSConfig := &routev1.TLSConfig{
		InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
		Termination:                   routev1.TLSTerminationEdge,
	}
	insecureRoutePort := &routev1.RoutePort{
		TargetPort: intstr.FromString("http"),
	}

	// configure route resource with default configs in ArgoCD
	sr.Instance.Spec.Server.Route = argoproj.ArgoCDRouteSpec{
		Enabled: true,
	}

	err := sr.reconcileRoute()
	assert.NoError(t, err)

	// route resource should be created with default tls config
	route := &routev1.Route{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "test-argocd-server", Namespace: test.TestNamespace}, route)
	assert.NoError(t, err)
	assert.Equal(t, secureTLSConfig, route.Spec.TLS)
	assert.Equal(t, secureRoutePort, route.Spec.Port)

	// disable tls using insecure flag
	sr.Instance.Spec.Server.Insecure = true
	err = sr.reconcileRoute()
	assert.NoError(t, err)

	// route resource should be updated to use insecure tls configs
	route = &routev1.Route{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "test-argocd-server", Namespace: test.TestNamespace}, route)
	assert.NoError(t, err)
	assert.Equal(t, insecureTLSConfig, route.Spec.TLS)
	assert.Equal(t, insecureRoutePort, route.Spec.Port)
}
