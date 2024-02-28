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

	// route disabled, cleanup any existing route and exit
	if !sr.Instance.Spec.Server.Route.Enabled {
		return sr.deleteRoute(resourceName, sr.Instance.Namespace)
	}

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
		req.Spec.Host = sr.Instance.Spec.Server.Host // TODO: What additional role needed for this?
	}

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

	desired, err := openshift.RequestRoute(req)
	if err != nil {
		return errors.Wrapf(err, "reconcileRoute: failed to request route %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err := controllerutil.SetControllerReference(sr.Instance, desired, sr.Scheme); err != nil {
		sr.Logger.Error(err, "reconcileRoute: failed to set owner reference for route", "name", desired.Name, "namespace", desired.Namespace)
	}

	// route doesn't exist in the namespace, create it
	existing, err := openshift.GetRoute(desired.Name, desired.Namespace, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileRoute: failed to retrieve route %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = openshift.CreateRoute(desired, sr.Client); err != nil {
			return errors.Wrapf(err, "reconcileRoute: failed to create route %s in namespace %s", desired.Name, desired.Namespace)
		}

		sr.Logger.V(0).Info("route created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// difference in existing & desired ingress, update it
	changed := false
	fieldsToCompare := []argocdcommon.FieldToCompare{
		{Existing: &existing.ObjectMeta.Labels, Desired: &desired.ObjectMeta.Labels, ExtraAction: nil},
		{Existing: &existing.ObjectMeta.Annotations, Desired: &desired.ObjectMeta.Annotations, ExtraAction: nil},
		{Existing: &existing.Spec, Desired: &desired.Spec, ExtraAction: nil},
	}
	argocdcommon.UpdateIfChanged(fieldsToCompare, &changed)

	// nothing changed, exit reconciliation
	if !changed {
		return nil
	}

	if err = openshift.UpdateRoute(existing, sr.Client); err != nil {
		return errors.Wrapf(err, "reconcileRoute: failed to update route %s in namespace %s", existing.Name, existing.Namespace)
	}

	sr.Logger.Info("route updated", "name", existing.Name, "namespace", existing.Namespace)
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
