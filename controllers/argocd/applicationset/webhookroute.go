package applicationset

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/networking"

	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (asr *ApplicationSetReconciler) reconcileWebhookRoute() error {
	asr.Logger.Info("reconciling webhookroutes")

	desiredWebhookRoute := asr.getDesiredWebhookRoute()
	webhookRouteRequest := asr.getWebhookRouteRequest(*desiredWebhookRoute)

	desiredWebhookRoute, err := networking.RequestRoute(webhookRouteRequest)

	if err != nil {
		asr.Logger.Error(err, "reconcileRoute: failed to request route", "name", desiredWebhookRoute.Name, "namespace", desiredWebhookRoute.Namespace)
		return err
	}

	namespace, err := cluster.GetNamespace(asr.Instance.Namespace, asr.Client)
	if err != nil {
		asr.Logger.Error(err, "reconcileRoute: failed to retrieve namespace", "name", asr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := asr.deleteWebhookRoute(desiredWebhookRoute.Name, desiredWebhookRoute.Namespace); err != nil {
			asr.Logger.Error(err, "reconcileRoute: failed to delete route", "name", desiredWebhookRoute.Name, "namespace", desiredWebhookRoute.Namespace)
		}
		return err
	}

	existingRoute, err := networking.GetRoute(desiredWebhookRoute.Name, desiredWebhookRoute.Namespace, asr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			asr.Logger.Error(err, "reconcileRoute: failed to retrieve route", "name", desiredWebhookRoute.Name, "namespace", desiredWebhookRoute.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(asr.Instance, desiredWebhookRoute, asr.Scheme); err != nil {
			asr.Logger.Error(err, "reconcileRoute: failed to set owner reference for route", "name", desiredWebhookRoute.Name, "namespace", desiredWebhookRoute.Namespace)
		}

		if err = networking.CreateRoute(desiredWebhookRoute, asr.Client); err != nil {
			asr.Logger.Error(err, "reconcileRoute: failed to create route", "name", desiredWebhookRoute.Name, "namespace", desiredWebhookRoute.Namespace)
			return err
		}
		asr.Logger.V(0).Info("reconcileRoute: route created", "name", desiredWebhookRoute.Name, "namespace", desiredWebhookRoute.Namespace)
		return nil
	}

	webhookRouteChanged := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{&existingRoute.Annotations, &desiredWebhookRoute.Annotations, nil},
		{&existingRoute.Labels, &desiredWebhookRoute.Labels, nil},
		{&existingRoute.Spec.WildcardPolicy, &desiredWebhookRoute.Spec.WildcardPolicy, nil},
		{&existingRoute.Spec.Host, &desiredWebhookRoute.Spec.Host, nil},
		{&existingRoute.Spec.Port, &desiredWebhookRoute.Spec.Port, nil},
		{&existingRoute.Spec.TLS, &desiredWebhookRoute.Spec.TLS, nil},
		{&existingRoute.Spec.To, &desiredWebhookRoute.Spec.To, nil},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &webhookRouteChanged)
	}

	if webhookRouteChanged {
		if err = networking.UpdateRoute(existingRoute, asr.Client); err != nil {
			asr.Logger.Error(err, "reconcileWebhookRoute: failed to update webhook route", "name", existingRoute.Name, "namespace", existingRoute.Namespace)
			return err
		}
	}

	asr.Logger.V(0).Info("reconcileRoute: webhook route updated", "name", desiredWebhookRoute.Name, "namespace", desiredWebhookRoute.Namespace)

	return nil
}

func (asr *ApplicationSetReconciler) deleteWebhookRoute(name, namespace string) error {
	if err := networking.DeleteRoute(name, namespace, asr.Client); err != nil {
		asr.Logger.Error(err, "DeleteRoute: failed to delete route", "name", name, "namespace", namespace)
		return err
	}
	asr.Logger.V(0).Info("DeleteRoute: route deleted", "name", name, "namespace", namespace)
	return nil
}

func (asr *ApplicationSetReconciler) getWebhookRouteSpec() routev1.RouteSpec {
	routeSpec := routev1.RouteSpec{
		Port: &routev1.RoutePort{
			TargetPort: intstr.FromString(common.Webhook),
		},
		TLS: &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationPassthrough,
		},
		To: routev1.RouteTargetReference{
			Kind: common.ServiceKind,
			Name: argoutil.NameWithSuffix(asr.Instance.Name, common.AppSetControllerComponent),
		},
	}

	if asr.Instance.Spec.Server.Insecure {
		routeSpec.TLS.Termination = routev1.TLSTerminationEdge
	}

	if len(asr.Instance.Spec.Server.Host) > 0 {
		routeSpec.Host = asr.Instance.Spec.Server.Host
	}

	// Allow override of the WildcardPolicy for the Route
	if asr.Instance.Spec.Server.Route.WildcardPolicy != nil && len(*asr.Instance.Spec.Server.Route.WildcardPolicy) > 0 {
		routeSpec.WildcardPolicy = *asr.Instance.Spec.Server.Route.WildcardPolicy
	}

	return routeSpec
}

func (asr *ApplicationSetReconciler) getWebhookRouteRequest(route routev1.Route) networking.RouteRequest {
	webhookRouteReq := networking.RouteRequest{
		ObjectMeta: route.ObjectMeta,
		Spec:       route.Spec,
		Client:     asr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}
	return webhookRouteReq
}

func (asr *ApplicationSetReconciler) getDesiredWebhookRoute() *routev1.Route {
	desiredWebhook := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:        common.AppSetWebhookRouteName,
			Namespace:   asr.Instance.Namespace,
			Labels:      resourceLabels,
			Annotations: asr.Instance.Annotations,
		},
		Spec: asr.getWebhookRouteSpec(),
	}

	// Allow override of the Annotations for the Route.
	if len(asr.Instance.Spec.ApplicationSet.WebhookServer.Route.Annotations) > 0 {
		desiredWebhook.ObjectMeta.Annotations = asr.Instance.Spec.Server.Route.Annotations
	}

	// Allow override of the Labels for the Route.
	if len(asr.Instance.Spec.ApplicationSet.WebhookServer.Route.Labels) > 0 {
		labels := desiredWebhook.ObjectMeta.Labels
		for key, val := range asr.Instance.Spec.Server.Route.Labels {
			labels[key] = val
		}
		desiredWebhook.ObjectMeta.Labels = labels
	}

	return desiredWebhook
}
