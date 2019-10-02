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
	err := r.reconcileServerRoute(cr)
	if err != nil {
		log.Error(err, "unable to reconcile server route")
		return err
	}
	return nil
}

func (r *ReconcileArgoCD) reconcileServerRoute(cr *argoproj.ArgoCD) error {
	route := newRoute("argocd-server-route", cr.Namespace)
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: route.Name}, route)
	if found {
		// Route found, do nothing
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, route, r.scheme); err != nil {
		return err
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

	return r.client.Create(context.TODO(), route)
}
