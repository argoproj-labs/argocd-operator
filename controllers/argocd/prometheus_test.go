package argocd

import (
	"context"
	"fmt"
	"testing"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	testclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
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
								StrVal: fmt.Sprintf("kube_statefulset_status_replicas{statefulset=\"%s\", namespace=\"%s\"} != kube_statefulset_status_replicas_ready{statefulset=\"%s\", namespace=\"%s\"} ", test.argocd.Name+"-application-controller", test.argocd.Namespace, test.argocd.Name+"-application-controller", test.argocd.Namespace),
							},
							For: ptr.To((monitoringv1.Duration)("1m")),
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
								StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", test.argocd.Name+"-server", test.argocd.Namespace, test.argocd.Name+"-server", test.argocd.Namespace),
							},
							For: ptr.To((monitoringv1.Duration)("1m")),
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
								StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", test.argocd.Name+"-repo-server", test.argocd.Namespace, test.argocd.Name+"-repo-server", test.argocd.Namespace),
							},
							For: ptr.To((monitoringv1.Duration)("1m")),
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
								StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", test.argocd.Name+"-applicationset-controller", test.argocd.Namespace, test.argocd.Name+"-applicationset-controller", test.argocd.Namespace),
							},
							For: ptr.To((monitoringv1.Duration)("5m")),
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
								StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", test.argocd.Name+"-dex-server", test.argocd.Namespace, test.argocd.Name+"-dex-server", test.argocd.Namespace),
							},
							For: ptr.To((monitoringv1.Duration)("5m")),
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
								StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", test.argocd.Name+"-notifications-controller", test.argocd.Namespace, test.argocd.Name+"-notifications-controller", test.argocd.Namespace),
							},
							For: ptr.To((monitoringv1.Duration)("5m")),
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
								StrVal: fmt.Sprintf("kube_deployment_status_replicas{deployment=\"%s\", namespace=\"%s\"} != kube_deployment_status_replicas_ready{deployment=\"%s\", namespace=\"%s\"} ", test.argocd.Name+"-redis", test.argocd.Namespace, test.argocd.Name+"-redis", test.argocd.Namespace),
							},
							For: ptr.To((monitoringv1.Duration)("5m")),
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
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			err := monitoringv1.AddToScheme(r.Scheme)
			assert.NoError(t, err)

			if test.existingPromRule {
				err := r.Create(context.TODO(), newPrometheusRule(test.argocd.Namespace, "argocd-component-status-alert"))
				assert.NoError(t, err)
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
				err = r.Get(context.TODO(), types.NamespacedName{
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

func TestReconcilePrometheus_Deleted(t *testing.T) {
	tests := []struct {
		name                string
		argocd              *argoproj.ArgoCD
		existingPrometheus  bool
		wantPrometheusFound bool
	}{
		{
			name: "prometheus enabled, no existing Prometheus CR, should not create",
			argocd: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.Prometheus.Enabled = true
			}),
			existingPrometheus:  false,
			wantPrometheusFound: false,
		},
		{
			name: "prometheus disabled, no existing Prometheus CR, should not create",
			argocd: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.Prometheus.Enabled = false
			}),
			existingPrometheus:  false,
			wantPrometheusFound: false,
		},
		{
			name: "prometheus enabled, existing Prometheus CR, should delete",
			argocd: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.Prometheus.Enabled = true
			}),
			existingPrometheus:  true,
			wantPrometheusFound: false,
		},
		{
			name: "prometheus disabled, existing Prometheus CR, should delete",
			argocd: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.Prometheus.Enabled = false
			}),
			existingPrometheus:  true,
			wantPrometheusFound: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resObjs := []client.Object{test.argocd}
			subresObjs := []client.Object{test.argocd}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			err := monitoringv1.AddToScheme(r.Scheme)
			assert.NoError(t, err)

			if test.existingPrometheus {
				prometheus := newPrometheus(test.argocd)
				err := r.Create(context.TODO(), prometheus)
				assert.NoError(t, err)
			}

			err = r.reconcilePrometheus(test.argocd)
			assert.NoError(t, err)

			// Verify the Prometheus CR state
			testPrometheus := &monitoringv1.Prometheus{}
			err = r.Get(context.TODO(), types.NamespacedName{
				Name:      test.argocd.Name,
				Namespace: test.argocd.Namespace,
			}, testPrometheus)

			if test.wantPrometheusFound {
				assert.NoError(t, err, "expected Prometheus CR to exist")
			} else {
				assert.True(t, errors.IsNotFound(err), "expected Prometheus CR to not exist")
			}
		})
	}
}

