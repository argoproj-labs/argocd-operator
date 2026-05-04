// Copyright 2026 ArgoCD Operator Developers
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

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
)

func makeTestReconcilerSchemeWithMonitoring() *runtime.Scheme {
	s := scheme.Scheme
	_ = argoproj.AddToScheme(s)
	_ = monitoringv1.AddToScheme(s)
	return s
}

func TestReconcilePrincipalServiceMonitor_ServiceMonitorDoesNotExist_PrometheusDisabled(t *testing.T) {
	// Test case: ServiceMonitor doesn't exist, principal is enabled but prometheus is disabled
	// Expected behavior: Should do nothing

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerSchemeWithMonitoring()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalServiceMonitor(cl, testCompName, cr, sch, false)
	assert.NoError(t, err)

	sm := &monitoringv1.ServiceMonitor{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, sm)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalServiceMonitor_ServiceMonitorDoesNotExist_PrincipalDisabled(t *testing.T) {
	// Test case: ServiceMonitor doesn't exist, prometheus is enabled but principal is disabled
	// Expected behavior: Should do nothing

	cr := makeTestArgoCD(withPrincipalEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerSchemeWithMonitoring()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalServiceMonitor(cl, testCompName, cr, sch, true)
	assert.NoError(t, err)

	sm := &monitoringv1.ServiceMonitor{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, sm)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalServiceMonitor_ServiceMonitorDoesNotExist_PrincipalEnabled(t *testing.T) {
	// Test case: ServiceMonitor doesn't exist, both principal and prometheus are enabled
	// Expected behavior: Should create the ServiceMonitor with expected labels, selector, endpoints and owner reference

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerSchemeWithMonitoring()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalServiceMonitor(cl, testCompName, cr, sch, true)
	assert.NoError(t, err)

	sm := &monitoringv1.ServiceMonitor{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, sm)
	assert.NoError(t, err)

	// Verify ServiceMonitor has expected metadata
	expectedName := generateAgentResourceName(cr.Name, testCompName+"-metrics")
	assert.Equal(t, expectedName, sm.Name)
	assert.Equal(t, cr.Namespace, sm.Namespace)

	expectedLabels := buildLabelsForAgentPrincipal(cr.Name, testCompName)
	expectedLabels[common.ArgoCDKeyRelease] = "prometheus-operator"
	assert.Equal(t, expectedLabels, sm.Labels)

	// Verify ServiceMonitor has expected spec
	assert.Equal(t, PrincipalMetricsServicePortName, sm.Spec.Endpoints[0].Port)
	assert.Equal(t, map[string]string{
		common.ArgoCDKeyName: generateAgentResourceName(cr.Name, testCompName),
	}, sm.Spec.Selector.MatchLabels)

	// Verify owner reference is set
	assert.Len(t, sm.OwnerReferences, 1)
	assert.Equal(t, cr.Name, sm.OwnerReferences[0].Name)
	assert.Equal(t, "ArgoCD", sm.OwnerReferences[0].Kind)
}

func TestReconcilePrincipalServiceMonitor_ServiceMonitorExists_PrometheusDisabled(t *testing.T) {
	// Test case: ServiceMonitor exists but prometheus is disabled
	// Expected behavior: Should delete the ServiceMonitor

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	existingSM := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
			Namespace: cr.Namespace,
		},
	}

	resObjs := []client.Object{cr, existingSM}
	sch := makeTestReconcilerSchemeWithMonitoring()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalServiceMonitor(cl, testCompName, cr, sch, false)
	assert.NoError(t, err)

	sm := &monitoringv1.ServiceMonitor{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, sm)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalServiceMonitor_ServiceMonitorExists_PrincipalDisabled(t *testing.T) {
	// Test case: ServiceMonitor exists but principal is disabled
	// Expected behavior: Should delete the ServiceMonitor

	cr := makeTestArgoCD(withPrincipalEnabled(false))

	existingSM := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
			Namespace: cr.Namespace,
		},
	}

	resObjs := []client.Object{cr, existingSM}
	sch := makeTestReconcilerSchemeWithMonitoring()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalServiceMonitor(cl, testCompName, cr, sch, true)
	assert.NoError(t, err)

	sm := &monitoringv1.ServiceMonitor{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, sm)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalServiceMonitor_ServiceMonitorExists_PrincipalEnabled(t *testing.T) {
	// Test case: ServiceMonitor exists and both principal and prometheus are enabled
	// Expected behavior: Should do nothing, keep existing ServiceMonitor

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	existingSM := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
			Namespace: cr.Namespace,
		},
	}

	resObjs := []client.Object{cr, existingSM}
	sch := makeTestReconcilerSchemeWithMonitoring()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalServiceMonitor(cl, testCompName, cr, sch, true)
	assert.NoError(t, err)

	sm := &monitoringv1.ServiceMonitor{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, sm)
	assert.NoError(t, err)
}

func TestReconcilePrincipalServiceMonitor_ServiceMonitorExists_PrincipalEnabled_DifferentSpec(t *testing.T) {
	// Test case: ServiceMonitor exists with different spec, both principal and prometheus are enabled
	// Expected behavior: Should update the ServiceMonitor with expected spec

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	existingSM := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
			Namespace: cr.Namespace,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "wrong-selector",
				},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port: "wrong-port",
				},
			},
		},
	}

	resObjs := []client.Object{cr, existingSM}
	sch := makeTestReconcilerSchemeWithMonitoring()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalServiceMonitor(cl, testCompName, cr, sch, true)
	assert.NoError(t, err)

	sm := &monitoringv1.ServiceMonitor{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, sm)
	assert.NoError(t, err)

	// Verify spec was updated to expected values
	assert.Equal(t, PrincipalMetricsServicePortName, sm.Spec.Endpoints[0].Port)
	assert.Equal(t, map[string]string{
		common.ArgoCDKeyName: generateAgentResourceName(cr.Name, testCompName),
	}, sm.Spec.Selector.MatchLabels)
}

func TestReconcilePrincipalServiceMonitor_ServiceMonitorDoesNotExist_PrincipalNotSet(t *testing.T) {
	// Test case: ServiceMonitor doesn't exist and principal spec is not set (nil)
	// Expected behavior: Should do nothing

	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerSchemeWithMonitoring()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalServiceMonitor(cl, testCompName, cr, sch, true)
	assert.NoError(t, err)

	sm := &monitoringv1.ServiceMonitor{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, sm)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalServiceMonitor_ServiceMonitorExists_PrincipalNotSet(t *testing.T) {
	// Test case: ServiceMonitor exists but principal spec is not set (nil)
	// Expected behavior: Should delete the ServiceMonitor

	cr := makeTestArgoCD()

	existingSM := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
			Namespace: cr.Namespace,
		},
	}

	resObjs := []client.Object{cr, existingSM}
	sch := makeTestReconcilerSchemeWithMonitoring()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalServiceMonitor(cl, testCompName, cr, sch, true)
	assert.NoError(t, err)

	sm := &monitoringv1.ServiceMonitor{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName+"-metrics"),
		Namespace: cr.Namespace,
	}, sm)
	assert.True(t, errors.IsNotFound(err))
}
