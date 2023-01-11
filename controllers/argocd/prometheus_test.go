package argocd

import (
	"context"
	"fmt"
	"testing"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestReconcileWorkloadStatusAlertRule(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	cr := makeTestArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
		cr.Spec.Monitoring.Enabled = true
	})

	r := makeTestReconciler(t, cr)
	err := monitoringv1.AddToScheme(r.Scheme)
	assert.NoError(t, err)

	err = r.reconcilePrometheusRule(cr)
	assert.NoError(t, err)

	testRule := &monitoringv1.PrometheusRule{}
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-component-status-alert",
		Namespace: cr.Namespace,
	}, testRule))

	desiredRuleGroup := []monitoringv1.RuleGroup{
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
					For: "10m",
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
					For: "10m",
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
					For: "10m",
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
					For: "10m",
					Labels: map[string]string{
						"severity": "warning",
					},
				},
			},
		},
	}

	assert.Equal(t, desiredRuleGroup, testRule.Spec.Groups)

	cr.Spec.Monitoring.Enabled = false
	err = r.reconcilePrometheusRule(cr)
	assert.NoError(t, err)

	testRule = &monitoringv1.PrometheusRule{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-component-status-alert",
		Namespace: cr.Namespace,
	}, testRule)
	assert.True(t, errors.IsNotFound(err))
}
