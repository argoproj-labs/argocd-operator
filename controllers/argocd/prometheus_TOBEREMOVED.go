package argocd

import (
	"context"
	"fmt"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	routev1 "github.com/openshift/api/route/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcilePrometheusIngress will ensure that the Prometheus Ingress is present.
func (r *ReconcileArgoCD) reconcilePrometheusIngress(cr *argoproj.ArgoCD) error {
	ingress := newIngressWithSuffix("prometheus", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.Client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Ingress.Enabled {
		return nil // Prometheus itself or Ingress not enabled, move along...
	}

	// Add default annotations
	atns := make(map[string]string)
	atns[common.ArgoCDKeyIngressSSLRedirect] = "true"
	atns[common.ArgoCDKeyIngressBackendProtocol] = "HTTP"

	// Override default annotations if specified
	if len(cr.Spec.Prometheus.Ingress.Annotations) > 0 {
		atns = cr.Spec.Prometheus.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	ingress.Spec.IngressClassName = cr.Spec.Prometheus.Ingress.IngressClassName

	pathType := networkingv1.PathTypeImplementationSpecific
	// Add rules
	ingress.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: getPrometheusHost(cr),
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path: getPathOrDefault(cr.Spec.Prometheus.Ingress.Path),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "prometheus-operated",
									Port: networkingv1.ServiceBackendPort{
										Name: "web",
									},
								},
							},
							PathType: &pathType,
						},
					},
				},
			},
		},
	}

	// Add TLS options
	ingress.Spec.TLS = []networkingv1.IngressTLS{
		{
			Hosts:      []string{cr.Name},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// Allow override of TLS options if specified
	if len(cr.Spec.Prometheus.Ingress.TLS) > 0 {
		ingress.Spec.TLS = cr.Spec.Prometheus.Ingress.TLS
	}

	if err := controllerutil.SetControllerReference(cr, ingress, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ingress)
}

// reconcilePrometheusRoute will ensure that the ArgoCD Prometheus Route is present.
func (r *ReconcileArgoCD) reconcilePrometheusRoute(cr *argoproj.ArgoCD) error {
	route := newRouteWithSuffix("prometheus", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, route.Name, route) {
		if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Route.Enabled {
			// Route exists but enabled flag has been set to false, delete the Route
			return r.Client.Delete(context.TODO(), route)
		}
		return nil // Route found, do nothing
	}

	if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Route.Enabled {
		return nil // Prometheus itself or Route not enabled, do nothing.
	}

	// Allow override of the Annotations for the Route.
	if len(cr.Spec.Prometheus.Route.Annotations) > 0 {
		route.Annotations = cr.Spec.Prometheus.Route.Annotations
	}

	// Allow override of the Labels for the Route.
	if len(cr.Spec.Prometheus.Route.Labels) > 0 {
		labels := route.Labels
		for key, val := range cr.Spec.Prometheus.Route.Labels {
			labels[key] = val
		}
		route.Labels = labels
	}

	// Allow override of the Host for the Route.
	if len(cr.Spec.Prometheus.Host) > 0 {
		route.Spec.Host = cr.Spec.Prometheus.Host // TODO: What additional role needed for this?
	}

	route.Spec.Port = &routev1.RoutePort{
		TargetPort: intstr.FromString("web"),
	}

	// Allow override of TLS options for the Route
	if cr.Spec.Prometheus.Route.TLS != nil {
		route.Spec.TLS = cr.Spec.Prometheus.Route.TLS
	}

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = "prometheus-operated"

	// Allow override of the WildcardPolicy for the Route
	if cr.Spec.Prometheus.Route.WildcardPolicy != nil && len(*cr.Spec.Prometheus.Route.WildcardPolicy) > 0 {
		route.Spec.WildcardPolicy = *cr.Spec.Prometheus.Route.WildcardPolicy
	}

	if err := controllerutil.SetControllerReference(cr, route, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), route)
}

