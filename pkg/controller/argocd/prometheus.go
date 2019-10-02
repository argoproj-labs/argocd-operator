package argocd

import (
	"context"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	argoproj "github.com/jmckind/argocd-operator/pkg/apis/argoproj/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// newPrometheus retuns a new Prometheus instance.
func newPrometheus(name string, namespace string) *monitoringv1.Prometheus {
	return &monitoringv1.Prometheus{
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

func (r *ReconcileArgoCD) reconcilePrometheus(cr *argoproj.ArgoCD) error {
	prometheus := newPrometheus("argocd-prometheus", cr.Namespace)
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: prometheus.Name}, prometheus)
	if found {
		// Prometheus found, do nothing
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, prometheus, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), prometheus)
}
