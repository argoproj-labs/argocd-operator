package argocd

import (
	"context"
	"fmt"
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReconcileWorkloadStatusAlertRule(t *testing.T) {
	tests := []struct {
		name              string
		argocd            *argoproj.ArgoCD
		wantPromRuleFound bool
		existingPromRule  bool
	}{
		{
			name: "monitoring enabled, no existing prom rule",
			argocd: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.Monitoring.Enabled = true
			}),
			existingPromRule:  false,
			wantPromRuleFound: true,
		},
		{
			name: "monitoring disabled, no existing prom rule",
			argocd: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.Monitoring.Enabled = false
			}),
			existingPromRule:  false,
			wantPromRuleFound: false,
		},
		{
			name: "monitoring enabled, existing prom rule",
			argocd: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.Monitoring.Enabled = true
			}),
			existingPromRule:  true,
			wantPromRuleFound: true,
		},
		{
			name: "monitoring disabled, existing prom rule",
			argocd: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.Monitoring.Enabled = false
			}),
			existingPromRule:  true,
			wantPromRuleFound: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			desiredRuleGroup := []monitoringv1.RuleGroup{
				{
					Name: "ArgoCDComponentStatus",
					Rules: []monitoringv1.Rule{
						{
							Alert: "ApplicationControllerNotReady",
							Annotations: map[string]string{
								"message": fmt.Sprintf("application controller deployment for Argo CD instance in namespace %s is not running", test.argocd.Namespace),
							},
							Expr: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: fmt.Sprintf("kube_statefulset_status_replicas{statefulset=\"%s\", namespace=\"%s\"} != kube_statefulset_status_replicas_ready{statefulset=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(test.argocd.Name+"-application-controller"), test.argocd.Namespace, fmt.Sprintf(test.argocd.Name+"-application-controller"), test.argocd.Namespace),
							},
							For: "1m",
							Labels: map[string]string{
								"severity": "critical",
							},
						},
						{
							Alert: "ServerNotReady",
							Annotations: map[string]string{
								"message": fmt.Sprintf("server deployment for Argo CD instance in namespace %s is not running", test.argocd.Namespace),
							},
							Expr: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(test.argocd.Name+"-server"), test.argocd.Namespace, fmt.Sprintf(test.argocd.Name+"-server"), test.argocd.Namespace),
							},
							For: "1m",
							Labels: map[string]string{
								"severity": "critical",
							},
						},
						{
							Alert: "RepoServerNotReady",
							Annotations: map[string]string{
								"message": fmt.Sprintf("repo server deployment for Argo CD instance in namespace %s is not running", test.argocd.Namespace),
							},
							Expr: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(test.argocd.Name+"-repo-server"), test.argocd.Namespace, fmt.Sprintf(test.argocd.Name+"-repo-server"), test.argocd.Namespace),
							},
							For: "1m",
							Labels: map[string]string{
								"severity": "critical",
							},
						},
						{
							Alert: "ApplicationSetControllerNotReady",
							Annotations: map[string]string{
								"message": fmt.Sprintf("applicationSet controller deployment for Argo CD instance in namespace %s is not running", test.argocd.Namespace),
							},
							Expr: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(test.argocd.Name+"-applicationset-controller"), test.argocd.Namespace, fmt.Sprintf(test.argocd.Name+"-applicationset-controller"), test.argocd.Namespace),
							},
							For: "5m",
							Labels: map[string]string{
								"severity": "warning",
							},
						},
						{
							Alert: "DexNotReady",
							Annotations: map[string]string{
								"message": fmt.Sprintf("dex deployment for Argo CD instance in namespace %s is not running", test.argocd.Namespace),
							},
							Expr: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(test.argocd.Name+"-dex-server"), test.argocd.Namespace, fmt.Sprintf(test.argocd.Name+"-dex-server"), test.argocd.Namespace),
							},
							For: "5m",
							Labels: map[string]string{
								"severity": "warning",
							},
						},
						{
							Alert: "NotificationsControllerNotReady",
							Annotations: map[string]string{
								"message": fmt.Sprintf("notifications controller deployment for Argo CD instance in namespace %s is not running", test.argocd.Namespace),
							},
							Expr: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(test.argocd.Name+"-notifications-controller"), test.argocd.Namespace, fmt.Sprintf(test.argocd.Name+"-notifications-controller"), test.argocd.Namespace),
							},
							For: "5m",
							Labels: map[string]string{
								"severity": "warning",
							},
						},
						{
							Alert: "RedisNotReady",
							Annotations: map[string]string{
								"message": fmt.Sprintf("redis deployment for Argo CD instance in namespace %s is not running", test.argocd.Namespace),
							},
							Expr: intstr.IntOrString{
								Type:   intstr.String,
								StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", fmt.Sprintf(test.argocd.Name+"-redis"), test.argocd.Namespace, fmt.Sprintf(test.argocd.Name+"-redis"), test.argocd.Namespace),
							},
							For: "5m",
							Labels: map[string]string{
								"severity": "warning",
							},
						},
					},
				},
			}

			resObjs := []client.Object{test.argocd}
			subresObjs := []client.Object{test.argocd}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch)

			err := monitoringv1.AddToScheme(r.Scheme)
			assert.NoError(t, err)

			if test.existingPromRule {
				r.Client.Create(context.TODO(), newPrometheusRule(test.argocd.Namespace, "argocd-component-status-alert"))
			}

			err = r.reconcilePrometheusRule(test.argocd)

			// reconciler doesn't need to do anything and should return nil
			if (test.existingPromRule && test.wantPromRuleFound) || (!test.existingPromRule && !test.wantPromRuleFound) {
				if err != nil {
					t.Fatal("expected nil response but got non-nil response")
				}
			} else {
				// reconciler either needs to create rule or delete it
				testRule := &monitoringv1.PrometheusRule{}
				err = r.Client.Get(context.TODO(), types.NamespacedName{
					Name:      "argocd-component-status-alert",
					Namespace: test.argocd.Namespace,
				}, testRule)

				if test.wantPromRuleFound && err != nil {
					t.Fatal("unexpected error - prometheusRule not found")
				} else if !test.wantPromRuleFound && err == nil {
					t.Fatal("expected error but did not get one - prometheusRule not deleted")
				}

				if !test.existingPromRule {
					assert.Equal(t, desiredRuleGroup, testRule.Spec.Groups)
				}

			}
		})
	}
}
