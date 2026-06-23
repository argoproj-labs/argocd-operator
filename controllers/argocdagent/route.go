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

	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// ReconcilePrincipalRoute reconciles the principal route for the ArgoCD agent.
// It creates, updates, or deletes the route based on the principal configuration.
func ReconcilePrincipalRoute(client client.Client, compName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme) error {
	// Verify Route API is available (only if not already checked)
	if !argoutil.IsRouteAPIAvailable() {
		if err := argoutil.VerifyRouteAPI(); err != nil {
			return fmt.Errorf("failed to verify route API: %v", err)
		}
	}

	if !argoutil.IsRouteAPIAvailable() {
		// Route API not available, skip route reconciliation
		return nil
	}

	route := buildRoute(compName, cr)
	expectedSpec := buildPrincipalRouteSpec(compName, cr)

	// Check if the route already exists in the cluster
	exists := true
	if err := client.Get(context.TODO(), types.NamespacedName{Name: route.Name, Namespace: route.Namespace}, route); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get existing principal route %s in namespace %s: %v", route.Name, route.Namespace, err)
		}
		exists = false
	}

	// By default create route if Route API is available
	// Only disable if explicitly configured with Enabled: false
	routeDisabled := false
	if hasPrincipal(cr) &&
		cr.Spec.ArgoCDAgent.Principal.IsEnabled() &&
		cr.Spec.ArgoCDAgent.Principal.Server != nil {
		routeSpec := cr.Spec.ArgoCDAgent.Principal.Server.Route
		if routeSpec.Enabled != nil && !*routeSpec.Enabled {
			routeDisabled = true
		}
	}

	// If route exists, handle updates or deletion
	if exists {
		if (cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled()) ||
			routeDisabled {
			argoutil.LogResourceDeletion(log, route, "principal route is being deleted as principal or principal route is disabled")
			if err := client.Delete(context.TODO(), route); err != nil {
				return fmt.Errorf("failed to delete principal route %s: %v", route.Name, err)
			}
			return nil
		}

		if !reflect.DeepEqual(route.Spec.Port, expectedSpec.Port) ||
			!reflect.DeepEqual(route.Spec.To, expectedSpec.To) ||
			!reflect.DeepEqual(route.Spec.TLS, expectedSpec.TLS) {

			route.Spec.Port = expectedSpec.Port
			route.Spec.To = expectedSpec.To
			route.Spec.TLS = expectedSpec.TLS

			argoutil.LogResourceUpdate(log, route, "updating principal route spec")
			if err := client.Update(context.TODO(), route); err != nil {
				return fmt.Errorf("failed to update principal route %s: %v", route.Name, err)
			}
		}
		return nil
	}

	// If route doesn't exist and principal is disabled or route is disabled, nothing to do
	if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() || routeDisabled {
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, route, scheme); err != nil {
		return fmt.Errorf("failed to set ArgoCD CR %s as owner for route %s: %w", cr.Name, route.Name, err)
	}

	route.Spec = expectedSpec

	argoutil.LogResourceCreation(log, route)
	if err := client.Create(context.TODO(), route); err != nil {
		return fmt.Errorf("failed to create principal route %s: %v", route.Name, err)
	}
	return nil
}

// buildRoute creates a base Route object for the ArgoCD agent principal.
func buildRoute(compName string, cr *argoproj.ArgoCD) *routev1.Route {
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, compName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, compName),
		},
	}
}

// buildPrincipalRouteSpec creates the RouteSpec for the ArgoCD agent principal route.
func buildPrincipalRouteSpec(compName string, cr *argoproj.ArgoCD) routev1.RouteSpec {
	return routev1.RouteSpec{
		Port: &routev1.RoutePort{
			TargetPort: intstr.FromInt(PrincipalServiceTargetPort),
		},
		To: routev1.RouteTargetReference{
			Kind: "Service",
			Name: generateAgentResourceName(cr.Name, compName),
		},
		TLS: &routev1.TLSConfig{
			Termination:                   routev1.TLSTerminationPassthrough,
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone,
		},
	}
}
