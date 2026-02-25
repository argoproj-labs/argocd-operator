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

	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReconcilePrincipalNetworkPolicy_DoesNotExist_PrincipalDisabled(t *testing.T) {
	cr := makeTestArgoCD(withPrincipalEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalNetworkPolicy(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	np := &networkingv1.NetworkPolicy{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName) + "-network-policy",
		Namespace: cr.Namespace,
	}, np)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalNetworkPolicy_DoesNotExist_PrincipalEnabled_Creates(t *testing.T) {
	cr := makeTestArgoCD(withPrincipalEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalNetworkPolicy(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	np := &networkingv1.NetworkPolicy{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName) + "-network-policy",
		Namespace: cr.Namespace,
	}, np)
	assert.NoError(t, err)

	assert.Equal(t, generateAgentResourceName(cr.Name, testCompName), np.Spec.PodSelector.MatchLabels["app.kubernetes.io/name"])
	assert.Equal(t, 2, len(np.Spec.Ingress))
	assert.Equal(t, 1, len(np.Spec.Ingress[0].From))
	assert.NotNil(t, np.Spec.Ingress[0].From[0].NamespaceSelector)
	assert.Equal(t, 5, len(np.Spec.Ingress[0].Ports))
	assert.Equal(t, "0.0.0.0/0", np.Spec.Ingress[1].From[0].IPBlock.CIDR)
	assert.Equal(t, 2, len(np.Spec.Ingress[1].Ports))
	assert.Equal(t, intstr.FromInt(8443), *np.Spec.Ingress[1].Ports[0].Port)
	assert.Equal(t, intstr.FromInt(443), *np.Spec.Ingress[1].Ports[1].Port)
}

func TestReconcilePrincipalNetworkPolicy_Exists_PrincipalDisabled_Deletes(t *testing.T) {
	cr := makeTestArgoCD(withPrincipalEnabled(true))

	sch := makeTestReconcilerScheme()
	existing := buildPrincipalNetworkPolicy(testCompName, cr)
	existing.Spec = buildPrincipalNetworkPolicySpec(testCompName, cr)

	resObjs := []client.Object{cr, existing}
	cl := makeTestReconcilerClient(sch, resObjs)

	// disable principal and reconcile
	withPrincipalEnabled(false)(cr)

	err := ReconcilePrincipalNetworkPolicy(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	np := &networkingv1.NetworkPolicy{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      existing.Name,
		Namespace: existing.Namespace,
	}, np)
	assert.True(t, errors.IsNotFound(err))
}
