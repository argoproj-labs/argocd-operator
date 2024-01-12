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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
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
			Labels:    argoutil.LabelsForCluster(cr),
		},
	}
}

// newServiceWithName returns a new Service instance for the given ArgoCD using the given name.
func newServiceWithName(name string, component string, cr *argoproj.ArgoCD) *corev1.Service {
	svc := newService(cr)
	svc.ObjectMeta.Name = name

	lbls := svc.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	lbls[common.ArgoCDKeyComponent] = component
	svc.ObjectMeta.Labels = lbls

	return svc
}

// newServiceWithSuffix returns a new Service instance for the given ArgoCD using the given suffix.
func newServiceWithSuffix(suffix string, component string, cr *argoproj.ArgoCD) *corev1.Service {
	return newServiceWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), component, cr)
}

// reconcileGrafanaService will ensure that the Service for Grafana is present.
func (r *ReconcileArgoCD) reconcileGrafanaService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("grafana", "grafana", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		if !cr.Spec.Grafana.Enabled {
			// Service exists but enabled flag has been set to false, delete the Service
			return r.Client.Delete(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("grafana", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http",
			Port:       80,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(3000),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("creating service %s for Argo CD instance %s in namespace %s", svc.Name, cr.Name, cr.Namespace))
	return r.Client.Create(context.TODO(), svc)
}

// reconcileMetricsService will ensure that the Service for the Argo CD application controller metrics is present.
func (r *ReconcileArgoCD) reconcileMetricsService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("metrics", "metrics", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		// Service found, do nothing
		return nil
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("application-controller", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       8082,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8082),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileRepoService will ensure that the Service for the Argo CD repo server is present.
func (r *ReconcileArgoCD) reconcileRepoService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("repo-server", "repo-server", cr)

	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		if !cr.Spec.Repo.IsEnabled() {
			return r.Client.Delete(context.TODO(), svc)
		}
		if ensureAutoTLSAnnotation(svc, common.ArgoCDRepoServerTLSSecretName, cr.Spec.Repo.WantsAutoTLS()) {
			return r.Client.Update(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	if !cr.Spec.Repo.IsEnabled() {
		return nil
	}

	ensureAutoTLSAnnotation(svc, common.ArgoCDRepoServerTLSSecretName, cr.Spec.Repo.WantsAutoTLS())

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("repo-server", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "server",
			Port:       common.ArgoCDDefaultRepoServerPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ArgoCDDefaultRepoServerPort),
		}, {
			Name:       "metrics",
			Port:       common.ArgoCDDefaultRepoMetricsPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ArgoCDDefaultRepoMetricsPort),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileServerMetricsService will ensure that the Service for the Argo CD server metrics is present.
func (r *ReconcileArgoCD) reconcileServerMetricsService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("server-metrics", "server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("server", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       8083,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8083),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileServerService will ensure that the Service is present for the Argo CD server component.
func (r *ReconcileArgoCD) reconcileServerService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("server", "server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		if !cr.Spec.Server.IsEnabled() {
			return r.Client.Delete(context.TODO(), svc)
		}
		if ensureAutoTLSAnnotation(svc, common.ArgoCDServerTLSSecretName, cr.Spec.Server.WantsAutoTLS()) {
			return r.Client.Update(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	if !cr.Spec.Repo.IsEnabled() {
		return nil
	}

	ensureAutoTLSAnnotation(svc, common.ArgoCDServerTLSSecretName, cr.Spec.Server.WantsAutoTLS())

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
		common.ArgoCDKeyName: nameWithSuffix("server", cr),
	}

	svc.Spec.Type = getArgoServerServiceType(cr)

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileServices will ensure that all Services are present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileServices(cr *argoproj.ArgoCD) error {

	if err := r.reconcileDexService(cr); err != nil {
		log.Error(err, "error reconciling dex service")
	}

	err := r.reconcileGrafanaService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileMetricsService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRedisHAServices(cr)
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
	return nil
}
