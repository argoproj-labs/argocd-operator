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
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
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
	// PrincipalServiceHTTPSPort is the external port for the principal service
	PrincipalServiceHTTPSPort = 443
	// PrincipalServiceTargetPort is the target port for the principal service
	PrincipalServiceTargetPort = 8443
	// PrincipalServicePortName is the name of the HTTPS port
	PrincipalServicePortName = "https"
	// PrincipalMetricsServicePortName is the name of the metrics port
	PrincipalMetricsServicePortName = "metrics"
	// PrincipalMetricsServicePort is the external port for the principal metrics service
	PrincipalMetricsServicePort = 8000
	// PrincipalMetricsServiceTargetPort is the target port for the principal metrics service
	PrincipalMetricsServiceTargetPort = 8000
	// PrincipalRedisProxyServicePortName is the name of the Redis proxy port
	PrincipalRedisProxyServicePortName = "redis"
	// PrincipalRedisProxyServicePort is the external port for the principal Redis proxy service
	PrincipalRedisProxyServicePort = 6379
	// PrincipalRedisProxyServiceTargetPort is the target port for the principal Redis proxy service
	PrincipalRedisProxyServiceTargetPort = 6379
	// PrincipalResourceProxyServicePortName is the name of the resource proxy port
	PrincipalResourceProxyServicePortName = "resource-proxy"
	// PrincipalResourceProxyServicePort is the external port for the principal resource proxy service
	PrincipalResourceProxyServicePort = 9090
	// PrincipalResourceProxyServiceTargetPort is the target port for the principal resource proxy service
	PrincipalResourceProxyServiceTargetPort = 9090
	// PrincipalHealthzServicePortName is the name of the healthz port
	PrincipalHealthzServicePortName = "healthz"
	// PrincipalHealthzServicePort is the external port for the principal healthz service
	PrincipalHealthzServicePort = 8003
	// PrincipalHealthzServiceTargetPort is the target port for the principal healthz service
	PrincipalHealthzServiceTargetPort = 8003
)

// ReconcilePrincipalService reconciles the principal service for the ArgoCD agent.
// It creates, updates, or deletes the service based on the principal configuration.
func ReconcilePrincipalService(client client.Client, compName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme) error {
	service := buildService(generateAgentResourceName(cr.Name, compName), compName, cr)
	expectedSpec := buildPrincipalServiceSpec(compName, cr)

	// Check if the service already exists in the cluster
	exists := true
	if err := client.Get(context.TODO(), types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, service); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get existing principal service %s in namespace %s: %v", service.Name, service.Namespace, err)
		}
		exists = false
	}

	// If service exists, handle updates or deletion
	if exists {
		if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
			argoutil.LogResourceDeletion(log, service, "principal service is being deleted as principal is disabled")
			if err := client.Delete(context.TODO(), service); err != nil {
				return fmt.Errorf("failed to delete principal service %s: %v", service.Name, err)
			}
			return nil
		}

		if !reflect.DeepEqual(service.Spec.Ports, expectedSpec.Ports) ||
			!reflect.DeepEqual(service.Spec.Selector, expectedSpec.Selector) ||
			!reflect.DeepEqual(service.Spec.Type, expectedSpec.Type) {

			service.Spec.Type = expectedSpec.Type
			service.Spec.Ports = expectedSpec.Ports
			service.Spec.Selector = expectedSpec.Selector

			argoutil.LogResourceUpdate(log, service, "updating principal service spec")
			if err := client.Update(context.TODO(), service); err != nil {
				return fmt.Errorf("failed to update principal service %s: %v", service.Name, err)
			}
		}
		return nil
	}

	// If service doesn't exist and principal is disabled, nothing to do
	if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
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
		return fmt.Errorf("failed to create principal service %s: %v", service.Name, err)
	}
	return nil
}

// ReconcilePrincipalMetricsService reconciles the principal metrics service for the ArgoCD agent.
// It creates, updates, or deletes the metrics service based on the principal configuration.
func ReconcilePrincipalMetricsService(client client.Client, compName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme) error {

	service := buildService(generateAgentResourceName(cr.Name, compName+"-metrics"), compName, cr)
	expectedSpec := buildPrincipalMetricsServiceSpec(compName, cr)

	// Check if the metrics service already exists in the cluster
	exists := true
	if err := client.Get(context.TODO(), types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, service); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get existing principal metrics service %s in namespace %s: %v", service.Name, service.Namespace, err)
		}
		exists = false
	}

	// If metrics service exists, handle updates or deletion
	if exists {
		if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
			argoutil.LogResourceDeletion(log, service, "principal metrics service is being deleted as principal is disabled")
			if err := client.Delete(context.TODO(), service); err != nil {
				return fmt.Errorf("failed to delete principal metrics service %s: %v", service.Name, err)
			}
			return nil
		}

		if !reflect.DeepEqual(service.Spec.Ports, expectedSpec.Ports) ||
			!reflect.DeepEqual(service.Spec.Selector, expectedSpec.Selector) ||
			!reflect.DeepEqual(service.Spec.Type, expectedSpec.Type) {

			service.Spec.Type = expectedSpec.Type
			service.Spec.Ports = expectedSpec.Ports
			service.Spec.Selector = expectedSpec.Selector

			argoutil.LogResourceUpdate(log, service, "updating principal metrics service spec")
			if err := client.Update(context.TODO(), service); err != nil {
				return fmt.Errorf("failed to update principal metrics service %s: %v", service.Name, err)
			}
		}
		return nil
	}

	// If metrics service doesn't exist and principal is disabled, nothing to do
	if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
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
		return fmt.Errorf("failed to create principal metrics service %s: %v", service.Name, err)
	}
	return nil
}

