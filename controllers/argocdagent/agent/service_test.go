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

package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

// Metrics Service Tests

func TestReconcileAgentMetricsService_ServiceDoesNotExist_AgentDisabled(t *testing.T) {
	// Test case: Service doesn't exist and agent is disabled
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestArgoCD(withAgentEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentMetricsService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was not created
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentMetricsService_ServiceDoesNotExist_AgentEnabled(t *testing.T) {
	// Test case: Service doesn't exist and agent is enabled
	// Expected behavior: Should create the Service with expected spec

	cr := makeTestArgoCD(withAgentEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentMetricsService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was created
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)

	// Verify Service has expected metadata
	expectedName := generateAgentResourceName(cr.Name, testAgentCompName+"-metrics")
	assert.Equal(t, expectedName, svc.Name)
	assert.Equal(t, cr.Namespace, svc.Namespace)
	assert.Equal(t, buildLabelsForAgent(cr.Name, testAgentCompName), svc.Labels)

	// Verify Service has correct metrics port configuration
	assert.Len(t, svc.Spec.Ports, 1)
	metricsPort := svc.Spec.Ports[0]
	assert.Equal(t, "metrics", metricsPort.Name)
	assert.Equal(t, int32(8181), metricsPort.Port)
	assert.Equal(t, intstr.FromInt(8181), metricsPort.TargetPort)
	assert.Equal(t, corev1.ProtocolTCP, metricsPort.Protocol)

	// Verify Service type is ClusterIP
	assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)

	// Verify owner reference is set
	assert.Len(t, svc.OwnerReferences, 1)
	assert.Equal(t, cr.Name, svc.OwnerReferences[0].Name)
	assert.Equal(t, "ArgoCD", svc.OwnerReferences[0].Kind)
}

func TestReconcileAgentMetricsService_ServiceExists_AgentDisabled(t *testing.T) {
	// Test case: Service exists and agent is disabled
	// Expected behavior: Should delete the Service

	cr := makeTestArgoCD(withAgentEnabled(false))

	// Create existing Service
	expectedSvc := buildService(generateAgentResourceName(cr.Name, testAgentCompName)+"-metrics", testAgentCompName, cr)
	expectedSvc.Spec = buildAgentMetricsServiceSpec(testAgentCompName, cr)
	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-metrics"),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Spec: expectedSvc.Spec,
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentMetricsService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was deleted
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentMetricsService_ServiceExists_AgentEnabled_SameSpec(t *testing.T) {
	// Test case: Metrics service exists, agent is enabled, and spec is the same
	// Expected behavior: Should do nothing (no update)

	cr := makeTestArgoCD(withAgentEnabled(true))

	expectedSvc := buildService(generateAgentResourceName(cr.Name, testAgentCompName)+"-metrics", testAgentCompName, cr)
	expectedSvc.Spec = buildAgentMetricsServiceSpec(testAgentCompName, cr)
	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-metrics"),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Spec: expectedSvc.Spec,
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentMetricsService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service still exists with same spec
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)
	assert.Equal(t, expectedSvc.Spec.Type, svc.Spec.Type)
	assert.Equal(t, expectedSvc.Spec.Ports, svc.Spec.Ports)
	assert.Equal(t, expectedSvc.Spec.Selector, svc.Spec.Selector)
}

func TestReconcileAgentMetricsService_ServiceExists_AgentEnabled_DifferentSpec(t *testing.T) {
	// Test case: Metrics service exists, agent is enabled, but spec is different
	// Expected behavior: Should update the metrics service with expected spec

	cr := makeTestArgoCD(withAgentEnabled(true))

	// Create existing Service with different spec
	differentSpec := corev1.ServiceSpec{
		Type: corev1.ServiceTypeLoadBalancer, // Different from expected ClusterIP
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
			Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-metrics"),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Spec: differentSpec,
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentMetricsService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was updated with expected spec
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)

	expectedSvc := buildService(generateAgentResourceName(cr.Name, testAgentCompName)+"-metrics", testAgentCompName, cr)
	expectedSvc.Spec = buildAgentMetricsServiceSpec(testAgentCompName, cr)
	assert.Equal(t, expectedSvc.Spec.Type, svc.Spec.Type)
	assert.Equal(t, expectedSvc.Spec.Ports, svc.Spec.Ports)
	assert.Equal(t, expectedSvc.Spec.Selector, svc.Spec.Selector)
}

func TestReconcileAgentMetricsService_ServiceExists_AgentNotSet(t *testing.T) {
	// Test case: Metrics service exists but agent spec is not set (nil)
	// Expected behavior: Should delete the metrics service

	cr := makeTestArgoCD() // No agent configuration

	// Create existing Service
	expectedSvc := buildService(generateAgentResourceName(cr.Name, testAgentCompName)+"-metrics", testAgentCompName, cr)
	expectedSvc.Spec = buildAgentMetricsServiceSpec(testAgentCompName, cr)
	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-metrics"),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Spec: expectedSvc.Spec,
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentMetricsService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was deleted
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentMetricsService_ServiceDoesNotExist_AgentNotSet(t *testing.T) {
	// Test case: Metrics service doesn't exist and agent spec is not set (nil)
	// Expected behavior: Should do nothing

	cr := makeTestArgoCD() // No agent configuration

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentMetricsService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was not created
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.True(t, errors.IsNotFound(err))
}

// Healthz Service Tests

func TestReconcileAgentHealthzService_ServiceDoesNotExist_AgentDisabled(t *testing.T) {
	// Test case: Healthz service doesn't exist and agent is disabled
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestArgoCD(withAgentEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentHealthzService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was not created
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-healthz"),
		Namespace: cr.Namespace,
	}, svc)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentHealthzService_ServiceDoesNotExist_AgentEnabled(t *testing.T) {
	// Test case: Healthz service doesn't exist and agent is enabled
	// Expected behavior: Should create the Service with expected spec

	cr := makeTestArgoCD(withAgentEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentHealthzService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was created
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-healthz"),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)

	// Verify Service has expected metadata
	expectedName := generateAgentResourceName(cr.Name, testAgentCompName+"-healthz")
	assert.Equal(t, expectedName, svc.Name)
	assert.Equal(t, cr.Namespace, svc.Namespace)
	assert.Equal(t, buildLabelsForAgent(cr.Name, testAgentCompName), svc.Labels)

	// Verify Service has correct healthz port configuration
	assert.Len(t, svc.Spec.Ports, 1)
	healthzPort := svc.Spec.Ports[0]
	assert.Equal(t, "healthz", healthzPort.Name)
	assert.Equal(t, int32(8002), healthzPort.Port)
	assert.Equal(t, intstr.FromInt(8002), healthzPort.TargetPort)
	assert.Equal(t, corev1.ProtocolTCP, healthzPort.Protocol)

	// Verify Service type is ClusterIP
	assert.Equal(t, corev1.ServiceTypeClusterIP, svc.Spec.Type)

	// Verify owner reference is set
	assert.Len(t, svc.OwnerReferences, 1)
	assert.Equal(t, cr.Name, svc.OwnerReferences[0].Name)
	assert.Equal(t, "ArgoCD", svc.OwnerReferences[0].Kind)
}

func TestReconcileAgentHealthzService_ServiceExists_AgentDisabled(t *testing.T) {
	// Test case: Healthz service exists and agent is disabled
	// Expected behavior: Should delete the Service

	cr := makeTestArgoCD(withAgentEnabled(false))

	// Create existing Service
	expectedSvc := buildService(generateAgentResourceName(cr.Name, testAgentCompName)+"-healthz", testAgentCompName, cr)
	expectedSvc.Spec = buildAgentHealthzServiceSpec(testAgentCompName, cr)
	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-healthz"),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Spec: expectedSvc.Spec,
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentHealthzService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was deleted
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-healthz"),
		Namespace: cr.Namespace,
	}, svc)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentHealthzService_ServiceExists_AgentEnabled_SameSpec(t *testing.T) {
	// Test case: Healthz service exists, agent is enabled, and spec is the same
	// Expected behavior: Should do nothing (no update)

	cr := makeTestArgoCD(withAgentEnabled(true))

	expectedSvc := buildService(generateAgentResourceName(cr.Name, testAgentCompName)+"-healthz", testAgentCompName, cr)
	expectedSvc.Spec = buildAgentHealthzServiceSpec(testAgentCompName, cr)
	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-healthz"),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Spec: expectedSvc.Spec,
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentHealthzService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service still exists with same spec
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-healthz"),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)
	assert.Equal(t, expectedSvc.Spec.Type, svc.Spec.Type)
	assert.Equal(t, expectedSvc.Spec.Ports, svc.Spec.Ports)
	assert.Equal(t, expectedSvc.Spec.Selector, svc.Spec.Selector)
}

func TestReconcileAgentHealthzService_ServiceExists_AgentEnabled_DifferentSpec(t *testing.T) {
	// Test case: Healthz service exists, agent is enabled, but spec is different
	// Expected behavior: Should update the healthz service with expected spec

	cr := makeTestArgoCD(withAgentEnabled(true))

	// Create existing Service with different spec
	differentSpec := corev1.ServiceSpec{
		Type: corev1.ServiceTypeLoadBalancer, // Different from expected ClusterIP
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
			Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-healthz"),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Spec: differentSpec,
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentHealthzService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was updated with expected spec
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-healthz"),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)

	expectedSvc := buildService(generateAgentResourceName(cr.Name, testAgentCompName)+"-healthz", testAgentCompName, cr)
	expectedSvc.Spec = buildAgentHealthzServiceSpec(testAgentCompName, cr)
	assert.Equal(t, expectedSvc.Spec.Type, svc.Spec.Type)
	assert.Equal(t, expectedSvc.Spec.Ports, svc.Spec.Ports)
	assert.Equal(t, expectedSvc.Spec.Selector, svc.Spec.Selector)
}

func TestReconcileAgentHealthzService_ServiceExists_AgentNotSet(t *testing.T) {
	// Test case: Healthz service exists but agent spec is not set (nil)
	// Expected behavior: Should delete the healthz service

	cr := makeTestArgoCD() // No agent configuration

	// Create existing Service
	expectedSvc := buildService(generateAgentResourceName(cr.Name, testAgentCompName)+"-healthz", testAgentCompName, cr)
	expectedSvc.Spec = buildAgentHealthzServiceSpec(testAgentCompName, cr)
	existingSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-healthz"),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Spec: expectedSvc.Spec,
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentHealthzService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was deleted
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-healthz"),
		Namespace: cr.Namespace,
	}, svc)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentHealthzService_ServiceDoesNotExist_AgentNotSet(t *testing.T) {
	// Test case: Healthz service doesn't exist and agent spec is not set (nil)
	// Expected behavior: Should do nothing

	cr := makeTestArgoCD() // No agent configuration

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentHealthzService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was not created
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-healthz"),
		Namespace: cr.Namespace,
	}, svc)
	assert.True(t, errors.IsNotFound(err))
}

