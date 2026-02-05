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

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
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

// IsPrometheusAPIAvailable returns true if the Prometheus API is present.
func IsPrometheusAPIAvailable() bool {
	return prometheusAPIFound
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
	svcmon.Name = name

	lbls := svcmon.Labels
	lbls[common.ArgoCDKeyName] = name
	lbls[common.ArgoCDKeyRelease] = "prometheus-operator"
	svcmon.Labels = lbls

	return svcmon
}

// newServiceMonitorWithSuffix returns a new ServiceMonitor instance for the given ArgoCD using the given suffix.
func newServiceMonitorWithSuffix(suffix string, cr *argoproj.ArgoCD) *monitoringv1.ServiceMonitor {
	return newServiceMonitorWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), cr)
}

// reconcileMetricsServiceMonitor will ensure that the ServiceMonitor is present for the ArgoCD metrics Service.
func (r *ReconcileArgoCD) reconcileMetricsServiceMonitor(cr *argoproj.ArgoCD) error {
	sm := newServiceMonitorWithSuffix(common.ArgoCDKeyMetrics, cr)
	smExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, sm.Name, sm)
	if err != nil {
		return err
	}
	if smExists {
		if !cr.Spec.Prometheus.Enabled {
			// ServiceMonitor exists but enabled flag has been set to false, delete the ServiceMonitor
			argoutil.LogResourceDeletion(log, sm, "prometheus is disabled")
			return r.Delete(context.TODO(), sm)
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
	argoutil.LogResourceCreation(log, sm)
	return r.Create(context.TODO(), sm)
}

// reconcilePrometheus will ensure that Prometheus CR is deleted.
// The Prometheus CR is deprecated and no longer created by the operator.
// If it exists, it will be deleted.
func (r *ReconcileArgoCD) reconcilePrometheus(cr *argoproj.ArgoCD) error {
	prometheus := newPrometheus(cr)
	prExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, prometheus.Name, prometheus)
	if err != nil {
		return err
	}
	if prExists {
		// Prometheus CR is deprecated, delete it if it exists
		argoutil.LogResourceDeletion(log, prometheus, "prometheus CR is deprecated and no longer supported")
		return r.Delete(context.TODO(), prometheus)
	}

	return nil // Prometheus CR does not exist, nothing to do
}

// reconcileRepoServerServiceMonitor will ensure that the ServiceMonitor is present for the Repo Server metrics Service.
func (r *ReconcileArgoCD) reconcileRepoServerServiceMonitor(cr *argoproj.ArgoCD) error {
	sm := newServiceMonitorWithSuffix("repo-server-metrics", cr)
	smExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, sm.Name, sm)
	if err != nil {
		return err
	}
	if smExists {
		if !cr.Spec.Prometheus.Enabled {
			// ServiceMonitor exists but enabled flag has been set to false, delete the ServiceMonitor
			argoutil.LogResourceDeletion(log, sm, "prometheus is disabled")
			return r.Delete(context.TODO(), sm)
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
	argoutil.LogResourceCreation(log, sm)
	return r.Create(context.TODO(), sm)
}

// reconcileServerMetricsServiceMonitor will ensure that the ServiceMonitor is present for the ArgoCD Server metrics Service.
func (r *ReconcileArgoCD) reconcileServerMetricsServiceMonitor(cr *argoproj.ArgoCD) error {
	sm := newServiceMonitorWithSuffix("server-metrics", cr)
	smExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, sm.Name, sm)
	if err != nil {
		return err
	}
	if smExists {
		if !cr.Spec.Prometheus.Enabled {
			// ServiceMonitor exists but enabled flag has been set to false, delete the ServiceMonitor
			argoutil.LogResourceDeletion(log, sm, "prometheus is disabled")
			return r.Delete(context.TODO(), sm)
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
	argoutil.LogResourceCreation(log, sm)
	return r.Create(context.TODO(), sm)
}

// reconcilePrometheusRule reconciles the PrometheusRule that triggers alerts based on workload statuses
func (r *ReconcileArgoCD) reconcilePrometheusRule(cr *argoproj.ArgoCD) error {

	promRule := newPrometheusRule(cr.Namespace, "argocd-component-status-alert")

	prExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, promRule.Name, promRule)
	if err != nil {
		return err
	}

	if prExists {

		if !cr.Spec.Monitoring.Enabled {
			// PrometheusRule exists but enabled flag has been set to false, delete the PrometheusRule
			log.Info("instance monitoring disabled, deleting component status tracking prometheusRule")
			argoutil.LogResourceDeletion(log, promRule, "instance monitoring is disabled")
			return r.Delete(context.TODO(), promRule)
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
					For: ptr.To((monitoringv1.Duration)("1m")),
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
					For: ptr.To((monitoringv1.Duration)("1m")),
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
					For: ptr.To((monitoringv1.Duration)("1m")),
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
					For: ptr.To((monitoringv1.Duration)("5m")),
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
					For: ptr.To((monitoringv1.Duration)("5m")),
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
					For: ptr.To((monitoringv1.Duration)("5m")),
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
					For: ptr.To((monitoringv1.Duration)("5m")),
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

	argoutil.LogResourceCreation(log, promRule, "for component status tracking, since instance monitoring is enabled")
	return r.Create(context.TODO(), promRule) // Create PrometheusRule
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
