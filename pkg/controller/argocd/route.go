// Copyright 2019 ArgoCD Operator Developers
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

package argocd

import (
	"context"
	"fmt"

	argoproj "github.com/jmckind/argocd-operator/pkg/apis/argoproj/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// newRoute returns a new Route instance for the given ArgoCD.
func newRoute(cr *argoproj.ArgoCD) *routev1.Route {
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}

// newRouteWithName returns a new Route with the given name and ArgoCD.
func newRouteWithName(name string, cr *argoproj.ArgoCD) *routev1.Route {
	route := newRoute(cr)
	route.ObjectMeta.Name = name
	return route
}

// newRouteWithSuffix returns a new Route with the given name suffix for the ArgoCD.
func newRouteWithSuffix(suffix string, cr *argoproj.ArgoCD) *routev1.Route {
	return newRouteWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), cr)
}

// reconcileRoutes will ensure that all ArgoCD Routes are present.
func (r *ReconcileArgoCD) reconcileRoutes(cr *argoproj.ArgoCD) error {
	if err := r.reconcileGrafanaRoute(cr); err != nil {
		return err
	}

	if err := r.reconcileServerRoute(cr); err != nil {
		return err
	}

	if err := r.reconcilePrometheusRoute(cr); err != nil {
		return err
	}
	return nil
}

// reconcileGrafanaRoute will ensure that the ArgoCD Grafana Route is present.
func (r *ReconcileArgoCD) reconcileGrafanaRoute(cr *argoproj.ArgoCD) error {
	route := newRouteWithSuffix("grafana", cr)
	if r.isObjectFound(cr.Namespace, route.Name, route) {
		return nil // Route found, do nothing
	}

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = nameWithSuffix("grafana", cr)
	route.Spec.Port = &routev1.RoutePort{
		TargetPort: intstr.FromString("http"),
	}

	if err := controllerutil.SetControllerReference(cr, route, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), route)
}

// reconcileServerRoute will ensure that the ArgoCD Server Route is present.
func (r *ReconcileArgoCD) reconcileServerRoute(cr *argoproj.ArgoCD) error {
	route := newRouteWithSuffix("server", cr)
	if r.isObjectFound(cr.Namespace, route.Name, route) {
		return nil // Route found, do nothing
	}

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = nameWithSuffix("server", cr)

	if cr.Spec.TLS.Enabled {
		// TLS enabled, pass through to let ArgoCD handle TLS.
		route.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("https"),
		}
		route.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone,
			Termination:                   routev1.TLSTerminationPassthrough,
		}
	} else {
		// TLS disabled, use edge termination.
		route.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("http"),
		}
		route.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationEdge,
		}
	}

	if err := controllerutil.SetControllerReference(cr, route, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), route)
}

// reconcilePrometheusRoute will ensure that the ArgoCD Prometheus Route is present.
func (r *ReconcileArgoCD) reconcilePrometheusRoute(cr *argoproj.ArgoCD) error {
	route := newRouteWithSuffix("prometheus", cr)
	if r.isObjectFound(cr.Namespace, route.Name, route) {
		return nil // Route found, do nothing
	}

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = "prometheus-operated"
	route.Spec.Port = &routev1.RoutePort{
		TargetPort: intstr.FromString("web"),
	}

	if err := controllerutil.SetControllerReference(cr, route, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), route)
}
