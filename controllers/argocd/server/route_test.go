package server

import (
	"context"
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	routev1 "github.com/openshift/api/route/v1"
)

func TestServerReconciler_createUpdateAndDeleteRoute(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sr := makeTestServerReconciler(t, ns)
	routev1.Install(sr.Scheme)

	ann := map[string]string {"example.com":"test"}

	// configure route resource in ArgoCD
	sr.Instance.Spec.Server.Route =  argoproj.ArgoCDRouteSpec{
		Enabled: true,
		Annotations: ann,
	}

	err := sr.reconcileRoute()
	assert.NoError(t, err)

	// route resource should be created
	route := &routev1.Route{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-server", Namespace: "argocd",}, route)
	assert.NoError(t, err)
	assert.Equal(t, ann, route.ObjectMeta.Annotations)

	// modify route resource in ArgoCD
	var policy routev1.WildcardPolicyType = "Subdomain"
	sr.Instance.Spec.Server.Route.WildcardPolicy = &policy
	err = sr.reconcileRoute()
	assert.NoError(t, err)
	
	// route resource should be updated
	route = &routev1.Route{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-server", Namespace: "argocd",}, route)
	assert.NoError(t, err)
	assert.Equal(t, policy, route.Spec.WildcardPolicy)

	// disable route in ArgoCD
	sr.Instance.Spec.Server.Route.Enabled = false
	err = sr.reconcileRoute()
	assert.NoError(t, err)

	// route resource should be deleted
	route = &routev1.Route{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-server", Namespace: "argocd",}, route)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestServerReconciler_routeTLS(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sr := makeTestServerReconciler(t, ns)
	routev1.Install(sr.Scheme)

	secureTLSConfig := &routev1.TLSConfig{
		Termination:                   routev1.TLSTerminationPassthrough,
		InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
	}
	secureRoutePort :=  &routev1.RoutePort{
		TargetPort: intstr.FromString("https"),
	}

	insecureTLSConfig := &routev1.TLSConfig{
		InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
		Termination:                   routev1.TLSTerminationEdge,
	}
	insecureRoutePort :=  &routev1.RoutePort{
		TargetPort: intstr.FromString("http"),
	}

	// configure route resource with default configs in ArgoCD
	sr.Instance.Spec.Server.Route =  argoproj.ArgoCDRouteSpec{
		Enabled: true,
	}

	err := sr.reconcileRoute()
	assert.NoError(t, err)

	// route resource should be created with default tls config
	route := &routev1.Route{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-server", Namespace: "argocd",}, route)
	assert.NoError(t, err)
	assert.Equal(t, secureTLSConfig, route.Spec.TLS)
	assert.Equal(t, secureRoutePort, route.Spec.Port)


	// disable tls using insecure flag
	sr.Instance.Spec.Server.Insecure = true
	err = sr.reconcileRoute()
	assert.NoError(t, err)
	
	// route resource should be updated to use insecure tls configs
	route = &routev1.Route{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-server", Namespace: "argocd",}, route)
	assert.NoError(t, err)
	assert.Equal(t, insecureTLSConfig, route.Spec.TLS)
	assert.Equal(t, insecureRoutePort, route.Spec.Port)

}


