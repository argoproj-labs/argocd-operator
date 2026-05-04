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

package agent

import (
	"context"
	"fmt"
	"reflect"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

func ReconcileAgentServiceMonitor(client client.Client, compName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme, prometheusEnabled bool) error {
	smName := generateAgentResourceName(cr.Name, compName) + "-metrics"

	sm := &monitoringv1.ServiceMonitor{}
	exists, err := argoutil.IsObjectFound(client, cr.Namespace, smName, sm)
	if err != nil {
		return err
	}

	if exists {
		if !prometheusEnabled || !hasAgent(cr) || !cr.Spec.ArgoCDAgent.Agent.IsEnabled() {
			argoutil.LogResourceDeletion(log, sm, "agent ServiceMonitor is being deleted as agent or prometheus is disabled")
			if err := client.Delete(context.TODO(), sm); err != nil {
				return fmt.Errorf("failed to delete agent ServiceMonitor %s: %w", smName, err)
			}
			return nil
		}

		expected := buildAgentServiceMonitor(smName, compName, cr)
		if !reflect.DeepEqual(sm.Spec.Endpoints, expected.Spec.Endpoints) ||
			!reflect.DeepEqual(sm.Spec.Selector, expected.Spec.Selector) {

			sm.Spec.Endpoints = expected.Spec.Endpoints
			sm.Spec.Selector = expected.Spec.Selector

			argoutil.LogResourceUpdate(log, sm, "updating agent ServiceMonitor spec")
			if err := client.Update(context.TODO(), sm); err != nil {
				return fmt.Errorf("failed to update agent ServiceMonitor %s: %w", smName, err)
			}
		}
		return nil
	}

	if !prometheusEnabled || !hasAgent(cr) || !cr.Spec.ArgoCDAgent.Agent.IsEnabled() {
		return nil // Prometheus not enabled or agent disabled, do nothing.
	}

	sm = buildAgentServiceMonitor(smName, compName, cr)

	if err := controllerutil.SetControllerReference(cr, sm, scheme); err != nil {
		return fmt.Errorf("failed to set ArgoCD CR %s as owner for ServiceMonitor %s: %w", cr.Name, smName, err)
	}

	argoutil.LogResourceCreation(log, sm)
	if err := client.Create(context.TODO(), sm); err != nil {
		return fmt.Errorf("failed to create agent ServiceMonitor %s: %w", smName, err)
	}
	return nil
}

func buildAgentServiceMonitor(name, compName string, cr *argoproj.ArgoCD) *monitoringv1.ServiceMonitor {
	lbls := buildLabelsForAgent(cr.Name, compName)
	lbls[common.ArgoCDKeyRelease] = "prometheus-operator"

	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
			Labels:    lbls,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					common.ArgoCDKeyName: generateAgentResourceName(cr.Name, compName),
				},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port: AgentMetricsServicePortName,
				},
			},
		},
	}
}
