// Copyright 2025 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package argocdagent

import (
	"context"
	"testing"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// Test helper function for route configuration
func withRouteEnabled(enabled bool) argoCDOpt {
	return func(a *argoproj.ClusterArgoCD) {
		if a.Spec.ArgoCDAgent == nil {
			a.Spec.ArgoCDAgent = &argoproj.ArgoCDAgentSpec{}
		}
		if a.Spec.ArgoCDAgent.Principal == nil {
			a.Spec.ArgoCDAgent.Principal = &argoproj.PrincipalSpec{}
		}
		if a.Spec.ArgoCDAgent.Principal.Server == nil {
			a.Spec.ArgoCDAgent.Principal.Server = &argoproj.PrincipalServerSpec{}
		}
		a.Spec.ArgoCDAgent.Principal.Server.Route = argoproj.ArgoCDAgentPrincipalRouteSpec{
			Enabled: &enabled,
		}
	}
}

// Override makeTestReconcilerScheme to include route API
func makeTestReconcilerSchemeWithRoute() *runtime.Scheme {
	s := scheme.Scheme
	_ = argoproj.AddToScheme(s)
	_ = routev1.AddToScheme(s)
	return s
}

// Helper function to create a test route
func makeTestRoute(cr *argoproj.ClusterArgoCD) *routev1.Route {
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
		Spec: buildPrincipalRouteSpec(testCompName, cr),
	}
}

