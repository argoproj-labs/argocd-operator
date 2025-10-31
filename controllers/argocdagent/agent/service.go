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
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

const (
	// AgentMetricsServicePortName is the name of the metrics port
	AgentMetricsServicePortName = "metrics"
	// AgentMetricsServicePort is the external port for the agent metrics service
	AgentMetricsServicePort = 8181
	// AgentMetricsServiceTargetPort is the target port for the agent metrics service
	AgentMetricsServiceTargetPort = 8181
	// AgentHealthzServicePortName is the name of the healthz port
	AgentHealthzServicePortName = "healthz"
	// AgentHealthzServicePort is the external port for the agent healthz service
	AgentHealthzServicePort = 8002
	// AgentHealthzServiceTargetPort is the target port for the agent healthz service
	AgentHealthzServiceTargetPort = 8002
)

// ReconcileAgentMetricsService reconciles the agent metrics service for the ArgoCD agent.
// It creates, updates, or deletes the metrics service based on the agent configuration.
func ReconcileAgentMetricsService(client client.Client, compName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme) error {

	service := buildService(generateAgentResourceName(cr.Name, compName)+"-metrics", compName, cr)
	expectedSpec := buildAgentMetricsServiceSpec(compName, cr)

	// Check if the metrics service already exists in the cluster
	exists := true
	if err := argoutil.FetchObject(client, cr.Namespace, service.Name, service); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get existing agent metrics service %s in namespace %s: %v", service.Name, cr.Namespace, err)
		}
		exists = false
	}

	// If metrics service exists, handle updates or deletion
	if exists {
		if !has(cr) || !cr.Spec.ArgoCDAgent.Agent.IsEnabled() {
			argoutil.LogResourceDeletion(log, service, "agent metrics service is being deleted as agent is disabled")
			if err := client.Delete(context.TODO(), service); err != nil {
				return fmt.Errorf("failed to delete agent metrics service %s: %v", service.Name, err)
			}
			return nil
		}

		if !reflect.DeepEqual(service.Spec.Ports, expectedSpec.Ports) ||
			!reflect.DeepEqual(service.Spec.Selector, expectedSpec.Selector) ||
			!reflect.DeepEqual(service.Spec.Type, expectedSpec.Type) {

			service.Spec.Type = expectedSpec.Type
			service.Spec.Ports = expectedSpec.Ports
			service.Spec.Selector = expectedSpec.Selector

			argoutil.LogResourceUpdate(log, service, "updating agent metrics service spec")
			if err := client.Update(context.TODO(), service); err != nil {
				return fmt.Errorf("failed to update agent metrics service %s: %v", service.Name, err)
			}
		}
		return nil
	}

	// If metrics service doesn't exist and agent is disabled, nothing to do
	if !has(cr) || !cr.Spec.ArgoCDAgent.Agent.IsEnabled() {
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, service, scheme); err != nil {
		return fmt.Errorf("failed to set ArgoCD CR %s as owner for service %s: %w", cr.Name, service.Name, err)
	}

	service.Spec.Type = expectedSpec.Type
	service.Spec.Ports = expectedSpec.Ports
	service.Spec.Selector = expectedSpec.Selector

	argoutil.LogResourceCreation(log, service)
	if err := client.Create(context.TODO(), service); err != nil {
		return fmt.Errorf("failed to create agent metrics service %s: %v", service.Name, err)
	}
	return nil
}

// ReconcileAgentHealthzService reconciles the agent healthz service for the ArgoCD agent.
// It creates, updates, or deletes the healthz service based on the agent configuration.
func ReconcileAgentHealthzService(client client.Client, compName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme) error {

	service := buildService(generateAgentResourceName(cr.Name, compName)+"-healthz", compName, cr)
	expectedSpec := buildAgentHealthzServiceSpec(compName, cr)

	// Check if the healthz service already exists in the cluster
	exists := true
	if err := argoutil.FetchObject(client, cr.Namespace, service.Name, service); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get existing agent healthz service %s in namespace %s: %v", service.Name, cr.Namespace, err)
		}
		exists = false
	}

	// If healthz service exists, handle updates or deletion
	if exists {
		if !has(cr) || !cr.Spec.ArgoCDAgent.Agent.IsEnabled() {
			argoutil.LogResourceDeletion(log, service, "agent healthz service is being deleted as agent is disabled")
			if err := client.Delete(context.TODO(), service); err != nil {
				return fmt.Errorf("failed to delete agent healthz service %s: %v", service.Name, err)
			}
			return nil
		}

		if !reflect.DeepEqual(service.Spec.Ports, expectedSpec.Ports) ||
			!reflect.DeepEqual(service.Spec.Selector, expectedSpec.Selector) ||
			!reflect.DeepEqual(service.Spec.Type, expectedSpec.Type) {

			service.Spec.Type = expectedSpec.Type
			service.Spec.Ports = expectedSpec.Ports
			service.Spec.Selector = expectedSpec.Selector

			argoutil.LogResourceUpdate(log, service, "updating agent healthz service spec")
			if err := client.Update(context.TODO(), service); err != nil {
				return fmt.Errorf("failed to update agent healthz service %s: %v", service.Name, err)
			}
		}
		return nil
	}

	// If healthz service doesn't exist and agent is disabled, nothing to do
	if !has(cr) || !cr.Spec.ArgoCDAgent.Agent.IsEnabled() {
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, service, scheme); err != nil {
		return fmt.Errorf("failed to set ArgoCD CR %s as owner for service %s: %w", cr.Name, service.Name, err)
	}

	service.Spec.Type = expectedSpec.Type
	service.Spec.Ports = expectedSpec.Ports
	service.Spec.Selector = expectedSpec.Selector

	argoutil.LogResourceCreation(log, service)
	if err := client.Create(context.TODO(), service); err != nil {
		return fmt.Errorf("failed to create agent healthz service %s: %v", service.Name, err)
	}
	return nil
}

func buildAgentMetricsServiceSpec(compName string, cr *argoproj.ArgoCD) corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Name:       AgentMetricsServicePortName,
				Port:       AgentMetricsServicePort,
				TargetPort: intstr.FromInt(AgentMetricsServiceTargetPort),
				Protocol:   corev1.ProtocolTCP,
			},
		},
		Selector: buildLabelsForAgent(cr.Name, compName),
		Type:     corev1.ServiceTypeClusterIP,
	}
}

func buildAgentHealthzServiceSpec(compName string, cr *argoproj.ArgoCD) corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Name:       AgentHealthzServicePortName,
				Port:       AgentHealthzServicePort,
				TargetPort: intstr.FromInt(AgentHealthzServiceTargetPort),
				Protocol:   corev1.ProtocolTCP,
			},
		},
		Selector: buildLabelsForAgent(cr.Name, compName),
		Type:     corev1.ServiceTypeClusterIP,
	}
}

func buildService(name, compName string, cr *argoproj.ArgoCD) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, compName),
		},
	}
}
