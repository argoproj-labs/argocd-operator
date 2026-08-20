// Copyright 2026 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package gitopspromoter handles reconcilation related to the GitOps Promoter
package gitopspromoter

import (
	"context"
	"fmt"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

const (
	// ControllerWebhookPort is the port for the webhook service
	ControllerWebhookPort = 3333
	// ControllerWebhookProtocol is the protocol for the webhook service
	ControllerWebhookProtocol = corev1.ProtocolTCP
	// APIServerPortName is the name of the port for the apiserver service
	APIServerPortName = "https"
	// APIServerPort is the port used by the api server service
	APIServerPort = 443
	// APIServerTargetPort is the target port for the apiserver service
	APIServerTargetPort = "https"
	// APIServerProtocol is the protocol used for the port in the apiserver service
	APIServerProtocol = corev1.ProtocolTCP
)

// ReconcilePromoterControllerWebhookService reconciles the Promoter's Controller Service for the webhook
func ReconcilePromoterControllerWebhookService(client client.Client, compName string, cr *argoproj.ArgoCD) (*corev1.Service, error) {
	expectedSpec := buildControllerWebhookServiceSpec(compName, cr)
	enabled := cr.Spec.Promoter.IsEnabled() && cr.Spec.Promoter.Webhook.IsEnabled()
	return ReconcilePromoterService(client, compName, cr, expectedSpec, enabled)
}

// ReconcilePromoterAPIServerService reconciles the Promoter's Service for the API Server
func ReconcilePromoterAPIServerService(client client.Client, compName string, cr *argoproj.ArgoCD) (*corev1.Service, error) {
	expectedSpec := buildAPIServerServiceSpec(compName)
	enabled := cr.Spec.Promoter == nil || cr.Spec.Promoter.APIServer.IsEnabled()
	return ReconcilePromoterService(client, compName, cr, expectedSpec, enabled)
}

// ReconcilePromoterService is a generic reconcilation function for Services and reconciles based on the provided spec. Handles creation, updating, and deletion.
func ReconcilePromoterService(client client.Client, compName string, cr *argoproj.ArgoCD, expectedSpec corev1.ServiceSpec, enabled bool) (*corev1.Service, error) {
	svc := buildService(compName, cr)

	allowed := argoutil.IsNamespaceClusterConfigNamespace(cr.Namespace)

	exists := true
	if err := argoutil.FetchObject(client, cr.Namespace, svc.Name, svc); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get existing promoter service %s in namespace %s: %v", svc.Name, svc.Namespace, err)
		}
		exists = false
	}

	if exists {
		if !cr.Spec.Promoter.IsEnabled() || !enabled || !allowed {
			argoutil.LogResourceDeletion(log, svc, fmt.Sprintf("promoter service for component %s is being deleted due to being disabled", compName))
			if err := client.Delete(context.Background(), svc); err != nil {
				return nil, fmt.Errorf("failed to delete promoter service %s: %v", svc.Name, err)
			}
			return svc, nil
		}

		if !reflect.DeepEqual(svc.Spec.Selector, expectedSpec.Selector) ||
			!reflect.DeepEqual(svc.Spec.Ports, expectedSpec.Ports) ||
			!reflect.DeepEqual(svc.Spec.Type, expectedSpec.Type) {

			svc.Spec.Selector = expectedSpec.Selector
			svc.Spec.Ports = expectedSpec.Ports
			svc.Spec.Type = expectedSpec.Type

			argoutil.LogResourceUpdate(log, svc)
			if err := client.Update(context.Background(), svc); err != nil {
				return nil, err
			}
			return svc, nil
		}

		return svc, nil
	}

	if !cr.Spec.Promoter.IsEnabled() || !enabled || !allowed {
		return svc, nil
	}

	svc.Spec = expectedSpec
	argoutil.LogResourceCreation(log, svc)
	if err := client.Create(context.Background(), svc); err != nil {
		return nil, fmt.Errorf("failed to create promoter service %s: %v", svc.Name, err)
	}
	return svc, nil
}

// buildService creates a Service object with metadata
func buildService(compName string, cr *argoproj.ArgoCD) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generatePromoterResourceName(compName, cr),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForPromoterResources(compName, cr),
		},
	}
}

// buildControllerWebhookServiceSpec builds the Spec for the Controller's Webhook Service
func buildControllerWebhookServiceSpec(compName string, cr *argoproj.ArgoCD) corev1.ServiceSpec {
	serviceType := corev1.ServiceTypeClusterIP
	if cr.Spec.Promoter != nil && cr.Spec.Promoter.Webhook != nil && cr.Spec.Promoter.Webhook.ServiceType != "" {
		serviceType = corev1.ServiceType(cr.Spec.Promoter.Webhook.ServiceType)
	}

	return corev1.ServiceSpec{
		Selector: map[string]string{
			common.ArgoCDKeyComponent: compName,
		},
		Ports: []corev1.ServicePort{
			{
				Port:       ControllerWebhookPort,
				TargetPort: intstr.FromInt(ControllerWebhookPort),
				Protocol:   ControllerWebhookProtocol,
			},
		},
		Type: serviceType,
	}
}

// buildAPIServerServiceSpec builds the Spec for the API Server Service
func buildAPIServerServiceSpec(compName string) corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Selector: map[string]string{
			common.ArgoCDKeyComponent: compName,
		},
		Ports: []corev1.ServicePort{
			{
				Name:       APIServerPortName,
				Port:       APIServerPort,
				TargetPort: intstr.FromString(APIServerTargetPort),
				Protocol:   APIServerProtocol,
			},
		},
		Type: corev1.ServiceTypeClusterIP,
	}
}