func TestReconcileMetricsServiceMonitor(t *testing.T) {
	tests := []struct {
		name           string
		argocd         *argoproj.ArgoCD
		wantSMFound    bool
		wantInterval   monitoringv1.Duration
		wantScrTimeout monitoringv1.Duration
	}{
		{
			name: "prometheus enabled, no metrics config",
			argocd: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.Prometheus.Enabled = true
			}),
			wantSMFound: true,
		},
		{
			name: "prometheus disabled",
			argocd: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.Prometheus.Enabled = false
			}),
			wantSMFound: false,
		},
		{
			name: "prometheus enabled, controller metrics set",
			argocd: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.Prometheus.Enabled = true
				cr.Spec.Controller.Metrics = &argoproj.ArgoCDMetricsSpec{
					Interval:      "45s",
					ScrapeTimeout: "20s",
				}
			}),
			wantSMFound:    true,
			wantInterval:   "45s",
			wantScrTimeout: "20s",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resObjs := []client.Object{test.argocd}
			subresObjs := []client.Object{test.argocd}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			err := monitoringv1.AddToScheme(r.Scheme)
			assert.NoError(t, err)

			err = r.reconcileMetricsServiceMonitor(test.argocd)
			assert.NoError(t, err)

			sm := &monitoringv1.ServiceMonitor{}
			err = r.Get(context.TODO(), types.NamespacedName{
				Name:      fmt.Sprintf("%s-%s", test.argocd.Name, common.ArgoCDKeyMetrics),
				Namespace: test.argocd.Namespace,
			}, sm)

			if test.wantSMFound {
				assert.NoError(t, err)
				assert.Equal(t, test.wantInterval, sm.Spec.Endpoints[0].Interval)
				assert.Equal(t, test.wantScrTimeout, sm.Spec.Endpoints[0].ScrapeTimeout)
			} else {
				assert.True(t, errors.IsNotFound(err))
			}
		})
	}
}

func TestReconcileRepoServerServiceMonitor(t *testing.T) {
	tests := []struct {
		name           string
		argocd         *argoproj.ArgoCD
		wantSMFound    bool
		wantInterval   monitoringv1.Duration
		wantScrTimeout monitoringv1.Duration
	}{
		{
			name: "prometheus enabled, no metrics config",
			argocd: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.Prometheus.Enabled = true
			}),
			wantSMFound: true,
		},
		{
			name: "prometheus disabled",
			argocd: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.Prometheus.Enabled = false
			}),
			wantSMFound: false,
		},
		{
			name: "prometheus enabled, repo metrics set",
			argocd: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.Prometheus.Enabled = true
				cr.Spec.Repo.Metrics = &argoproj.ArgoCDMetricsSpec{
					Interval:      "90s",
					ScrapeTimeout: "45s",
				}
			}),
			wantSMFound:    true,
			wantInterval:   "90s",
			wantScrTimeout: "45s",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resObjs := []client.Object{test.argocd}
			subresObjs := []client.Object{test.argocd}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			err := monitoringv1.AddToScheme(r.Scheme)
			assert.NoError(t, err)

			err = r.reconcileRepoServerServiceMonitor(test.argocd)
			assert.NoError(t, err)

			sm := &monitoringv1.ServiceMonitor{}
			err = r.Get(context.TODO(), types.NamespacedName{
				Name:      fmt.Sprintf("%s-repo-server-metrics", test.argocd.Name),
				Namespace: test.argocd.Namespace,
			}, sm)

			if test.wantSMFound {
				assert.NoError(t, err)
				assert.Equal(t, test.wantInterval, sm.Spec.Endpoints[0].Interval)
				assert.Equal(t, test.wantScrTimeout, sm.Spec.Endpoints[0].ScrapeTimeout)
			} else {
				assert.True(t, errors.IsNotFound(err))
			}
		})
	}
}

