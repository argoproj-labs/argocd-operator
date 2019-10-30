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
				"prometheus":                "k8s",
				"app.kubernetes.io/name":    name,
				"app.kubernetes.io/part-of": "argocd",
			},
		},
	}
}

// newServiceMonitor retuns a new ServiceMonitor instance.
func newServiceMonitor(name string, namespace string) *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":    name,
				"app.kubernetes.io/part-of": "argocd",
				"release":                   "prometheus-operator",
			},
		},
	}
}

func (r *ReconcileArgoCD) reconcilePrometheus(cr *argoproj.ArgoCD) error {
	prometheus := newPrometheus("argocd-prometheus", cr.Namespace)
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: prometheus.Name}, prometheus)
	if found {
		return nil // Prometheus found, do nothing
	}

	var replicas int32 = 2
	prometheus.Spec.Replicas = &replicas
	prometheus.Spec.ServiceAccountName = "prometheus-k8s"
	prometheus.Spec.ServiceMonitorSelector = &metav1.LabelSelector{}

	if err := controllerutil.SetControllerReference(cr, prometheus, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), prometheus)
}

func (r *ReconcileArgoCD) reconcileMetricsServiceMonitor(cr *argoproj.ArgoCD) error {
	sm := newServiceMonitor("argocd-metrics", cr.Namespace)
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: sm.Name}, sm)
	if found {
		return nil // ServiceMonitor found, do nothing
	}

	sm.Spec.Selector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/name": "argocd-metrics",
		},
	}
	sm.Spec.Endpoints = []monitoringv1.Endpoint{
		{
			Port: "metrics",
		},
	}

	if err := controllerutil.SetControllerReference(cr, sm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), sm)
}

func (r *ReconcileArgoCD) reconcileRepoServerServiceMonitor(cr *argoproj.ArgoCD) error {
	sm := newServiceMonitor("argocd-repo-server-metrics", cr.Namespace)
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: sm.Name}, sm)
	if found {
		return nil // ServiceMonitor found, do nothing
	}

	sm.Spec.Selector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/name": "argocd-repo-server",
		},
	}
	sm.Spec.Endpoints = []monitoringv1.Endpoint{
		{
			Port: "metrics",
		},
	}

	if err := controllerutil.SetControllerReference(cr, sm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), sm)
}

func (r *ReconcileArgoCD) reconcileServerMetricsServiceMonitor(cr *argoproj.ArgoCD) error {
	sm := newServiceMonitor("argocd-server-metrics", cr.Namespace)
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: sm.Name}, sm)
	if found {
		return nil // ServiceMonitor found, do nothing
	}

	sm.Spec.Selector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/name": "argocd-server-metrics",
		},
	}
	sm.Spec.Endpoints = []monitoringv1.Endpoint{
		{
			Port: "metrics",
		},
	}

	if err := controllerutil.SetControllerReference(cr, sm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), sm)
}