// reconcilePrometheus will ensure that Prometheus is present for ArgoCD metrics.
func (r *ReconcileArgoCD) reconcilePrometheus(cr *argoproj.ArgoCD) error {
	prometheus := newPrometheus(cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, prometheus.Name, prometheus) {
		if !cr.Spec.Prometheus.Enabled {
			// Prometheus exists but enabled flag has been set to false, delete the Prometheus
			return r.Client.Delete(context.TODO(), prometheus)
		}
		if hasPrometheusSpecChanged(prometheus, cr) {
			prometheus.Spec.Replicas = cr.Spec.Prometheus.Size
			return r.Client.Update(context.TODO(), prometheus)
		}
		return nil // Prometheus found, do nothing
	}

	if !cr.Spec.Prometheus.Enabled {
		return nil // Prometheus not enabled, do nothing.
	}

	prometheus.Spec.Replicas = getPrometheusReplicas(cr)
	prometheus.Spec.ServiceAccountName = "prometheus-k8s"
	prometheus.Spec.ServiceMonitorSelector = &metav1.LabelSelector{}

	if err := controllerutil.SetControllerReference(cr, prometheus, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), prometheus)
}

// hasPrometheusSpecChanged will return true if the supported properties differs in the actual versus the desired state.
func hasPrometheusSpecChanged(actual *monitoringv1.Prometheus, desired *argoproj.ArgoCD) bool {
	// Replica count
	if desired.Spec.Prometheus.Size != nil && *desired.Spec.Prometheus.Size >= 0 { // Valid replica count specified in desired state
		if actual.Spec.Replicas != nil { // Actual replicas value is set
			if *actual.Spec.Replicas != *desired.Spec.Prometheus.Size {
				return true
			}
		} else if *desired.Spec.Prometheus.Size != common.ArgoCDDefaultPrometheusReplicas { // Actual replicas value is NOT set, but desired replicas differs from the default
			return true
		}
	} else { // Replica count NOT specified in desired state
		if actual.Spec.Replicas != nil && *actual.Spec.Replicas != common.ArgoCDDefaultPrometheusReplicas {
			return true
		}
	}
	return false
}

// getPrometheusHost will return the hostname value for Prometheus.
func getPrometheusHost(cr *argoproj.ArgoCD) string {
	host := nameWithSuffix("prometheus", cr)
	if len(cr.Spec.Prometheus.Host) > 0 {
		host = cr.Spec.Prometheus.Host
	}
	return host
}

// getPrometheusSize will return the size value for the Prometheus replica count.
func getPrometheusReplicas(cr *argoproj.ArgoCD) *int32 {
	replicas := common.ArgoCDDefaultPrometheusReplicas
	if cr.Spec.Prometheus.Size != nil {
		if *cr.Spec.Prometheus.Size >= 0 && *cr.Spec.Prometheus.Size != replicas {
			replicas = *cr.Spec.Prometheus.Size
		}
	}
	return &replicas
}

// newPrometheus returns a new Prometheus instance for the given ArgoCD.
func newPrometheus(cr *argoproj.ArgoCD) *monitoringv1.Prometheus {
	return &monitoringv1.Prometheus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
	}
}

// newServiceMonitor returns a new ServiceMonitor instance.
func newServiceMonitor(cr *argoproj.ArgoCD) *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
	}
}

// newServiceMonitorWithName returns a new ServiceMonitor instance for the given ArgoCD using the given name.
func newServiceMonitorWithName(name string, cr *argoproj.ArgoCD) *monitoringv1.ServiceMonitor {
	svcmon := newServiceMonitor(cr)
	svcmon.ObjectMeta.Name = name

	lbls := svcmon.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	lbls[common.ArgoCDKeyRelease] = "prometheus-operator"
	svcmon.ObjectMeta.Labels = lbls

	return svcmon
}

// newServiceMonitorWithSuffix returns a new ServiceMonitor instance for the given ArgoCD using the given suffix.
func newServiceMonitorWithSuffix(suffix string, cr *argoproj.ArgoCD) *monitoringv1.ServiceMonitor {
	return newServiceMonitorWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), cr)
}