func TestReconcileServerMetricsServiceMonitor(t *testing.T) {
	tests := []struct {
		name           string
		argocd         *argoproj.ArgoCD
		wantSMFound    bool
		wantInterval   monitoringv1.Duration
		wantScrTimeout monitoringv1.Duration
	}{
		{
			name: "prometheus enabled, no metrics config",
			argocd: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.Prometheus.Enabled = true
			}),
			wantSMFound: true,
		},
		{
			name: "prometheus disabled",
			argocd: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.Prometheus.Enabled = false
			}),
			wantSMFound: false,
		},
		{
			name: "prometheus enabled, server metrics set",
			argocd: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.Prometheus.Enabled = true
				cr.Spec.Server.Metrics = &argoproj.ArgoCDMetricsSpec{
					Interval:      "120s",
					ScrapeTimeout: "60s",
				}
			}),
			wantSMFound:    true,
			wantInterval:   "120s",
			wantScrTimeout: "60s",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resObjs := []client.Object{test.argocd}
			subresObjs := []client.Object{test.argocd}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			err := monitoringv1.AddToScheme(r.Scheme)
			assert.NoError(t, err)

			err = r.reconcileServerMetricsServiceMonitor(test.argocd)
			assert.NoError(t, err)

			sm := &monitoringv1.ServiceMonitor{}
			err = r.Get(context.TODO(), types.NamespacedName{
				Name:      fmt.Sprintf("%s-server-metrics", test.argocd.Name),
				Namespace: test.argocd.Namespace,
			}, sm)

			if test.wantSMFound {
				assert.NoError(t, err)
				assert.Equal(t, test.wantInterval, sm.Spec.Endpoints[0].Interval)
				assert.Equal(t, test.wantScrTimeout, sm.Spec.Endpoints[0].ScrapeTimeout)
			} else {
				assert.True(t, errors.IsNotFound(err))
			}
		})
	}
}

func TestServiceMonitorEndpointUpdate(t *testing.T) {
	a := makeTestArgoCD(func(cr *argoproj.ArgoCD) {
		cr.Spec.Prometheus.Enabled = true
		cr.Spec.Controller.Metrics = &argoproj.ArgoCDMetricsSpec{
			Interval: "30s",
		}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err := monitoringv1.AddToScheme(r.Scheme)
	assert.NoError(t, err)

	err = r.reconcileMetricsServiceMonitor(a)
	assert.NoError(t, err)

	sm := &monitoringv1.ServiceMonitor{}
	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", a.Name, common.ArgoCDKeyMetrics),
		Namespace: a.Namespace,
	}, sm)
	assert.NoError(t, err)
	assert.Equal(t, monitoringv1.Duration("30s"), sm.Spec.Endpoints[0].Interval)

	a.Spec.Controller.Metrics.Interval = "60s"
	a.Spec.Controller.Metrics.ScrapeTimeout = "25s"
	err = r.reconcileMetricsServiceMonitor(a)
	assert.NoError(t, err)

	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", a.Name, common.ArgoCDKeyMetrics),
		Namespace: a.Namespace,
	}, sm)
	assert.NoError(t, err)
	assert.Equal(t, monitoringv1.Duration("60s"), sm.Spec.Endpoints[0].Interval)
	assert.Equal(t, monitoringv1.Duration("25s"), sm.Spec.Endpoints[0].ScrapeTimeout)

	a.Spec.Controller.Metrics = nil
	err = r.reconcileMetricsServiceMonitor(a)
	assert.NoError(t, err)

	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", a.Name, common.ArgoCDKeyMetrics),
		Namespace: a.Namespace,
	}, sm)
	assert.NoError(t, err)
	assert.Empty(t, sm.Spec.Endpoints[0].Interval)
	assert.Empty(t, sm.Spec.Endpoints[0].ScrapeTimeout)
}
