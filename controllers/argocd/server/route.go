package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/networking"
	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileRoute will ensure that ArgoCD .Spec.Server.Route resource is present.
func (sr * ServerReconciler) reconcileRoute() error {

	sr.Logger.Info("reconciling route")

	routeName := getRouteName(sr.Instance.Name)
	routeLabels := common.DefaultLabels(routeName, sr.Instance.Name, ServerControllerComponent)

	// route disabled, cleanup and exit 
	if !sr.Instance.Spec.Server.Route.Enabled {
		return sr.deleteRoute(routeName, sr.Instance.Namespace)
	}

	routeRequest := networking.RouteRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        routeName,
			Labels:      routeLabels,
			Annotations: sr.Instance.Annotations,
			Namespace: sr.Instance.Namespace,
		},
		Client:    sr.Client,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	// Allow override of the Annotations for the Route.
	if len(sr.Instance.Spec.Server.Route.Annotations) > 0 {
		routeRequest.ObjectMeta.Annotations = sr.Instance.Spec.Server.Route.Annotations
	}

	// Allow override of the Labels for the Route.
	if len(sr.Instance.Spec.Server.Route.Labels) > 0 {
		labels := routeRequest.ObjectMeta.Labels
		for key, val := range sr.Instance.Spec.Server.Route.Labels {
			labels[key] = val
		}
		routeRequest.ObjectMeta.Labels = labels
	}

	// Allow override of the Host for the Route.
	if len(sr.Instance.Spec.Server.Host) > 0 {
		routeRequest.Spec.Host = sr.Instance.Spec.Server.Host // TODO: What additional role needed for this?
	}

	if sr.Instance.Spec.Server.Insecure {
		// Disable TLS and rely on the cluster certificate.
		routeRequest.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("http"),
		}
		routeRequest.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationEdge,
		}
	} else {
		// Server is using TLS configure passthrough.
		routeRequest.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("https"),
		}
		routeRequest.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationPassthrough,
		}
	}

	// Allow override of TLS options for the Route
	if sr.Instance.Spec.Server.Route.TLS != nil {
		routeRequest.Spec.TLS = sr.Instance.Spec.Server.Route.TLS
	}

	routeRequest.Spec.To.Kind = common.ServiceKind
	routeRequest.Spec.To.Name = getServiceName(sr.Instance.Name)

	// Allow override of the WildcardPolicy for the Route
	if sr.Instance.Spec.Server.Route.WildcardPolicy != nil && len(*sr.Instance.Spec.Server.Route.WildcardPolicy) > 0 {
		routeRequest.Spec.WildcardPolicy = *sr.Instance.Spec.Server.Route.WildcardPolicy
	}

	desiredRoute, err := networking.RequestRoute(routeRequest)
	if err != nil {
		sr.Logger.Error(err, "reconcileRoute: failed to request route", "name", desiredRoute.Name, "namespace", desiredRoute.Namespace)
		sr.Logger.V(1).Info("reconcileRoute: one or more mutations could not be applied")
		return err
	}

	// route doesn't exist in the namespace, create it
	existingRoute, err := networking.GetRoute(desiredRoute.Name, desiredRoute.Namespace, sr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			sr.Logger.Error(err, "reconcileRoute: failed to retrieve route", "name", desiredRoute.Name, "namespace", desiredRoute.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(sr.Instance, desiredRoute, sr.Scheme); err != nil {
			sr.Logger.Error(err, "reconcileRoute: failed to set owner reference for route", "name", desiredRoute.Name, "namespace", desiredRoute.Namespace)
		}

		if err = networking.CreateRoute(desiredRoute, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileRoute: failed to create route", "name", desiredRoute.Name, "namespace", desiredRoute.Namespace)
			return err
		}
		
		sr.Logger.V(0).Info("reconcileRoute: route created", "name", desiredRoute.Name, "namespace", desiredRoute.Namespace)
		return nil
	}

	// difference in existing & desired ingress, update it
	changed := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{&existingRoute.ObjectMeta.Labels, &desiredRoute.ObjectMeta.Labels, nil},
		{&existingRoute.ObjectMeta.Annotations, &desiredRoute.ObjectMeta.Annotations, nil},
		{&existingRoute.Spec, &desiredRoute.Spec, nil},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &changed)
	}

	if changed {
		if err = networking.UpdateRoute(existingRoute, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileRoute: failed to update route", "name", existingRoute.Name, "namespace", existingRoute.Namespace)
			return err
		}
		sr.Logger.V(0).Info("reconcileRoute: route updated", "name", existingRoute.Name, "namespace", existingRoute.Namespace)
	}

	// route found, no changes detected
	return nil
}

// deleteRoute will delete route with given name.
func (sr *ServerReconciler) deleteRoute(name, namespace string) error {
	if err := networking.DeleteRoute(name, namespace, sr.Client); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		sr.Logger.Error(err, "reconcileRoute: failed to delete route", "name", name, "namespace", namespace)
		return err
	}
	sr.Logger.V(0).Info("reconcileRoute: route deleted", "name", name, "namespace", namespace)
	return nil
}
