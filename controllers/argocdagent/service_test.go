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

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReconcilePrincipalService_ServiceDoesNotExist_PrincipalDisabled(t *testing.T) {
	// Test case: Service doesn't exist and principal is disabled
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestArgoCD(withPrincipalEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalService(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was not created
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, svc)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalService_ServiceDoesNotExist_PrincipalEnabled(t *testing.T) {
	// Test case: Service doesn't exist and principal is enabled
	// Expected behavior: Should create the Service with expected spec

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalService(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was created
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)

	// Verify Service has expected metadata
	expectedName := generateAgentResourceName(cr.Name, testCompName)
	assert.Equal(t, expectedName, svc.Name)
	assert.Equal(t, cr.Namespace, svc.Namespace)
	assert.Equal(t, buildLabelsForAgentPrincipal(cr.Name, testCompName), svc.Labels)

	// Verify Service has expected spec
	expectedSpec := buildPrincipalServiceSpec(testCompName, cr)
	assert.Equal(t, expectedSpec.Type, svc.Spec.Type)
	assert.Equal(t, expectedSpec.Ports, svc.Spec.Ports)
	assert.Equal(t, expectedSpec.Selector, svc.Spec.Selector)

	// Verify specific port configuration using constants
	assert.Len(t, svc.Spec.Ports, 1)
	port := svc.Spec.Ports[0]
	assert.Equal(t, PrincipalServicePortName, port.Name)
	assert.Equal(t, int32(PrincipalServiceHTTPSPort), port.Port)
	assert.Equal(t, intstr.FromInt(PrincipalServiceTargetPort), port.TargetPort)
	assert.Equal(t, corev1.ProtocolTCP, port.Protocol)

	// Verify Service type is LoadBalancer
	assert.Equal(t, corev1.ServiceTypeLoadBalancer, svc.Spec.Type)

	// Verify owner reference is set
	assert.Len(t, svc.OwnerReferences, 1)
	assert.Equal(t, cr.Name, svc.OwnerReferences[0].Name)
	assert.Equal(t, "ArgoCD", svc.OwnerReferences[0].Kind)
}

func TestReconcilePrincipalService_ServiceExists_PrincipalDisabled(t *testing.T) {
	// Test case: Service exists and principal is disabled
	// Expected behavior: Should delete the Service

	cr := makeTestArgoCD(withPrincipalEnabled(false))

	// Create existing Service
	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
		Spec: buildPrincipalServiceSpec(testCompName, cr),
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalService(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was deleted
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, svc)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalService_ServiceExists_PrincipalEnabled_SameSpec(t *testing.T) {
	// Test case: Service exists, principal is enabled, and spec is the same
	// Expected behavior: Should do nothing (no update)

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	expectedSpec := buildPrincipalServiceSpec(testCompName, cr)
	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
		Spec: expectedSpec,
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalService(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service still exists with same spec
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)
	assert.Equal(t, expectedSpec.Type, svc.Spec.Type)
	assert.Equal(t, expectedSpec.Ports, svc.Spec.Ports)
	assert.Equal(t, expectedSpec.Selector, svc.Spec.Selector)
}

func TestReconcilePrincipalService_ServiceExists_PrincipalEnabled_DifferentSpec(t *testing.T) {
	// Test case: Service exists, principal is enabled, but spec is different
	// Expected behavior: Should update the Service with expected spec

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	// Create existing Service with different spec
	differentSpec := corev1.ServiceSpec{
		Type: corev1.ServiceTypeClusterIP, // Different from expected LoadBalancer
		Ports: []corev1.ServicePort{
			{
				Name:       "http", // Different port name
				Port:       80,     // Different port
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(8080), // Different target port
			},
		},
		Selector: map[string]string{
			"app": "different-app", // Different selector
		},
	}

	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
		Spec: differentSpec,
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalService(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was updated with expected spec
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)

	expectedSpec := buildPrincipalServiceSpec(testCompName, cr)
	assert.Equal(t, expectedSpec.Type, svc.Spec.Type)
	assert.Equal(t, expectedSpec.Ports, svc.Spec.Ports)
	assert.Equal(t, expectedSpec.Selector, svc.Spec.Selector)
}

func TestReconcilePrincipalService_ServiceExists_PrincipalNotSet(t *testing.T) {
	// Test case: Service exists but principal spec is not set (nil)
	// Expected behavior: Should delete the Service

	cr := makeTestArgoCD() // No principal configuration

	// Create existing Service
	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
		Spec: buildPrincipalServiceSpec(testCompName, cr),
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalService(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was deleted
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, svc)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalService_ServiceDoesNotExist_AgentNotSet(t *testing.T) {
	// Test case: Service doesn't exist and agent spec is not set (nil)
	// Expected behavior: Should do nothing

	cr := makeTestArgoCD() // No agent configuration

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalService(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was not created
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, svc)
	assert.True(t, errors.IsNotFound(err))
}

// Tests for ReconcilePrincipalMetricsService

func TestReconcilePrincipalMetricsService_ServiceDoesNotExist_PrincipalDisabled(t *testing.T) {
	// Test case: Metrics service doesn't exist and principal is disabled
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestArgoCD(withPrincipalEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalMetricsService(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was not created
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalMetricsService_ServiceDoesNotExist_PrincipalEnabled(t *testing.T) {
	// Test case: Metrics service doesn't exist and principal is enabled
	// Expected behavior: Should create the metrics service with expected spec

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalMetricsService(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was created
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)

	// Verify Service has expected metadata
	expectedName := generateAgentResourceName(cr.Name, testCompName+"-metrics")
	assert.Equal(t, expectedName, svc.Name)
	assert.Equal(t, cr.Namespace, svc.Namespace)
	assert.Equal(t, buildLabelsForAgentPrincipal(cr.Name, testCompName), svc.Labels)

	// Verify Service has expected spec
	expectedSpec := buildPrincipalMetricsServiceSpec(testCompName, cr)
	assert.Equal(t, expectedSpec.Type, svc.Spec.Type)
	assert.Equal(t, expectedSpec.Ports, svc.Spec.Ports)
	assert.Equal(t, expectedSpec.Selector, svc.Spec.Selector)

	// Verify specific port configuration using constants
	assert.Len(t, svc.Spec.Ports, 1)
	metricsPort := svc.Spec.Ports[0]
	assert.Equal(t, PrincipalMetricsServicePortName, metricsPort.Name)
	assert.Equal(t, int32(PrincipalMetricsServicePort), metricsPort.Port)
	assert.Equal(t, intstr.FromInt(PrincipalMetricsServiceTargetPort), metricsPort.TargetPort)
	assert.Equal(t, corev1.ProtocolTCP, metricsPort.Protocol)

	// Verify Service type is LoadBalancer
	assert.Equal(t, corev1.ServiceTypeLoadBalancer, svc.Spec.Type)

	// Verify owner reference is set
	assert.Len(t, svc.OwnerReferences, 1)
	assert.Equal(t, cr.Name, svc.OwnerReferences[0].Name)
	assert.Equal(t, "ArgoCD", svc.OwnerReferences[0].Kind)
}

func TestReconcilePrincipalMetricsService_ServiceExists_PrincipalDisabled(t *testing.T) {
	// Test case: Metrics service exists and principal is disabled
	// Expected behavior: Should delete the metrics service

	cr := makeTestArgoCD(withPrincipalEnabled(false))

	// Create existing Service
	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
		Spec: buildPrincipalMetricsServiceSpec(testCompName, cr),
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalMetricsService(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was deleted
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalMetricsService_ServiceExists_PrincipalEnabled_SameSpec(t *testing.T) {
	// Test case: Metrics service exists, principal is enabled, and spec is the same
	// Expected behavior: Should do nothing (no update)

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	expectedSpec := buildPrincipalMetricsServiceSpec(testCompName, cr)
	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
		Spec: expectedSpec,
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalMetricsService(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service still exists with same spec
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)
	assert.Equal(t, expectedSpec.Type, svc.Spec.Type)
	assert.Equal(t, expectedSpec.Ports, svc.Spec.Ports)
	assert.Equal(t, expectedSpec.Selector, svc.Spec.Selector)
}

func TestReconcilePrincipalMetricsService_ServiceExists_PrincipalEnabled_DifferentSpec(t *testing.T) {
	// Test case: Metrics service exists, principal is enabled, but spec is different
	// Expected behavior: Should update the metrics service with expected spec

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	// Create existing Service with different spec
	differentSpec := corev1.ServiceSpec{
		Type: corev1.ServiceTypeClusterIP, // Different from expected LoadBalancer
		Ports: []corev1.ServicePort{
			{
				Name:       "http", // Different port name
				Port:       80,     // Different port
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(8080), // Different target port
			},
		},
		Selector: map[string]string{
			"app": "different-app", // Different selector
		},
	}

	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
		Spec: differentSpec,
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalMetricsService(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was updated with expected spec
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)

	expectedSpec := buildPrincipalMetricsServiceSpec(testCompName, cr)
	assert.Equal(t, expectedSpec.Type, svc.Spec.Type)
	assert.Equal(t, expectedSpec.Ports, svc.Spec.Ports)
	assert.Equal(t, expectedSpec.Selector, svc.Spec.Selector)
}

func TestReconcilePrincipalMetricsService_ServiceExists_PrincipalNotSet(t *testing.T) {
	// Test case: Metrics service exists but principal spec is not set (nil)
	// Expected behavior: Should delete the metrics service

	cr := makeTestArgoCD() // No principal configuration

	// Create existing Service
	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
		Spec: buildPrincipalMetricsServiceSpec(testCompName, cr),
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalMetricsService(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was deleted
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalMetricsService_ServiceDoesNotExist_AgentNotSet(t *testing.T) {
	// Test case: Metrics service doesn't exist and agent spec is not set (nil)
	// Expected behavior: Should do nothing

	cr := makeTestArgoCD() // No agent configuration

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalMetricsService(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was not created
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalMetricsService_VerifyMetricsServiceSpec(t *testing.T) {
	// Test case: Verify the metrics service spec has correct metrics-specific configuration
	// Expected behavior: Should create service with metrics port and correct selector

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalMetricsService(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was created
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)

	// Verify Service has correct metrics port configuration
	assert.Len(t, svc.Spec.Ports, 1)
	metricsPort := svc.Spec.Ports[0]
	assert.Equal(t, PrincipalMetricsServicePortName, metricsPort.Name)
	assert.Equal(t, int32(PrincipalMetricsServicePort), metricsPort.Port)
	assert.Equal(t, intstr.FromInt(PrincipalMetricsServiceTargetPort), metricsPort.TargetPort)
	assert.Equal(t, corev1.ProtocolTCP, metricsPort.Protocol)

	// Verify Service type is LoadBalancer
	assert.Equal(t, corev1.ServiceTypeLoadBalancer, svc.Spec.Type)

	// Verify selector points to the correct component
	expectedSelector := map[string]string{
		common.ArgoCDKeyName: generateAgentResourceName(cr.Name, testCompName),
	}
	assert.Equal(t, expectedSelector, svc.Spec.Selector)
}

func TestReconcilePrincipalService_VerifyPrincipalServiceSpec(t *testing.T) {
	// Test case: Verify the principal service spec has correct HTTPS port configuration
	// Expected behavior: Should create service with HTTPS port (443) and correct selector

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalService(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was created
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)

	// Verify Service has correct HTTPS port configuration
	assert.Len(t, svc.Spec.Ports, 1)
	httpsPort := svc.Spec.Ports[0]
	assert.Equal(t, PrincipalServicePortName, httpsPort.Name)
	assert.Equal(t, int32(PrincipalServiceHTTPSPort), httpsPort.Port)
	assert.Equal(t, intstr.FromInt(PrincipalServiceTargetPort), httpsPort.TargetPort)
	assert.Equal(t, corev1.ProtocolTCP, httpsPort.Protocol)

	// Verify Service type is LoadBalancer
	assert.Equal(t, corev1.ServiceTypeLoadBalancer, svc.Spec.Type)

	// Verify selector points to the correct component
	expectedSelector := map[string]string{
		common.ArgoCDKeyName: generateAgentResourceName(cr.Name, testCompName),
	}
	assert.Equal(t, expectedSelector, svc.Spec.Selector)
}