// ReconcilePrincipalRedisProxyService reconciles the principal Redis proxy service for the ArgoCD agent.
// It creates, updates, or deletes the Redis proxy service based on the principal configuration.
func ReconcilePrincipalRedisProxyService(client client.Client, compName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme) error {

	service := buildService(argoutil.GenerateAgentPrincipalRedisProxyServiceName(cr.Name), compName, cr)
	expectedSpec := buildPrincipalRedisProxyServiceSpec(compName, cr)

	// Check if the Redis proxy service already exists in the cluster
	exists := true
	if err := client.Get(context.TODO(), types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, service); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get existing principal Redis proxy service %s in namespace %s: %v", service.Name, service.Namespace, err)
		}
		exists = false
	}

	// If Redis proxy service exists, handle updates or deletion
	if exists {
		if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
			argoutil.LogResourceDeletion(log, service, "principal Redis proxy service is being deleted as principal is disabled")
			if err := client.Delete(context.TODO(), service); err != nil {
				return fmt.Errorf("failed to delete principal Redis proxy service %s: %v", service.Name, err)
			}
			return nil
		}

		if !reflect.DeepEqual(service.Spec.Ports, expectedSpec.Ports) ||
			!reflect.DeepEqual(service.Spec.Selector, expectedSpec.Selector) ||
			!reflect.DeepEqual(service.Spec.Type, expectedSpec.Type) {

			service.Spec.Type = expectedSpec.Type
			service.Spec.Ports = expectedSpec.Ports
			service.Spec.Selector = expectedSpec.Selector

			argoutil.LogResourceUpdate(log, service, "updating principal Redis proxy service spec")
			if err := client.Update(context.TODO(), service); err != nil {
				return fmt.Errorf("failed to update principal Redis proxy service %s: %v", service.Name, err)
			}
		}
		return nil
	}

	// If Redis proxy service doesn't exist and principal is disabled, nothing to do
	if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
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
		return fmt.Errorf("failed to create principal Redis proxy service %s: %v", service.Name, err)
	}
	return nil
}

// ReconcilePrincipalResourceProxyService reconciles the principal resource proxy service for the ArgoCD agent.
// It creates, updates, or deletes the resource proxy service based on the principal configuration.
func ReconcilePrincipalResourceProxyService(client client.Client, compName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme) error {

	service := buildService(generateAgentResourceName(cr.Name, compName+"-resource-proxy"), compName, cr)
	expectedSpec := buildPrincipalResourceProxyServiceSpec(compName, cr)

	// Check if the resource proxy service already exists in the cluster
	exists := true
	if err := client.Get(context.TODO(), types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, service); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get existing principal resource proxy service %s in namespace %s: %v", service.Name, service.Namespace, err)
		}
		exists = false
	}

	// If resource proxy service exists, handle updates or deletion
	if exists {
		if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
			argoutil.LogResourceDeletion(log, service, "principal resource proxy service is being deleted as principal is disabled")
			if err := client.Delete(context.TODO(), service); err != nil {
				return fmt.Errorf("failed to delete principal resource proxy service %s: %v", service.Name, err)
			}
			return nil
		}

		if !reflect.DeepEqual(service.Spec.Ports, expectedSpec.Ports) ||
			!reflect.DeepEqual(service.Spec.Selector, expectedSpec.Selector) ||
			!reflect.DeepEqual(service.Spec.Type, expectedSpec.Type) {

			service.Spec.Type = expectedSpec.Type
			service.Spec.Ports = expectedSpec.Ports
			service.Spec.Selector = expectedSpec.Selector

			argoutil.LogResourceUpdate(log, service, "updating principal resource proxy service spec")
			if err := client.Update(context.TODO(), service); err != nil {
				return fmt.Errorf("failed to update principal resource proxy service %s: %v", service.Name, err)
			}
		}
		return nil
	}

	// If resource proxy service doesn't exist and principal is disabled, nothing to do
	if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
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
		return fmt.Errorf("failed to create principal resource proxy service %s: %v", service.Name, err)
	}
	return nil
}