func TestReconcilePrincipalRoute_RouteDoesNotExist_PrincipalDisabled(t *testing.T) {
	// Test case: Route doesn't exist and principal is disabled
	// Expected behavior: Should do nothing (no creation, no error)
	argoutil.SetRouteAPIFound(true)

	cr := makeTestClusterArgoCD(withPrincipalEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerSchemeWithRoute()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalRoute(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Route was not created
	route := &routev1.Route{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: testNamespace,
	}, route)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalRoute_RouteDoesNotExist_PrincipalEnabled(t *testing.T) {
	// Test case: Route doesn't exist and principal is enabled
	// Expected behavior: Should create the Route with expected spec
	argoutil.SetRouteAPIFound(true)

	cr := makeTestClusterArgoCD(withPrincipalEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerSchemeWithRoute()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalRoute(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Route was created
	route := &routev1.Route{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: testNamespace,
	}, route)
	assert.NoError(t, err)

	// Verify Route has expected metadata
	expectedName := generateAgentResourceName(cr.Name, testCompName)
	assert.Equal(t, expectedName, route.Name)
	assert.Equal(t, cr.Namespace, route.Namespace)
	assert.Equal(t, buildLabelsForAgentPrincipal(cr.Name, testCompName), route.Labels)

	// Verify Route has expected spec
	expectedSpec := buildPrincipalRouteSpec(testCompName, cr)
	assert.Equal(t, expectedSpec.Port, route.Spec.Port)
	assert.Equal(t, expectedSpec.To, route.Spec.To)
	assert.Equal(t, expectedSpec.TLS, route.Spec.TLS)

	// Verify owner reference is set
	assert.Len(t, route.OwnerReferences, 1)
	assert.Equal(t, cr.Name, route.OwnerReferences[0].Name)
	assert.Equal(t, "ArgoCD", route.OwnerReferences[0].Kind)
}

func TestReconcilePrincipalRoute_RouteExists_PrincipalDisabled(t *testing.T) {
	// Test case: Route exists and principal is disabled
	// Expected behavior: Should delete the Route
	argoutil.SetRouteAPIFound(true)

	cr := makeTestClusterArgoCD(withPrincipalEnabled(false))

	// Create existing Route
	existingRoute := makeTestRoute(makeTestClusterArgoCD(withPrincipalEnabled(true)))

	resObjs := []client.Object{cr, existingRoute}
	sch := makeTestReconcilerSchemeWithRoute()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalRoute(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Route was deleted
	route := &routev1.Route{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: testNamespace,
	}, route)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalRoute_RouteExists_PrincipalEnabled_SameSpec(t *testing.T) {
	// Test case: Route exists, principal is enabled, and spec is the same
	// Expected behavior: Should do nothing (no update)
	argoutil.SetRouteAPIFound(true)

	cr := makeTestClusterArgoCD(withPrincipalEnabled(true))

	existingRoute := makeTestRoute(cr)

	resObjs := []client.Object{cr, existingRoute}
	sch := makeTestReconcilerSchemeWithRoute()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalRoute(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Route still exists with same spec
	route := &routev1.Route{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: testNamespace,
	}, route)
	assert.NoError(t, err)

	expectedSpec := buildPrincipalRouteSpec(testCompName, cr)
	assert.Equal(t, expectedSpec.Port, route.Spec.Port)
	assert.Equal(t, expectedSpec.To, route.Spec.To)
	assert.Equal(t, expectedSpec.TLS, route.Spec.TLS)
}

func TestReconcilePrincipalRoute_RouteExists_PrincipalEnabled_DifferentSpec(t *testing.T) {
	// Test case: Route exists, principal is enabled, but spec is different
	// Expected behavior: Should update the Route with expected spec
	argoutil.SetRouteAPIFound(true)

	cr := makeTestClusterArgoCD(withPrincipalEnabled(true))

	// Create existing Route with different spec
	existingRoute := makeTestRoute(cr)
	existingRoute.Spec.TLS.Termination = routev1.TLSTerminationEdge
	existingRoute.Spec.To.Name = "different-service"

	resObjs := []client.Object{cr, existingRoute}
	sch := makeTestReconcilerSchemeWithRoute()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalRoute(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Route was updated with correct spec
	route := &routev1.Route{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: testNamespace,
	}, route)
	assert.NoError(t, err)

	expectedSpec := buildPrincipalRouteSpec(testCompName, cr)
	assert.Equal(t, expectedSpec.Port, route.Spec.Port)
	assert.Equal(t, expectedSpec.To, route.Spec.To)
	assert.Equal(t, expectedSpec.TLS, route.Spec.TLS)
}

func TestReconcilePrincipalRoute_RouteExists_PrincipalNotSet(t *testing.T) {
	// Test case: Route exists but principal spec is not set (nil)
	// Expected behavior: Should delete the Route
	argoutil.SetRouteAPIFound(true)

	cr := makeTestClusterArgoCD() // No principal configuration

	// Create existing Route
	existingRoute := makeTestRoute(makeTestClusterArgoCD(withPrincipalEnabled(true)))

	resObjs := []client.Object{cr, existingRoute}
	sch := makeTestReconcilerSchemeWithRoute()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalRoute(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Route was deleted
	route := &routev1.Route{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: testNamespace,
	}, route)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalRoute_RouteDoesNotExist_AgentNotSet(t *testing.T) {
	// Test case: Route doesn't exist and agent spec is not set (nil)
	// Expected behavior: Should do nothing
	argoutil.SetRouteAPIFound(true)

	cr := makeTestClusterArgoCD() // No agent configuration
	cr.Spec.ArgoCDAgent = nil

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerSchemeWithRoute()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalRoute(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Route was not created
	route := &routev1.Route{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: testNamespace,
	}, route)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalRoute_VerifyRouteSpec(t *testing.T) {
	// Test case: Verify the route spec has correct configuration
	// Expected behavior: Should create route with correct port, target service, and TLS configuration
	argoutil.SetRouteAPIFound(true)

	cr := makeTestClusterArgoCD(withPrincipalEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerSchemeWithRoute()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalRoute(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Route was created
	route := &routev1.Route{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: testNamespace,
	}, route)
	assert.NoError(t, err)

	// Verify port configuration
	assert.NotNil(t, route.Spec.Port)
	assert.Equal(t, intstr.FromInt(PrincipalServiceTargetPort), route.Spec.Port.TargetPort)

	// Verify target service
	assert.Equal(t, "Service", route.Spec.To.Kind)
	assert.Equal(t, generateAgentResourceName(cr.Name, testCompName), route.Spec.To.Name)

	// Verify TLS configuration
	assert.NotNil(t, route.Spec.TLS)
	assert.Equal(t, routev1.TLSTerminationPassthrough, route.Spec.TLS.Termination)
	assert.Equal(t, routev1.InsecureEdgeTerminationPolicyNone, route.Spec.TLS.InsecureEdgeTerminationPolicy)
}

func TestReconcilePrincipalRoute_RouteDisabled_RouteDoesNotExist(t *testing.T) {
	// Test case: Route doesn't exist and route is disabled
	// Expected behavior: Should not create the Route
	argoutil.SetRouteAPIFound(true)

	cr := makeTestClusterArgoCD(withPrincipalEnabled(true), withRouteEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerSchemeWithRoute()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalRoute(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Route was not created
	route := &routev1.Route{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: testNamespace,
	}, route)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalRoute_RouteDisabled_RouteExists(t *testing.T) {
	// Test case: Route exists but route is disabled
	// Expected behavior: Should delete the Route
	argoutil.SetRouteAPIFound(true)

	cr := makeTestClusterArgoCD(withPrincipalEnabled(true), withRouteEnabled(false))

	// Create existing Route
	existingRoute := makeTestRoute(makeTestClusterArgoCD(withPrincipalEnabled(true), withRouteEnabled(true)))

	resObjs := []client.Object{cr, existingRoute}
	sch := makeTestReconcilerSchemeWithRoute()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalRoute(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Route was deleted
	route := &routev1.Route{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: testNamespace,
	}, route)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalRoute_RouteEnabled_RouteDoesNotExist(t *testing.T) {
	// Test case: Route doesn't exist and route is enabled
	// Expected behavior: Should create the Route
	argoutil.SetRouteAPIFound(true)

	cr := makeTestClusterArgoCD(withPrincipalEnabled(true), withRouteEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerSchemeWithRoute()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalRoute(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Route was created
	route := &routev1.Route{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: testNamespace,
	}, route)
	assert.NoError(t, err)

	// Verify Route has expected spec
	expectedSpec := buildPrincipalRouteSpec(testCompName, cr)
	assert.Equal(t, expectedSpec.Port, route.Spec.Port)
	assert.Equal(t, expectedSpec.To, route.Spec.To)
	assert.Equal(t, expectedSpec.TLS, route.Spec.TLS)
}

func TestReconcilePrincipalRoute_RouteEnabled_RouteExists(t *testing.T) {
	// Test case: Route exists and route is enabled
	// Expected behavior: Should keep the Route
	argoutil.SetRouteAPIFound(true)

	cr := makeTestClusterArgoCD(withPrincipalEnabled(true), withRouteEnabled(true))

	// Create existing Route
	existingRoute := makeTestRoute(cr)

	resObjs := []client.Object{cr, existingRoute}
	sch := makeTestReconcilerSchemeWithRoute()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalRoute(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Route still exists
	route := &routev1.Route{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: testNamespace,
	}, route)
	assert.NoError(t, err)

	// Verify Route has expected spec
	expectedSpec := buildPrincipalRouteSpec(testCompName, cr)
	assert.Equal(t, expectedSpec.Port, route.Spec.Port)
	assert.Equal(t, expectedSpec.To, route.Spec.To)
	assert.Equal(t, expectedSpec.TLS, route.Spec.TLS)
}

func TestReconcilePrincipalRoute_RouteToggle_EnabledToDisabled(t *testing.T) {
	// Test case: Route exists and is enabled, then gets disabled
	// Expected behavior: Should delete the Route when disabled
	argoutil.SetRouteAPIFound(true)

	// First create with route enabled
	crEnabled := makeTestClusterArgoCD(withPrincipalEnabled(true), withRouteEnabled(true))
	existingRoute := makeTestRoute(crEnabled)

	resObjs := []client.Object{crEnabled, existingRoute}
	sch := makeTestReconcilerSchemeWithRoute()
	cl := makeTestReconcilerClient(sch, resObjs)

	// Verify route exists initially
	route := &routev1.Route{}
	err := cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(crEnabled.Name, testCompName),
		Namespace: testNamespace,
	}, route)
	assert.NoError(t, err)

	// Now disable the route
	crDisabled := makeTestClusterArgoCD(withPrincipalEnabled(true), withRouteEnabled(false))

	err = ReconcilePrincipalRoute(cl, testCompName, crDisabled, sch)
	assert.NoError(t, err)

	// Verify Route was deleted
	route = &routev1.Route{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(crDisabled.Name, testCompName),
		Namespace: testNamespace,
	}, route)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalRoute_RouteToggle_DisabledToEnabled(t *testing.T) {
	// Test case: Route is disabled, then re-enabled
	// Expected behavior: Should create the Route when enabled
	argoutil.SetRouteAPIFound(true)

	// First ensure no route exists with route disabled
	crDisabled := makeTestClusterArgoCD(withPrincipalEnabled(true), withRouteEnabled(false))

	resObjs := []client.Object{crDisabled}
	sch := makeTestReconcilerSchemeWithRoute()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalRoute(cl, testCompName, crDisabled, sch)
	assert.NoError(t, err)

	// Verify no route exists
	route := &routev1.Route{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(crDisabled.Name, testCompName),
		Namespace: testNamespace,
	}, route)
	assert.True(t, errors.IsNotFound(err))

	// Now enable the route
	crEnabled := makeTestClusterArgoCD(withPrincipalEnabled(true), withRouteEnabled(true))

	err = ReconcilePrincipalRoute(cl, testCompName, crEnabled, sch)
	assert.NoError(t, err)

	// Verify Route was created
	route = &routev1.Route{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(crEnabled.Name, testCompName),
		Namespace: testNamespace,
	}, route)
	assert.NoError(t, err)

	// Verify Route has expected spec
	expectedSpec := buildPrincipalRouteSpec(testCompName, crEnabled)
	assert.Equal(t, expectedSpec.Port, route.Spec.Port)
	assert.Equal(t, expectedSpec.To, route.Spec.To)
	assert.Equal(t, expectedSpec.TLS, route.Spec.TLS)
}

func TestReconcilePrincipalRoute_DefaultBehavior_RouteCreated(t *testing.T) {
	// Test case: Route configuration not explicitly set (nil)
	// Expected behavior: Should create route by default when Route API is available
	argoutil.SetRouteAPIFound(true)

	// Create ArgoCD without explicit route configuration
	cr := makeTestClusterArgoCD(withPrincipalEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerSchemeWithRoute()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalRoute(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Route was created by default
	route := &routev1.Route{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: testNamespace,
	}, route)
	assert.NoError(t, err)

	// Verify Route has expected spec
	expectedSpec := buildPrincipalRouteSpec(testCompName, cr)
	assert.Equal(t, expectedSpec.Port, route.Spec.Port)
	assert.Equal(t, expectedSpec.To, route.Spec.To)
	assert.Equal(t, expectedSpec.TLS, route.Spec.TLS)
}
