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

	argoproj "github.com/jmckind/argocd-operator/pkg/apis/argoproj/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// getArgoServerServiceType will return the server Service type for the ArgoCD.
func getArgoServerServiceType(cr *argoproj.ArgoCD) corev1.ServiceType {
	if len(cr.Spec.Server.Service.Type) > 0 {
		return cr.Spec.Server.Service.Type
	}
	return corev1.ServiceTypeClusterIP
}

// newService returns a new Service for the given ArgoCD instance.
func newService(cr *argoproj.ArgoCD) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}

// newService retuns a new Service instance.
func newServiceWithName(name string, component string, cr *argoproj.ArgoCD) *corev1.Service {
	svc := newService(cr)
	svc.ObjectMeta.Name = name

	lbls := svc.ObjectMeta.Labels
	lbls[ArgoCDKeyName] = name
	lbls[ArgoCDKeyComponent] = component
	svc.ObjectMeta.Labels = lbls

	return svc
}

// newServiceWithSuffix retuns a new Service instance for the given ArgoCD using the given suffix.
func newServiceWithSuffix(suffix string, component string, cr *argoproj.ArgoCD) *corev1.Service {
	return newServiceWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), component, cr)
}

func (r *ReconcileArgoCD) reconcileDexService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("dex-server", "dex-server", cr)
	found := r.isObjectFound(cr.Namespace, svc.Name, svc)
	if found {
		// Service found, do nothing
		return nil
	}

	svc.Spec.Selector = map[string]string{
		ArgoCDKeyName: nameWithSuffix("dex-server", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http",
			Port:       5556,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(5556),
		}, {
			Name:       "grpc",
			Port:       5557,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(5557),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), svc)
}

func (r *ReconcileArgoCD) reconcileGrafanaService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("grafana", "grafana", cr)
	found := r.isObjectFound(cr.Namespace, svc.Name, svc)
	if found {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		ArgoCDKeyName: nameWithSuffix("grafana", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http",
			Port:       80,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(3000),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), svc)
}

func (r *ReconcileArgoCD) reconcileMetricsService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("metrics", "metrics", cr)
	found := r.isObjectFound(cr.Namespace, svc.Name, svc)
	if found {
		// Service found, do nothing
		return nil
	}

	svc.Spec.Selector = map[string]string{
		ArgoCDKeyName: nameWithSuffix("application-controller", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       8082,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8082),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), svc)
}

func (r *ReconcileArgoCD) reconcileRedisService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("redis", "redis", cr)
	if r.isObjectFound(cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		ArgoCDKeyName: nameWithSuffix("redis", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "tcp-redis",
			Port:       6379,
			TargetPort: intstr.FromInt(6379),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), svc)
}

func (r *ReconcileArgoCD) reconcileRepoService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("repo-server", "repo-server", cr)
	if r.isObjectFound(cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		ArgoCDKeyName: nameWithSuffix("repo-server", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "server",
			Port:       8081,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8081),
		}, {
			Name:       "metrics",
			Port:       8084,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8084),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), svc)
}

func (r *ReconcileArgoCD) reconcileServerMetricsService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("server-metrics", "server", cr)
	if r.isObjectFound(cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		ArgoCDKeyName: nameWithSuffix("server", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       8083,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8083),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), svc)
}

// reconcileServerService will ensure that the Service is present for the Argo CD server component.
func (r *ReconcileArgoCD) reconcileServerService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("server", "server", cr)
	if r.isObjectFound(cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http",
			Port:       80,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8080),
		}, {
			Name:       "https",
			Port:       443,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8080),
		},
	}

	svc.Spec.Selector = map[string]string{
		ArgoCDKeyName: nameWithSuffix("server", cr),
	}

	svc.Spec.Type = getArgoServerServiceType(cr)

	if err := controllerutil.SetControllerReference(cr, svc, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), svc)
}

// reconcileServices will ensure that all Services are present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileServices(cr *argoproj.ArgoCD) error {
	err := r.reconcileDexService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileMetricsService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRedisService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRepoService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileServerMetricsService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileServerService(cr)
	if err != nil {
		return err
	}

	if IsOpenShift() {
		err = r.reconcileGrafanaService(cr)
		if err != nil {
			return err
		}
	}

	return nil
}