// ReconcilePrincipalHealthzService reconciles the principal healthz service for the ArgoCD agent.
// It creates, updates, or deletes the healthz service based on the principal configuration.
func ReconcilePrincipalHealthzService(client client.Client, compName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme) error {

	service := buildService(generateAgentResourceName(cr.Name, compName+"-healthz"), compName, cr)
	expectedSpec := buildPrincipalHealthzServiceSpec(compName, cr)

	// Check if the healthz service already exists in the cluster
	exists := true
	if err := client.Get(context.TODO(), types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, service); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get existing principal healthz service %s in namespace %s: %v", service.Name, service.Namespace, err)
		}
		exists = false
	}

	// If healthz service exists, handle updates or deletion
	if exists {
		if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
			argoutil.LogResourceDeletion(log, service, "principal healthz service is being deleted as principal is disabled")
			if err := client.Delete(context.TODO(), service); err != nil {
				return fmt.Errorf("failed to delete principal healthz service %s: %v", service.Name, err)
			}
			return nil
		}

		if !reflect.DeepEqual(service.Spec.Ports, expectedSpec.Ports) ||
			!reflect.DeepEqual(service.Spec.Selector, expectedSpec.Selector) ||
			!reflect.DeepEqual(service.Spec.Type, expectedSpec.Type) {

			service.Spec.Type = expectedSpec.Type
			service.Spec.Ports = expectedSpec.Ports
			service.Spec.Selector = expectedSpec.Selector

			argoutil.LogResourceUpdate(log, service, "updating principal healthz service spec")
			if err := client.Update(context.TODO(), service); err != nil {
				return fmt.Errorf("failed to update principal healthz service %s: %v", service.Name, err)
			}
		}
		return nil
	}

	// If healthz service doesn't exist and principal is disabled, nothing to do
	if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
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
		return fmt.Errorf("failed to create principal healthz service %s: %v", service.Name, err)
	}
	return nil
}

func buildPrincipalServiceSpec(compName string, cr *argoproj.ArgoCD) corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Name:       PrincipalServicePortName,
				Port:       PrincipalServiceHTTPSPort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(PrincipalServiceTargetPort),
			},
		},
		Selector: map[string]string{
			common.ArgoCDKeyName: generateAgentResourceName(cr.Name, compName),
		},
		Type: getPrincipalServiceType(cr),
	}
}

func buildPrincipalMetricsServiceSpec(compName string, cr *argoproj.ArgoCD) corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Name:       PrincipalMetricsServicePortName,
				Port:       PrincipalMetricsServicePort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(PrincipalMetricsServiceTargetPort),
			},
		},
		Selector: map[string]string{
			common.ArgoCDKeyName: generateAgentResourceName(cr.Name, compName),
		},
		Type: corev1.ServiceTypeClusterIP,
	}
}

func buildPrincipalRedisProxyServiceSpec(compName string, cr *argoproj.ArgoCD) corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Name:       PrincipalRedisProxyServicePortName,
				Port:       PrincipalRedisProxyServicePort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(PrincipalRedisProxyServiceTargetPort),
			},
		},
		Selector: map[string]string{
			common.ArgoCDKeyName: generateAgentResourceName(cr.Name, compName),
		},
		Type: corev1.ServiceTypeClusterIP,
	}
}

func buildPrincipalResourceProxyServiceSpec(compName string, cr *argoproj.ArgoCD) corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Name:       PrincipalResourceProxyServicePortName,
				Port:       PrincipalResourceProxyServicePort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(PrincipalResourceProxyServiceTargetPort),
			},
		},
		Selector: map[string]string{
			common.ArgoCDKeyName: generateAgentResourceName(cr.Name, compName),
		},
		Type: corev1.ServiceTypeClusterIP,
	}
}

func buildPrincipalHealthzServiceSpec(compName string, cr *argoproj.ArgoCD) corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Ports: []corev1.ServicePort{
			{
				Name:       PrincipalHealthzServicePortName,
				Port:       PrincipalHealthzServicePort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(PrincipalHealthzServiceTargetPort),
			},
		},
		Selector: map[string]string{
			common.ArgoCDKeyName: generateAgentResourceName(cr.Name, compName),
		},
		Type: corev1.ServiceTypeClusterIP,
	}
}

func buildService(name, compName string, cr *argoproj.ArgoCD) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, compName),
		},
	}
}

// getPrincipalServiceType will return the principal service type.
func getPrincipalServiceType(cr *argoproj.ArgoCD) corev1.ServiceType {
	if cr.Spec.ArgoCDAgent != nil &&
		cr.Spec.ArgoCDAgent.Principal != nil &&
		cr.Spec.ArgoCDAgent.Principal.Server != nil &&
		len(cr.Spec.ArgoCDAgent.Principal.Server.Service.Type) > 0 {
		return cr.Spec.ArgoCDAgent.Principal.Server.Service.Type
	}
	return corev1.ServiceTypeClusterIP
}
