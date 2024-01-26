package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/openshift"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileRoute will ensure that ArgoCD .Spec.Server.Route resource is present.
func (sr *ServerReconciler) reconcileRoute() error {

	// route disabled, cleanup any existing route and exit
	if !sr.Instance.Spec.Server.Route.Enabled {
		return sr.deleteRoute(resourceName, sr.Instance.Namespace)
	}

	routeReq := openshift.RouteRequest{
		ObjectMeta:  argoutil.GetObjMeta(resourceName, sr.Instance.Namespace, sr.Instance.Name, sr.Instance.Namespace, component),
		Client:    sr.Client,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	// Allow override of the Annotations for the Route.
	if len(sr.Instance.Spec.Server.Route.Annotations) > 0 {
		routeReq.ObjectMeta.Annotations = sr.Instance.Spec.Server.Route.Annotations
	}

	// Allow override of the Labels for the Route.
	if len(sr.Instance.Spec.Server.Route.Labels) > 0 {
		labels := routeReq.ObjectMeta.Labels
		for key, val := range sr.Instance.Spec.Server.Route.Labels {
			labels[key] = val
		}
		routeReq.ObjectMeta.Labels = labels
	}

	// Allow override of the Host for the Route.
	if len(sr.Instance.Spec.Server.Host) > 0 {
		routeReq.Spec.Host = sr.Instance.Spec.Server.Host // TODO: What additional role needed for this?
	}

	if sr.Instance.Spec.Server.Insecure {
		// Disable TLS and rely on the cluster certificate.
		routeReq.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("http"),
		}
		routeReq.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationEdge,
		}
	} else {
		// Server is using TLS configure passthrough.
		routeReq.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("https"),
		}
		routeReq.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationPassthrough,
		}
	}

	// Allow override of TLS options for the Route
	if sr.Instance.Spec.Server.Route.TLS != nil {
		routeReq.Spec.TLS = sr.Instance.Spec.Server.Route.TLS
	}

	routeReq.Spec.To.Kind = common.ServiceKind
	routeReq.Spec.To.Name = resourceName

	// Allow override of the WildcardPolicy for the Route
	if sr.Instance.Spec.Server.Route.WildcardPolicy != nil && len(*sr.Instance.Spec.Server.Route.WildcardPolicy) > 0 {
		routeReq.Spec.WildcardPolicy = *sr.Instance.Spec.Server.Route.WildcardPolicy
	}

	desiredRoute, err := openshift.RequestRoute(routeReq)
	if err != nil {
		return errors.Wrapf(err, "reconcileRoute: failed to request route %s in namespace %s", desiredRoute.Name, desiredRoute.Namespace)
	}

	if err := controllerutil.SetControllerReference(sr.Instance, desiredRoute, sr.Scheme); err != nil {
		sr.Logger.Error(err, "reconcileRoute: failed to set owner reference for route", "name", desiredRoute.Name, "namespace", desiredRoute.Namespace)
	}

	// route doesn't exist in the namespace, create it
	existingRoute, err := openshift.GetRoute(desiredRoute.Name, desiredRoute.Namespace, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileRoute: failed to retrieve route %s in namespace %s", desiredRoute.Name, desiredRoute.Namespace)
		}

		if err = openshift.CreateRoute(desiredRoute, sr.Client); err != nil {
			return errors.Wrapf(err, "reconcileRoute: failed to create route %s in namespace %s", desiredRoute.Name, desiredRoute.Namespace)
		}

		sr.Logger.V(0).Info("route created", "name", desiredRoute.Name, "namespace", desiredRoute.Namespace)
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

	// nothing changed, exit reconciliation
	if !changed {
		return nil
	}

	if err = openshift.UpdateRoute(existingRoute, sr.Client); err != nil {
		return errors.Wrapf(err, "reconcileRoute: failed to update route %s in namespace %s", existingRoute.Name, existingRoute.Namespace)
	}

	sr.Logger.V(0).Info("route updated", "name", existingRoute.Name, "namespace", existingRoute.Namespace)
	return nil
}

// deleteRoute will delete route with given name.
func (sr *ServerReconciler) deleteRoute(name, namespace string) error {
	if err := openshift.DeleteRoute(name, namespace, sr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteRoute: failed to delete rpute %s in namespace %s", name, namespace)
	}
	sr.Logger.V(0).Info("route deleted", "name", name, "namespace", namespace)
	return nil
}
