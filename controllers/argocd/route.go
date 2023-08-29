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

	routev1 "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

var routeAPIFound = false

// IsRouteAPIAvailable returns true if the Route API is present.
func IsRouteAPIAvailable() bool {
	return routeAPIFound
}

// verifyRouteAPI will verify that the Route API is present.
func verifyRouteAPI() error {
	found, err := argoutil.VerifyAPI(routev1.GroupName, routev1.GroupVersion.Version)
	if err != nil {
		return err
	}
	routeAPIFound = found
	return nil
}

// newRoute returns a new Route instance for the given ArgoCD.
func newRoute(cr *argoproj.ArgoCD) *routev1.Route {
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
	}
}

// newRouteWithName returns a new Route with the given name and ArgoCD.
func newRouteWithName(name string, cr *argoproj.ArgoCD) *routev1.Route {
	route := newRoute(cr)
	route.ObjectMeta.Name = name

	lbls := route.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	route.ObjectMeta.Labels = lbls

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

	if err := r.reconcilePrometheusRoute(cr); err != nil {
		return err
	}

	if err := r.reconcileServerRoute(cr); err != nil {
		return err
	}

	if err := r.reconcileApplicationSetControllerWebhookRoute(cr); err != nil {
		return err
	}

	return nil
}

// reconcileGrafanaRoute will ensure that the ArgoCD Grafana Route is present.
func (r *ReconcileArgoCD) reconcileGrafanaRoute(cr *argoproj.ArgoCD) error {
	route := newRouteWithSuffix("grafana", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, route.Name, route) {
		if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Route.Enabled {
			// Route exists but enabled flag has been set to false, delete the Route
			return r.Client.Delete(context.TODO(), route)
		}
		return nil // Route found, do nothing
	}

	if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Route.Enabled {
		return nil // Grafana itself or Route not enabled, do nothing.
	}

	// Allow override of the Annotations for the Route.
	if len(cr.Spec.Grafana.Route.Annotations) > 0 {
		route.Annotations = cr.Spec.Grafana.Route.Annotations
	}

	// Allow override of the Labels for the Route.
	if len(cr.Spec.Grafana.Route.Labels) > 0 {
		labels := route.Labels
		for key, val := range cr.Spec.Grafana.Route.Labels {
			labels[key] = val
		}
		route.Labels = labels
	}

	// Allow override of the Host for the Route.
	if len(cr.Spec.Grafana.Host) > 0 {
		route.Spec.Host = cr.Spec.Grafana.Host // TODO: What additional role needed for this?
	}

	// Allow override of the Path for the Route
	if len(cr.Spec.Grafana.Route.Path) > 0 {
		route.Spec.Path = cr.Spec.Grafana.Route.Path
	}

	route.Spec.Port = &routev1.RoutePort{
		TargetPort: intstr.FromString("http"),
	}

	// Allow override of TLS options for the Route
	if cr.Spec.Grafana.Route.TLS != nil {
		route.Spec.TLS = cr.Spec.Grafana.Route.TLS
	}

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = nameWithSuffix("grafana", cr)

	// Allow override of the WildcardPolicy for the Route
	if cr.Spec.Grafana.Route.WildcardPolicy != nil && len(*cr.Spec.Grafana.Route.WildcardPolicy) > 0 {
		route.Spec.WildcardPolicy = *cr.Spec.Grafana.Route.WildcardPolicy
	}

	if err := controllerutil.SetControllerReference(cr, route, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), route)
}

// reconcilePrometheusRoute will ensure that the ArgoCD Prometheus Route is present.
func (r *ReconcileArgoCD) reconcilePrometheusRoute(cr *argoproj.ArgoCD) error {
	route := newRouteWithSuffix("prometheus", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, route.Name, route) {
		if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Route.Enabled {
			// Route exists but enabled flag has been set to false, delete the Route
			return r.Client.Delete(context.TODO(), route)
		}
		return nil // Route found, do nothing
	}

	if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Route.Enabled {
		return nil // Prometheus itself or Route not enabled, do nothing.
	}

	// Allow override of the Annotations for the Route.
	if len(cr.Spec.Prometheus.Route.Annotations) > 0 {
		route.Annotations = cr.Spec.Prometheus.Route.Annotations
	}

	// Allow override of the Labels for the Route.
	if len(cr.Spec.Prometheus.Route.Labels) > 0 {
		labels := route.Labels
		for key, val := range cr.Spec.Prometheus.Route.Labels {
			labels[key] = val
		}
		route.Labels = labels
	}

	// Allow override of the Host for the Route.
	if len(cr.Spec.Prometheus.Host) > 0 {
		route.Spec.Host = cr.Spec.Prometheus.Host // TODO: What additional role needed for this?
	}

	route.Spec.Port = &routev1.RoutePort{
		TargetPort: intstr.FromString("web"),
	}

	// Allow override of TLS options for the Route
	if cr.Spec.Prometheus.Route.TLS != nil {
		route.Spec.TLS = cr.Spec.Prometheus.Route.TLS
	}

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = "prometheus-operated"

	// Allow override of the WildcardPolicy for the Route
	if cr.Spec.Prometheus.Route.WildcardPolicy != nil && len(*cr.Spec.Prometheus.Route.WildcardPolicy) > 0 {
		route.Spec.WildcardPolicy = *cr.Spec.Prometheus.Route.WildcardPolicy
	}

	if err := controllerutil.SetControllerReference(cr, route, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), route)
}