// Agent Metrics Service Annotation Tests

func TestReconcileAgentMetricsService_WithAnnotations_NewService(t *testing.T) {
	// Test case: Create new metrics service with annotations
	// Expected behavior: Service should be created with custom annotations

	cr := makeTestArgoCD(withAgentEnabled(true))
	cr.Spec.ArgoCDAgent.Agent.Metrics = &argoproj.ArgoCDMetricsSpec{
		Annotations: map[string]string{
			"ad.datadoghq.com/service.check_names":  `["openmetrics"]`,
			"ad.datadoghq.com/service.init_configs": `[{}]`,
			"custom.io/annotation":                  "test-value",
		},
	}

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentMetricsService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was created with annotations
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)

	// Verify annotations are present
	assert.Equal(t, `["openmetrics"]`, svc.Annotations["ad.datadoghq.com/service.check_names"])
	assert.Equal(t, `[{}]`, svc.Annotations["ad.datadoghq.com/service.init_configs"])
	assert.Equal(t, "test-value", svc.Annotations["custom.io/annotation"])
}

func TestReconcileAgentMetricsService_WithAnnotations_UpdateAnnotationsOnly(t *testing.T) {
	// Test case: Existing service, update only annotations (spec unchanged)
	// Expected behavior: Service should be updated with new annotations

	cr := makeTestArgoCD(withAgentEnabled(true))

	// Create existing service without annotations
	existingSvc := buildService(generateAgentResourceName(cr.Name, testAgentCompName)+"-metrics", testAgentCompName, cr)
	existingSvc.Spec = buildAgentMetricsServiceSpec(testAgentCompName, cr)

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	// Now add annotations to the CR
	cr.Spec.ArgoCDAgent.Agent.Metrics = &argoproj.ArgoCDMetricsSpec{
		Annotations: map[string]string{
			"custom.io/annotation": "new-value",
		},
	}

	err := ReconcileAgentMetricsService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was updated with annotations
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)

	// Verify annotation is present
	assert.Equal(t, "new-value", svc.Annotations["custom.io/annotation"])
}