// reconcilePrometheusRule reconciles the PrometheusRule that triggers alerts based on workload statuses
func (r *ReconcileArgoCD) reconcilePrometheusRule(cr *argoproj.ArgoCD) error {

	promRule := newPrometheusRule(cr.Namespace, "argocd-component-status-alert")

	if argoutil.IsObjectFound(r.Client, cr.Namespace, promRule.Name, promRule) {

		if !cr.Spec.Monitoring.Enabled {
			// PrometheusRule exists but enabled flag has been set to false, delete the PrometheusRule
			log.Info("instance monitoring disabled, deleting component status tracking prometheusRule")
			return r.Client.Delete(context.TODO(), promRule)
		}
		return nil // PrometheusRule found, do nothing
	}

	if !cr.Spec.Monitoring.Enabled {
		return nil // Monitoring not enabled, do nothing.
	}

	ruleGroups := []monitoringv1.RuleGroup{
		{
			Name: "ArgoCDComponentStatus",
			Rules: []monitoringv1.Rule{
				{
					Alert: "ApplicationControllerNotReady",
					Annotations: map[string]string{
						"message": fmt.Sprintf("application controller deployment for Argo CD instance in namespace %s is not running", cr.Namespace),
					},
					Expr: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: fmt.Sprintf("kube_statefulset_status_replicas{statefulset=\"%s\", namespace=\"%s\"} != kube_statefulset_status_replicas_ready{statefulset=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(cr.Name+"-application-controller"), cr.Namespace, fmt.Sprintf(cr.Name+"-application-controller"), cr.Namespace),
					},
					For: "1m",
					Labels: map[string]string{
						"severity": "critical",
					},
				},
				{
					Alert: "ServerNotReady",
					Annotations: map[string]string{
						"message": fmt.Sprintf("server deployment for Argo CD instance in namespace %s is not running", cr.Namespace),
					},
					Expr: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(cr.Name+"-server"), cr.Namespace, fmt.Sprintf(cr.Name+"-server"), cr.Namespace),
					},
					For: "1m",
					Labels: map[string]string{
						"severity": "critical",
					},
				},
				{
					Alert: "RepoServerNotReady",
					Annotations: map[string]string{
						"message": fmt.Sprintf("repo server deployment for Argo CD instance in namespace %s is not running", cr.Namespace),
					},
					Expr: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(cr.Name+"-repo-server"), cr.Namespace, fmt.Sprintf(cr.Name+"-repo-server"), cr.Namespace),
					},
					For: "1m",
					Labels: map[string]string{
						"severity": "critical",
					},
				},
				{
					Alert: "ApplicationSetControllerNotReady",
					Annotations: map[string]string{
						"message": fmt.Sprintf("applicationSet controller deployment for Argo CD instance in namespace %s is not running", cr.Namespace),
					},
					Expr: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(cr.Name+"-applicationset-controller"), cr.Namespace, fmt.Sprintf(cr.Name+"-applicationset-controller"), cr.Namespace),
					},
					For: "5m",
					Labels: map[string]string{
						"severity": "warning",
					},
				},
				{
					Alert: "DexNotReady",
					Annotations: map[string]string{
						"message": fmt.Sprintf("dex deployment for Argo CD instance in namespace %s is not running", cr.Namespace),
					},
					Expr: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(cr.Name+"-dex-server"), cr.Namespace, fmt.Sprintf(cr.Name+"-dex-server"), cr.Namespace),
					},
					For: "5m",
					Labels: map[string]string{
						"severity": "warning",
					},
				},
				{
					Alert: "NotificationsControllerNotReady",
					Annotations: map[string]string{
						"message": fmt.Sprintf("notifications controller deployment for Argo CD instance in namespace %s is not running", cr.Namespace),
					},
					Expr: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(cr.Name+"-notifications-controller"), cr.Namespace, fmt.Sprintf(cr.Name+"-notifications-controller"), cr.Namespace),
					},
					For: "5m",
					Labels: map[string]string{
						"severity": "warning",
					},
				},
				{
					Alert: "RedisNotReady",
					Annotations: map[string]string{
						"message": fmt.Sprintf("redis deployment for Argo CD instance in namespace %s is not running", cr.Namespace),
					},
					Expr: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(cr.Name+"-redis"), cr.Namespace, fmt.Sprintf(cr.Name+"-redis"), cr.Namespace),
					},
					For: "5m",
					Labels: map[string]string{
						"severity": "warning",
					},
				},
			},
		},
	}
	promRule.Spec.Groups = ruleGroups

	if err := controllerutil.SetControllerReference(cr, promRule, r.Scheme); err != nil {
		return err
	}

	log.Info("instance monitoring enabled, creating component status tracking prometheusRule")
	return r.Client.Create(context.TODO(), promRule) // Create PrometheusRule
}

// newPrometheusRule returns an empty PrometheusRule
func newPrometheusRule(namespace, alertRuleName string) *monitoringv1.PrometheusRule {

	promRule := &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      alertRuleName,
			Namespace: namespace,
		},
		Spec: monitoringv1.PrometheusRuleSpec{},
	}
	return promRule
}
