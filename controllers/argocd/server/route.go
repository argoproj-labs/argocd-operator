package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/openshift"
	"github.com/argoproj-labs/argocd-operator/pkg/util"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileRoute will ensure that ArgoCD .Spec.Server.Route resource is present.
func (sr *ServerReconciler) reconcileRoute() error {

	req := openshift.RouteRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, sr.Instance.Namespace, sr.Instance.Name, sr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Client:     sr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	// Allow override of the Annotations for the Route.
	if len(sr.Instance.Spec.Server.Route.Annotations) > 0 {
		req.ObjectMeta.Annotations = sr.Instance.Spec.Server.Route.Annotations
	}

	// Allow override of the Labels for the Route.
	if len(sr.Instance.Spec.Server.Route.Labels) > 0 {
		labels := req.ObjectMeta.Labels
		for key, val := range sr.Instance.Spec.Server.Route.Labels {
			labels[key] = val
		}
		req.ObjectMeta.Labels = labels
	}

	// Allow override of the Host for the Route.
	if len(sr.Instance.Spec.Server.Host) > 0 {
		req.Spec.Host = sr.getHost() // TODO: What additional role needed for this?
	}

	hostname, err := argocdcommon.ShortenHostname(req.Spec.Host)
	if err != nil {
		return err
	}
	req.Spec.Host = hostname

	if sr.Instance.Spec.Server.Insecure {
		// Disable TLS and rely on the cluster certificate.
		req.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("http"),
		}
		req.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationEdge,
		}
	} else {
		// Server is using TLS configure passthrough.
		req.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("https"),
		}
		req.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationPassthrough,
		}
	}

	// Allow override of TLS options for the Route
	if sr.Instance.Spec.Server.Route.TLS != nil {
		req.Spec.TLS = sr.Instance.Spec.Server.Route.TLS
	}

	req.Spec.To.Kind = common.ServiceKind
	req.Spec.To.Name = resourceName

	// Allow override of the WildcardPolicy for the Route
	if sr.Instance.Spec.Server.Route.WildcardPolicy != nil && len(*sr.Instance.Spec.Server.Route.WildcardPolicy) > 0 {
		req.Spec.WildcardPolicy = *sr.Instance.Spec.Server.Route.WildcardPolicy
	}

	ignoreDrift := false
	updateFn := func(existing, desired *routev1.Route, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			{Existing: &existing.Spec, Desired: &desired.Spec, ExtraAction: nil},
		}
		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}
	return sr.reconRoute(req, argocdcommon.UpdateFnRoute(updateFn), ignoreDrift)
}

func (sr *ServerReconciler) reconRoute(req openshift.RouteRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := openshift.RequestRoute(req)
	if err != nil {
		sr.Logger.Debug("reconRoute: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconRoute: failed to request Route %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(sr.Instance, desired, sr.Scheme); err != nil {
		sr.Logger.Error(err, "reconRoute: failed to set owner reference for Route", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := openshift.GetRoute(desired.Name, desired.Namespace, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconRoute: failed to retrieve Route %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = openshift.CreateRoute(desired, sr.Client); err != nil {
			return errors.Wrapf(err, "reconRoute: failed to create Route %s in namespace %s", desired.Name, desired.Namespace)
		}
		sr.Logger.Info("Route created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// Route found, no update required - nothing to do
	if ignoreDrift {
		return nil
	}

	changed := false

	// execute supplied update function
	if updateFn != nil {
		if fn, ok := updateFn.(argocdcommon.UpdateFnRoute); ok {
			if err := fn(existing, desired, &changed); err != nil {
				return errors.Wrapf(err, "reconRoute: failed to execute update function for %s in namespace %s", existing.Name, existing.Namespace)
			}
		}
	}

	if !changed {
		return nil
	}

	if err = openshift.UpdateRoute(existing, sr.Client); err != nil {
		return errors.Wrapf(err, "reconRoute: failed to update Route %s", existing.Name)
	}

	sr.Logger.Info("Route updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteRoute will delete route with given name.
func (sr *ServerReconciler) deleteRoute(name, namespace string) error {
	if err := openshift.DeleteRoute(name, namespace, sr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteRoute: failed to delete route %s in namespace %s", name, namespace)
	}
	sr.Logger.Info("route deleted", "name", name, "namespace", namespace)
	return nil
}