func TestReconcileAgentMetricsService_WithAnnotations_UpdateBothSpecAndAnnotations(t *testing.T) {
	// Test case: Update both spec and annotations
	// Expected behavior: Both should be updated in single update call

	cr := makeTestArgoCD(withAgentEnabled(true))
	cr.Spec.ArgoCDAgent.Agent.Metrics = &argoproj.ArgoCDMetricsSpec{
		Annotations: map[string]string{
			"custom.io/annotation": "old-value",
		},
	}

	// Create existing service with old annotations and different port
	existingSvc := buildService(generateAgentResourceName(cr.Name, testAgentCompName)+"-metrics", testAgentCompName, cr)
	existingSvc.Spec = corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Name:       "metrics",
				Port:       9999, // Wrong port
				TargetPort: intstr.FromInt(9999),
				Protocol:   corev1.ProtocolTCP,
			},
		},
		Selector: buildLabelsForAgent(cr.Name, testAgentCompName),
		Type:     corev1.ServiceTypeClusterIP,
	}
	existingSvc.Annotations = map[string]string{
		"custom.io/annotation": "old-value",
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	// Update both spec (port) and annotations
	cr.Spec.ArgoCDAgent.Agent.Metrics.Annotations["custom.io/annotation"] = "new-value"

	err := ReconcileAgentMetricsService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was updated
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)

	// Verify both port and annotation were updated
	assert.Equal(t, int32(8181), svc.Spec.Ports[0].Port, "Port should be updated")
	assert.Equal(t, "new-value", svc.Annotations["custom.io/annotation"], "Annotation should be updated")
}

