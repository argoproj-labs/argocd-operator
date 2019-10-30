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

	argoproj "github.com/jmckind/argocd-operator/pkg/apis/argoproj/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// newRoute retuns a new Route instance.
func newRoute(name string, namespace string) *routev1.Route {
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":    name,
				"app.kubernetes.io/part-of": "argocd",
			},
		},
	}
}

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

func (r *ReconcileArgoCD) reconcileGrafanaRoute(cr *argoproj.ArgoCD) error {
	route := newRoute("argocd-grafana", cr.Namespace)
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: route.Name}, route)
	if found {
		return nil // Route found, do nothing
	}

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = "argocd-grafana"
	route.Spec.Port = &routev1.RoutePort{
		TargetPort: intstr.FromString("http"),
	}

	if err := controllerutil.SetControllerReference(cr, route, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), route)
}

func (r *ReconcileArgoCD) reconcileServerRoute(cr *argoproj.ArgoCD) error {
	route := newRoute("argocd-server-route", cr.Namespace)
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: route.Name}, route)
	if found {
		return nil // Route found, do nothing
	}

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = "argocd-server"
	route.Spec.Port = &routev1.RoutePort{
		TargetPort: intstr.FromString("https"),
	}
	route.Spec.TLS = &routev1.TLSConfig{
		InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyNone,
		Termination:                   routev1.TLSTerminationPassthrough,
	}

	if err := controllerutil.SetControllerReference(cr, route, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), route)
}

func (r *ReconcileArgoCD) reconcilePrometheusRoute(cr *argoproj.ArgoCD) error {
	route := newRoute("argocd-prometheus", cr.Namespace)
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: route.Name}, route)
	if found {
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
