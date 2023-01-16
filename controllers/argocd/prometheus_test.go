package argocd

import (
	"context"
	"fmt"
	"testing"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestReconcileWorkloadStatusAlertRule(t *testing.T) {
	tests := []struct {
		name    string
		argocd  *argoprojv1alpha1.ArgoCD
		wantErr bool
	}{
		{
			name: "monitoring enabled",
			argocd: makeTestArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.Monitoring.Enabled = true
			}),
			wantErr: false,
		},
		{
			name: "monitoring disabled",
			argocd: makeTestArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.Monitoring.Enabled = false
			}),
			wantErr: true,
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
							For: "10m",
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
							For: "10m",
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
							For: "10m",
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
							For: "10m",
							Labels: map[string]string{
								"severity": "warning",
							},
						},
					},
				},
			}

			r := makeTestReconciler(t, test.argocd)
			err := monitoringv1.AddToScheme(r.Scheme)
			assert.NoError(t, err)

			err = r.reconcilePrometheusRule(test.argocd)
			assert.NoError(t, err)

			testRule := &monitoringv1.PrometheusRule{}
			err = r.Client.Get(context.TODO(), types.NamespacedName{
				Name:      "argocd-component-status-alert",
				Namespace: test.argocd.Namespace,
			}, testRule)

			if test.wantErr && err == nil {
				t.Fatal("Expected error but did not get one")
			} else if !test.wantErr && err != nil {
				t.Fatal("Unexpected error")
			}

			if test.argocd.Spec.Monitoring.Enabled {
				assert.Equal(t, desiredRuleGroup, testRule.Spec.Groups)
			}

		})
	}
}
