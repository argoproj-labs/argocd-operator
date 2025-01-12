// Copyright 2019 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package argocd

import (
	"context"
	"fmt"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

var prometheusAPIFound = false

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

// IsPrometheusAPIAvailable returns true if the Prometheus API is present.
func IsPrometheusAPIAvailable() bool {
	return prometheusAPIFound
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

// verifyPrometheusAPI will verify that the Prometheus API is present.
func verifyPrometheusAPI() error {
	found, err := argoutil.VerifyAPI(monitoringv1.SchemeGroupVersion.Group, monitoringv1.SchemeGroupVersion.Version)
	if err != nil {
		return err
	}
	prometheusAPIFound = found
	return nil
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

// reconcileMetricsServiceMonitor will ensure that the ServiceMonitor is present for the ArgoCD metrics Service.
func (r *ReconcileArgoCD) reconcileMetricsServiceMonitor(cr *argoproj.ArgoCD) error {
	sm := newServiceMonitorWithSuffix(common.ArgoCDKeyMetrics, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, sm.Name, sm) {
		if !cr.Spec.Prometheus.Enabled {
			// ServiceMonitor exists but enabled flag has been set to false, delete the ServiceMonitor
			return r.Client.Delete(context.TODO(), sm)
		}
		return nil // ServiceMonitor found, do nothing
	}

	if !cr.Spec.Prometheus.Enabled {
		return nil // Prometheus not enabled, do nothing.
	}

	sm.Spec.Selector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.ArgoCDKeyName: nameWithSuffix(common.ArgoCDKeyMetrics, cr),
		},
	}
	sm.Spec.Endpoints = []monitoringv1.Endpoint{
		{
			Port: common.ArgoCDKeyMetrics,
		},
	}

	if err := controllerutil.SetControllerReference(cr, sm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), sm)
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

// reconcileRepoServerServiceMonitor will ensure that the ServiceMonitor is present for the Repo Server metrics Service.
func (r *ReconcileArgoCD) reconcileRepoServerServiceMonitor(cr *argoproj.ArgoCD) error {
	sm := newServiceMonitorWithSuffix("repo-server-metrics", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, sm.Name, sm) {
		if !cr.Spec.Prometheus.Enabled {
			// ServiceMonitor exists but enabled flag has been set to false, delete the ServiceMonitor
			return r.Client.Delete(context.TODO(), sm)
		}
		return nil // ServiceMonitor found, do nothing
	}

	if !cr.Spec.Prometheus.Enabled {
		return nil // Prometheus not enabled, do nothing.
	}

	sm.Spec.Selector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.ArgoCDKeyName: nameWithSuffix("repo-server", cr),
		},
	}
	sm.Spec.Endpoints = []monitoringv1.Endpoint{
		{
			Port: common.ArgoCDKeyMetrics,
		},
	}

	if err := controllerutil.SetControllerReference(cr, sm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), sm)
}

// reconcileServerMetricsServiceMonitor will ensure that the ServiceMonitor is present for the ArgoCD Server metrics Service.
func (r *ReconcileArgoCD) reconcileServerMetricsServiceMonitor(cr *argoproj.ArgoCD) error {
	sm := newServiceMonitorWithSuffix("server-metrics", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, sm.Name, sm) {
		if !cr.Spec.Prometheus.Enabled {
			// ServiceMonitor exists but enabled flag has been set to false, delete the ServiceMonitor
			return r.Client.Delete(context.TODO(), sm)
		}
		return nil // ServiceMonitor found, do nothing
	}

	if !cr.Spec.Prometheus.Enabled {
		return nil // Prometheus not enabled, do nothing.
	}

	sm.Spec.Selector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.ArgoCDKeyName: nameWithSuffix("server-metrics", cr),
		},
	}
	sm.Spec.Endpoints = []monitoringv1.Endpoint{
		{
			Port: common.ArgoCDKeyMetrics,
		},
	}

	if err := controllerutil.SetControllerReference(cr, sm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), sm)
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
						StrVal: fmt.Sprintf("kube_statefulset_status_replicas{statefulset=\"%s\", namespace=\"%s\"} != kube_statefulset_status_replicas_ready{statefulset=\"%s\", namespace=\"%s\"} ", cr.Name+"-application-controller", cr.Namespace, cr.Name+"-application-controller", cr.Namespace),
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
						StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", cr.Name+"-server", cr.Namespace, cr.Name+"-server", cr.Namespace),
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
						StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", cr.Name+"-repo-server", cr.Namespace, cr.Name+"-repo-server", cr.Namespace),
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
						StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", cr.Name+"-applicationset-controller", cr.Namespace, cr.Name+"-applicationset-controller", cr.Namespace),
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
						StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", cr.Name+"-dex-server", cr.Namespace, cr.Name+"-dex-server", cr.Namespace),
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
						StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", cr.Name+"-notifications-controller", cr.Namespace, cr.Name+"-notifications-controller", cr.Namespace),
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
						StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", cr.Name+"-redis", cr.Namespace, cr.Name+"-redis", cr.Namespace),
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
