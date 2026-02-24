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
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

const (
	principalNetworkPolicySuffix = "network-policy"
)

// ReconcilePrincipalNetworkPolicy reconciles a NetworkPolicy for the ArgoCD agent principal component.
// It limits inbound traffic to the principal pods to the set of ports that the principal exposes.
func ReconcilePrincipalNetworkPolicy(c client.Client, compName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme) error {
	desired := buildPrincipalNetworkPolicy(compName, cr)
	desired.Spec = buildPrincipalNetworkPolicySpec(compName, cr)

	existing := &networkingv1.NetworkPolicy{}
	key := types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}

	exists := true
	if err := c.Get(context.TODO(), key, existing); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get existing principal network policy %s in namespace %s: %v", desired.Name, cr.Namespace, err)
		}
		exists = false
	}

	enabled := hasPrincipal(cr) && cr.Spec.ArgoCDAgent.Principal.IsEnabled()

	if exists {
		if !cr.Spec.NetworkPolicy.IsEnabled() || !enabled {
			argoutil.LogResourceDeletion(log, existing, "principal network policy is being deleted as principal is disabled or network policy is disabled")
			if err := c.Delete(context.TODO(), existing); err != nil {
				return fmt.Errorf("failed to delete principal network policy %s: %v", existing.Name, err)
			}
			return nil
		}

		if !reflect.DeepEqual(existing.Spec, desired.Spec) {
			existing.Spec = desired.Spec
			argoutil.LogResourceUpdate(log, existing, "updating principal network policy spec")
			if err := c.Update(context.TODO(), existing); err != nil {
				return fmt.Errorf("failed to update principal network policy %s: %v", existing.Name, err)
			}
		}
		return nil
	}

	if !enabled {
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, desired, scheme); err != nil {
		return fmt.Errorf("failed to set ArgoCD CR %s as owner for network policy %s: %w", cr.Name, desired.Name, err)
	}

	argoutil.LogResourceCreation(log, desired)
	if err := c.Create(context.TODO(), desired); err != nil {
		return fmt.Errorf("failed to create principal network policy %s in namespace %s: %v", desired.Name, cr.Namespace, err)
	}

	return nil
}

func buildPrincipalNetworkPolicy(compName string, cr *argoproj.ArgoCD) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", generateAgentResourceName(cr.Name, compName), principalNetworkPolicySuffix),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, compName),
		},
	}
}

func buildPrincipalNetworkPolicySpec(compName string, cr *argoproj.ArgoCD) networkingv1.NetworkPolicySpec {
	tcp := corev1.ProtocolTCP
	targetPorts := []int32{
		PrincipalServiceTargetPort,
		PrincipalMetricsServiceTargetPort,
		PrincipalRedisProxyServiceTargetPort,
		PrincipalResourceProxyServiceTargetPort,
		PrincipalHealthzServiceTargetPort,
	}

	ports := make([]networkingv1.NetworkPolicyPort, 0, len(targetPorts))
	for _, p := range targetPorts {
		ports = append(ports, networkingv1.NetworkPolicyPort{
			Protocol: &tcp,
			Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: p},
		})
	}

	return networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				common.ArgoCDKeyName: generateAgentResourceName(cr.Name, compName),
			},
		},
		PolicyTypes: []networkingv1.PolicyType{
			networkingv1.PolicyTypeIngress,
		},
		Ingress: []networkingv1.NetworkPolicyIngressRule{
			{
				From: []networkingv1.NetworkPolicyPeer{
					{
						NamespaceSelector: &metav1.LabelSelector{},
					},
				},
				Ports: ports,
			},
			{ // Allow traffic from the internet to the principal
				From: []networkingv1.NetworkPolicyPeer{
					{
						IPBlock: &networkingv1.IPBlock{
							CIDR: "0.0.0.0/0",
						},
					},
				},
				Ports: []networkingv1.NetworkPolicyPort{
					{
						Protocol: &tcp,
						Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: int32(8443)},
					},
					{
						Protocol: &tcp,
						Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: int32(443)},
					},
				},
			},
		},
	}
}
