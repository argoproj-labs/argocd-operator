package argocd

import (
	"context"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
)

const (
	grafanaDeprecatedWarning = "Warning: grafana field is deprecated from ArgoCD: field will be ignored."
)

// reconcileGrafanaService will ensure that the Service for Grafana is present.
func (r *ReconcileArgoCD) reconcileGrafanaService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("grafana", "grafana", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		if !cr.Spec.Grafana.Enabled {
			// Service exists but enabled flag has been set to false, delete the Service
			return r.Client.Delete(context.TODO(), svc)
		}
		log.Info(grafanaDeprecatedWarning)
		return nil // Service found, do nothing
	}

	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	log.Info(grafanaDeprecatedWarning)
	return nil
}

// reconcileGrafanaDeployment will ensure the Deployment resource is present for the ArgoCD Grafana component.
func (r *ReconcileArgoCD) reconcileGrafanaDeployment(cr *argoproj.ArgoCD) error {
	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}
	log.Info(grafanaDeprecatedWarning)
	return nil
}

// reconcileGrafanaIngress will ensure that the ArgoCD Server GRPC Ingress is present.
func (r *ReconcileArgoCD) reconcileGrafanaIngress(cr *argoproj.ArgoCD) error {
	ingress := newIngressWithSuffix("grafana", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.Client.Delete(context.TODO(), ingress)
		}
		log.Info(grafanaDeprecatedWarning)
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Ingress.Enabled {
		return nil // Grafana itself or Ingress not enabled, move along...
	}

	log.Info(grafanaDeprecatedWarning)

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
		log.Info(grafanaDeprecatedWarning)
		return nil // Route found, do nothing
	}

	if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Route.Enabled {
		return nil // Grafana itself or Route not enabled, do nothing.
	}

	log.Info(grafanaDeprecatedWarning)

	return nil
}