// reconcileServerRoute will ensure that the ArgoCD Server Route is present.
func (r *ReconcileArgoCD) reconcileServerRoute(cr *argoproj.ArgoCD) error {

	route := newRouteWithSuffix("server", cr)
	found := argoutil.IsObjectFound(r.Client, cr.Namespace, route.Name, route)
	if found {
		if !cr.Spec.Server.Route.Enabled {
			// Route exists but enabled flag has been set to false, delete the Route
			return r.Client.Delete(context.TODO(), route)
		}
	}

	if !cr.Spec.Server.Route.Enabled {
		return nil // Route not enabled, move along...
	}

	// Allow override of the Annotations for the Route.
	if len(cr.Spec.Server.Route.Annotations) > 0 {
		route.Annotations = cr.Spec.Server.Route.Annotations
	}

	// Allow override of the Labels for the Route.
	if len(cr.Spec.Server.Route.Labels) > 0 {
		labels := route.Labels
		for key, val := range cr.Spec.Server.Route.Labels {
			labels[key] = val
		}
		route.Labels = labels
	}

	// Allow override of the Host for the Route.
	if len(cr.Spec.Server.Host) > 0 {
		route.Spec.Host = cr.Spec.Server.Host // TODO: What additional role needed for this?
	}

	if cr.Spec.Server.Insecure {
		// Disable TLS and rely on the cluster certificate.
		route.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("http"),
		}
		route.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationEdge,
		}
	} else {
		// Server is using TLS configure passthrough.
		route.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("https"),
		}
		route.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationPassthrough,
		}
	}

	// Allow override of TLS options for the Route
	if cr.Spec.Server.Route.TLS != nil {
		route.Spec.TLS = cr.Spec.Server.Route.TLS
	}

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = nameWithSuffix("server", cr)

	// Allow override of the WildcardPolicy for the Route
	if cr.Spec.Server.Route.WildcardPolicy != nil && len(*cr.Spec.Server.Route.WildcardPolicy) > 0 {
		route.Spec.WildcardPolicy = *cr.Spec.Server.Route.WildcardPolicy
	}

	if err := controllerutil.SetControllerReference(cr, route, r.Scheme); err != nil {
		return err
	}
	if !found {
		return r.Client.Create(context.TODO(), route)
	}
	return r.Client.Update(context.TODO(), route)
}

// reconcileApplicationSetControllerWebhookRoute will ensure that the ArgoCD Server Route is present.
func (r *ReconcileArgoCD) reconcileApplicationSetControllerWebhookRoute(cr *argoproj.ArgoCD) error {
	name := fmt.Sprintf("%s-%s", common.ApplicationSetServiceNameSuffix, "webhook")
	route := newRouteWithSuffix(name, cr)
	found := argoutil.IsObjectFound(r.Client, cr.Namespace, route.Name, route)
	if found {
		if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.WebhookServer.Route.Enabled {
			// Route exists but enabled flag has been set to false, delete the Route
			return r.Client.Delete(context.TODO(), route)
		}
	}

	if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.WebhookServer.Route.Enabled {
		return nil // Route not enabled, move along...
	}

	// Allow override of the Annotations for the Route.
	if len(cr.Spec.ApplicationSet.WebhookServer.Route.Annotations) > 0 {
		route.Annotations = cr.Spec.Server.Route.Annotations
	}

	// Allow override of the Labels for the Route.
	if len(cr.Spec.ApplicationSet.WebhookServer.Route.Labels) > 0 {
		labels := route.Labels
		for key, val := range cr.Spec.Server.Route.Labels {
			labels[key] = val
		}
		route.Labels = labels
	}

	// Allow override of the Host for the Route.
	if len(cr.Spec.Server.Host) > 0 {
		route.Spec.Host = cr.Spec.ApplicationSet.WebhookServer.Host
	}

	if cr.Spec.Server.Insecure {
		// Disable TLS and rely on the cluster certificate.
		route.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("webhook"),
		}
		route.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationEdge,
		}
	} else {
		// Server is using TLS configure passthrough.
		route.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("webhook"),
		}
		route.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationPassthrough,
		}
	}

	// Allow override of TLS options for the Route
	if cr.Spec.Server.Route.TLS != nil {
		route.Spec.TLS = cr.Spec.Server.Route.TLS
	}

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = nameWithSuffix(common.ApplicationSetServiceNameSuffix, cr)

	// Allow override of the WildcardPolicy for the Route
	if cr.Spec.Server.Route.WildcardPolicy != nil && len(*cr.Spec.Server.Route.WildcardPolicy) > 0 {
		route.Spec.WildcardPolicy = *cr.Spec.Server.Route.WildcardPolicy
	}

	if err := controllerutil.SetControllerReference(cr, route, r.Scheme); err != nil {
		return err
	}
	if !found {
		return r.Client.Create(context.TODO(), route)
	}
	return r.Client.Update(context.TODO(), route)
}
