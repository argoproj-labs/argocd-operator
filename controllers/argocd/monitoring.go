package argocd

import (
	"fmt"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/monitoring"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ArgoCDReconciler) reoncilePrometheusRule() error {
	req := monitoring.PrometheusRuleRequest{
		ObjectMeta: argoutil.GetObjMeta(
			common.ArogCDComponentStatusAlertRuleName, r.Instance.Namespace, r.Instance.Name, r.Instance.Namespace, "", util.EmptyMap(), util.EmptyMap(),
		),
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name: "ArgoCDComponentStatus",
					Rules: []monitoringv1.Rule{
						{
							Alert: "ApplicationControllerNotReady",
							Annotations: map[string]string{
								"message": fmt.Sprintf("application controller deployment for Argo CD instance in namespace %s is not running", r.Instance.Namespace),
							},
							Expr: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: fmt.Sprintf("kube_statefulset_status_replicas{statefulset=\"%s\", namespace=\"%s\"} != kube_statefulset_status_replicas_ready{statefulset=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(r.Instance.Name+"-application-controller"), r.Instance.Namespace, fmt.Sprintf(r.Instance.Name+"-application-controller"), r.Instance.Namespace),
							},
							For: "1m",
							Labels: map[string]string{
								"severity": "r.Instanceitical",
							},
						},
						{
							Alert: "ServerNotReady",
							Annotations: map[string]string{
								"message": fmt.Sprintf("server deployment for Argo CD instance in namespace %s is not running", r.Instance.Namespace),
							},
							Expr: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(r.Instance.Name+"-server"), r.Instance.Namespace, fmt.Sprintf(r.Instance.Name+"-server"), r.Instance.Namespace),
							},
							For: "1m",
							Labels: map[string]string{
								"severity": "r.Instanceitical",
							},
						},
						{
							Alert: "RepoServerNotReady",
							Annotations: map[string]string{
								"message": fmt.Sprintf("repo server deployment for Argo CD instance in namespace %s is not running", r.Instance.Namespace),
							},
							Expr: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(r.Instance.Name+"-repo-server"), r.Instance.Namespace, fmt.Sprintf(r.Instance.Name+"-repo-server"), r.Instance.Namespace),
							},
							For: "1m",
							Labels: map[string]string{
								"severity": "r.Instanceitical",
							},
						},
						{
							Alert: "ApplicationSetControllerNotReady",
							Annotations: map[string]string{
								"message": fmt.Sprintf("applicationSet controller deployment for Argo CD instance in namespace %s is not running", r.Instance.Namespace),
							},
							Expr: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(r.Instance.Name+"-applicationset-controller"), r.Instance.Namespace, fmt.Sprintf(r.Instance.Name+"-applicationset-controller"), r.Instance.Namespace),
							},
							For: "5m",
							Labels: map[string]string{
								"severity": "warning",
							},
						},
						{
							Alert: "DexNotReady",
							Annotations: map[string]string{
								"message": fmt.Sprintf("dex deployment for Argo CD instance in namespace %s is not running", r.Instance.Namespace),
							},
							Expr: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(r.Instance.Name+"-dex-server"), r.Instance.Namespace, fmt.Sprintf(r.Instance.Name+"-dex-server"), r.Instance.Namespace),
							},
							For: "5m",
							Labels: map[string]string{
								"severity": "warning",
							},
						},
						{
							Alert: "NotificationsControllerNotReady",
							Annotations: map[string]string{
								"message": fmt.Sprintf("notifications controller deployment for Argo CD instance in namespace %s is not running", r.Instance.Namespace),
							},
							Expr: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(r.Instance.Name+"-notifications-controller"), r.Instance.Namespace, fmt.Sprintf(r.Instance.Name+"-notifications-controller"), r.Instance.Namespace),
							},
							For: "5m",
							Labels: map[string]string{
								"severity": "warning",
							},
						},
						{
							Alert: "RedisNotReady",
							Annotations: map[string]string{
								"message": fmt.Sprintf("redis deployment for Argo CD instance in namespace %s is not running", r.Instance.Namespace),
							},
							Expr: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(r.Instance.Name+"-redis"), r.Instance.Namespace, fmt.Sprintf(r.Instance.Name+"-redis"), r.Instance.Namespace),
							},
							For: "5m",
							Labels: map[string]string{
								"severity": "warning",
							},
						},
					},
				},
			},
		},
	}

	ignoreDrift := true

	return r.reconPrometheusRule(req, nil, ignoreDrift)
}

func (r *ArgoCDReconciler) reconPrometheusRule(req monitoring.PrometheusRuleRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := monitoring.RequestPrometheusRule(req)
	if err != nil {
		r.Logger.Debug("reconPrometheusRule: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconPrometheusRule: failed to request PrometheusRule %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(r.Instance, desired, r.Scheme); err != nil {
		r.Logger.Error(err, "reconPrometheusRule: failed to set owner reference for PrometheusRule", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := monitoring.GetPrometheusRule(desired.Name, desired.Namespace, r.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconPrometheusRule: failed to retrieve PrometheusRule %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = monitoring.CreatePrometheusRule(desired, r.Client); err != nil {
			return errors.Wrapf(err, "reconPrometheusRule: failed to r.Instanceeate PrometheusRule %s in namespace %s", desired.Name, desired.Namespace)
		}
		r.Logger.Info("PrometheusRule r.Instanceeated", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// PrometheusRule found, no update required - nothing to do
	if ignoreDrift {
		return nil
	}

	changed := false

	// execute supplied update function
	if updateFn != nil {
		if fn, ok := updateFn.(argocdcommon.UpdateFnPR); ok {
			if err := fn(existing, desired, &changed); err != nil {
				return errors.Wrapf(err, "reconPrometheusRule: failed to execute update function for %s in namespace %s", existing.Name, existing.Namespace)
			}
		}
	}

	if !changed {
		return nil
	}

	if err = monitoring.UpdatePrometheusRule(existing, r.Client); err != nil {
		return errors.Wrapf(err, "reconPrometheusRule: failed to update PrometheusRule %s", existing.Name)
	}

	r.Logger.Info("PrometheusRule updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

func (r *ArgoCDReconciler) deletePrometheusRule(name, namespace string) error {
	// Return if Prometheus API is not present on the cluster
	if !monitoring.IsPrometheusAPIAvailable() {
		r.Logger.Debug("Prometheus API unavailable, skipping PrometheusRule deletion")
		return nil
	}

	if err := monitoring.DeletePrometheusRule(name, namespace, r.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deletePrometheusRule: failed to delete PrometheusRule %s in namespace %s", name, namespace)
	}
	r.Logger.Info("PrometheusRule deleted", "name", name, "namespace", namespace)
	return nil
}
