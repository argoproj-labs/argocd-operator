package applicationset

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

func (asr *ApplicationSetReconciler) reconcileWebhookRoute() error {
	req := openshift.RouteRequest{
		ObjectMeta: argoutil.GetObjMeta(webhookResourceName, asr.Instance.Namespace, asr.Instance.Name, asr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Spec:       asr.getRouteReqSpec(),
		Client:     asr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	// Allow override of the Annotations for the Route.
	if len(asr.Instance.Spec.ApplicationSet.WebhookServer.Route.Annotations) > 0 {
		req.ObjectMeta.Annotations = asr.Instance.Spec.Server.Route.Annotations
	}

	// Allow override of the Labels for the Route.
	if len(asr.Instance.Spec.ApplicationSet.WebhookServer.Route.Labels) > 0 {
		labels := req.ObjectMeta.Labels
		for key, val := range asr.Instance.Spec.Server.Route.Labels {
			labels[key] = val
		}
		req.ObjectMeta.Labels = labels
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
	return asr.reconRoute(req, argocdcommon.UpdateFnRoute(updateFn), ignoreDrift)

}

func (asr *ApplicationSetReconciler) reconRoute(req openshift.RouteRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := openshift.RequestRoute(req)
	if err != nil {
		asr.Logger.Debug("reconRoute: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconRoute: failed to request Route %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(asr.Instance, desired, asr.Scheme); err != nil {
		asr.Logger.Error(err, "reconRoute: failed to set owner reference for Route", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := openshift.GetRoute(desired.Name, desired.Namespace, asr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconRoute: failed to retrieve Route %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = openshift.CreateRoute(desired, asr.Client); err != nil {
			return errors.Wrapf(err, "reconRoute: failed to create Route %s in namespace %s", desired.Name, desired.Namespace)
		}
		asr.Logger.Info("Route created", "name", desired.Name, "namespace", desired.Namespace)
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

	if err = openshift.UpdateRoute(existing, asr.Client); err != nil {
		return errors.Wrapf(err, "reconRoute: failed to update Route %s", existing.Name)
	}

	asr.Logger.Info("Route updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteRoute will delete route with given name.
func (asr *ApplicationSetReconciler) deleteRoute(name, namespace string) error {
	if err := openshift.DeleteRoute(name, namespace, asr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteRoute: failed to delete route %s in namespace %s", name, namespace)
	}
	asr.Logger.Info("route deleted", "name", name, "namespace", namespace)
	return nil
}

func (asr *ApplicationSetReconciler) getRouteReqSpec() routev1.RouteSpec {
	spec := routev1.RouteSpec{}

	// Allow override of the Host for the Route.
	if len(asr.Instance.Spec.Server.Host) > 0 {
		spec.Host = asr.Instance.Spec.ApplicationSet.WebhookServer.Host
	}

	hostname, err := argocdcommon.ShortenHostname(spec.Host)
	if err != nil {
		asr.Logger.Error(err, "getRouteReqSpec")
	}

	spec.Host = hostname

	if asr.Instance.Spec.Server.Insecure {
		// Disable TLS and rely on the cluster certificate.
		spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("webhook"),
		}
		spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationEdge,
		}
	} else {
		// Server is using TLS configure passthrough.
		spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("webhook"),
		}
		spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationPassthrough,
		}
	}

	// Allow override of TLS options for the Route
	if asr.Instance.Spec.Server.Route.TLS != nil {
		spec.TLS = asr.Instance.Spec.Server.Route.TLS
	}

	spec.To.Kind = common.ServiceKind
	spec.To.Name = resourceName

	// Allow override of the WildcardPolicy for the Route
	if asr.Instance.Spec.Server.Route.WildcardPolicy != nil && len(*asr.Instance.Spec.Server.Route.WildcardPolicy) > 0 {
		spec.WildcardPolicy = *asr.Instance.Spec.Server.Route.WildcardPolicy
	}

	return spec
}