func TestReconcileAgentMetricsService_WithAnnotations_RemoveAnnotations(t *testing.T) {
	// Test case: Remove annotations from spec
	// Expected behavior: Annotations should be removed from service

	cr := makeTestArgoCD(withAgentEnabled(true))

	// Create existing service with annotations
	existingSvc := buildService(generateAgentResourceName(cr.Name, testAgentCompName)+"-metrics", testAgentCompName, cr)
	existingSvc.Spec = buildAgentMetricsServiceSpec(testAgentCompName, cr)
	existingSvc.Annotations = map[string]string{
		"custom.io/annotation": "value-to-remove",
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	// Reconcile without annotations in spec (nil Metrics)
	err := ReconcileAgentMetricsService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service annotations were removed
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)

	// Verify annotation is removed
	_, hasAnnotation := svc.Annotations["custom.io/annotation"]
	assert.False(t, hasAnnotation, "Custom annotation should be removed")
}

func TestReconcileAgentMetricsService_WithAnnotations_PreserveSystemAnnotations(t *testing.T) {
	// Test case: Ensure system annotations are preserved when updating
	// Expected behavior: Kubernetes/OpenShift annotations should not be removed

	cr := makeTestArgoCD(withAgentEnabled(true))
	cr.Spec.ArgoCDAgent.Agent.Metrics = &argoproj.ArgoCDMetricsSpec{
		Annotations: map[string]string{
			"custom.io/annotation": "value",
		},
	}

	// Create existing service with system annotations
	existingSvc := buildService(generateAgentResourceName(cr.Name, testAgentCompName)+"-metrics", testAgentCompName, cr)
	existingSvc.Spec = buildAgentMetricsServiceSpec(testAgentCompName, cr)
	existingSvc.Annotations = map[string]string{
		"service.beta.openshift.io/serving-cert-secret-name": "my-tls-secret",
		"kubernetes.io/service-name":                         "my-service",
	}

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentMetricsService(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Service was updated
	svc := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, svc)
	assert.NoError(t, err)

	// Verify custom annotation was added
	assert.Equal(t, "value", svc.Annotations["custom.io/annotation"])

	// Verify system annotations were preserved
	assert.Equal(t, "my-tls-secret", svc.Annotations["service.beta.openshift.io/serving-cert-secret-name"])
	assert.Equal(t, "my-service", svc.Annotations["kubernetes.io/service-name"])
}
