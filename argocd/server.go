// Copyright 2019 Argo CD Operator Developers
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
	"time"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/resources"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/sethvargo/go-password/password"
	autoscaling "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// getArgoServerCommand will return the command for the ArgoCD server component.
func getArgoServerCommand(cr *v1alpha1.ArgoCD) []string {
	cmd := make([]string, 0)
	cmd = append(cmd, "argocd-server")

	if getArgoServerInsecure(cr) {
		cmd = append(cmd, "--insecure")
	}

	cmd = append(cmd, "--staticassets")
	cmd = append(cmd, "/shared/app")

	cmd = append(cmd, "--dex-server")
	cmd = append(cmd, getDexServerAddress(cr))

	cmd = append(cmd, "--repo-server")
	cmd = append(cmd, geRepoServerAddress(cr))

	cmd = append(cmd, "--redis")
	cmd = append(cmd, getRedisServerAddress(cr))

	return cmd
}

// getArgoServerGRPCHost will return the GRPC host for the given ArgoCD.
func getArgoServerGRPCHost(cr *v1alpha1.ArgoCD) string {
	host := common.NameWithSuffix(cr.ObjectMeta, "grpc")
	if len(cr.Spec.Server.GRPC.Host) > 0 {
		host = cr.Spec.Server.GRPC.Host
	}
	return host
}

// getArgoServerHost will return the host for the given ArgoCD.
func getArgoServerHost(cr *v1alpha1.ArgoCD) string {
	host := cr.Name
	if len(cr.Spec.Server.Host) > 0 {
		host = cr.Spec.Server.Host
	}
	return host
}

// getArgoServerInsecure returns the insecure value for the ArgoCD Server component.
func getArgoServerInsecure(cr *v1alpha1.ArgoCD) bool {
	return cr.Spec.Server.Insecure
}

// getArgoServerOperationProcessors will return the numeric Operation Processors value for the ArgoCD Server.
func getArgoServerOperationProcessors(cr *v1alpha1.ArgoCD) int32 {
	op := common.ArgoCDDefaultServerOperationProcessors
	if cr.Spec.Controller.Processors.Operation > op {
		op = cr.Spec.Controller.Processors.Operation
	}
	return op
}

// getArgoServerResources will return the ResourceRequirements for the Argo CD server container.
func getArgoServerResources(cr *v1alpha1.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	if cr.Spec.Server.Autoscale.Enabled {
		resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(common.ArgoCDDefaultServerResourceLimitCPU),
				corev1.ResourceMemory: resource.MustParse(common.ArgoCDDefaultServerResourceLimitMemory),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(common.ArgoCDDefaultServerResourceRequestCPU),
				corev1.ResourceMemory: resource.MustParse(common.ArgoCDDefaultServerResourceRequestMemory),
			},
		}
	}

	// Allow override of resource requirements from CR
	if cr.Spec.Server.Resources != nil {
		resources = *cr.Spec.Server.Resources
	}

	return resources
}

// getArgoServerStatusProcessors will return the numeric Status Processors value for the ArgoCD Server.
func getArgoServerStatusProcessors(cr *v1alpha1.ArgoCD) int32 {
	sp := common.ArgoCDDefaultServerStatusProcessors
	if cr.Spec.Controller.Processors.Status > sp {
		sp = cr.Spec.Controller.Processors.Status
	}
	return sp
}

// getArgoServerServiceType will return the server Service type for the ArgoCD.
func getArgoServerServiceType(cr *v1alpha1.ArgoCD) corev1.ServiceType {
	if len(cr.Spec.Server.Service.Type) > 0 {
		return cr.Spec.Server.Service.Type
	}
	return corev1.ServiceTypeClusterIP
}

// getArgoServerURI will return the URI for the ArgoCD server.
// The hostname for argocd-server is from the route, ingress or service name in that order.
func (r *ArgoClusterReconciler) getArgoServerURI(cr *v1alpha1.ArgoCD) string {
	host := common.NameWithSuffix(cr.ObjectMeta, "server") // Default to service name

	// Use Ingress host if enabled
	if cr.Spec.Server.Ingress.Enabled {
		ing := resources.NewIngressWithSuffix(cr.ObjectMeta, "server")
		if resources.IsObjectFound(r.Client, cr.Namespace, ing.Name, ing) {
			host = ing.Spec.Rules[0].Host
		}
	}

	// Use Route host if available, override Ingress if both exist
	if resources.IsRouteAPIAvailable() {
		route := resources.NewRouteWithSuffix(cr.ObjectMeta, "server")
		if resources.IsObjectFound(r.Client, cr.Namespace, route.Name, route) {
			host = route.Spec.Host
		}
	}

	return fmt.Sprintf("https://%s", host) // TODO: Safe to assume HTTPS here?
}

// generateArgoServerKey will generate and return the server signature key for session validation.
func generateArgoServerSessionKey() ([]byte, error) {
	pass, err := password.Generate(
		common.ArgoCDDefaultServerSessionKeyLength,
		common.ArgoCDDefaultServerSessionKeyNumDigits,
		common.ArgoCDDefaultServerSessionKeyNumSymbols,
		false, false)

	return []byte(pass), err
}

// getPathOrDefault will return the Ingress Path for the Argo CD component.
func getPathOrDefault(path string) string {
	result := common.ArgoCDDefaultIngressPath
	if len(path) > 0 {
		result = path
	}
	return result
}

// reconcileServerDeployment will ensure the Deployment resource is present for the ArgoCD Server component.
func (r *ArgoClusterReconciler) reconcileServerDeployment(cr *v1alpha1.ArgoCD) error {
	deploy := resources.NewDeploymentWithSuffix(cr.ObjectMeta, "server", "server")
	if resources.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		actualImage := deploy.Spec.Template.Spec.Containers[0].Image
		desiredImage := getArgoContainerImage(cr)
		if actualImage != desiredImage {
			deploy.Spec.Template.Spec.Containers[0].Image = desiredImage
			deploy.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			return r.Client.Update(context.TODO(), deploy)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command:         getArgoServerCommand(cr),
		Image:           getArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8080),
				},
			},
			InitialDelaySeconds: 3,
			PeriodSeconds:       30,
		},
		Name: "argocd-server",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 8080,
			}, {
				ContainerPort: 8083,
			},
		},
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8080),
				},
			},
			InitialDelaySeconds: 3,
			PeriodSeconds:       30,
		},
		Resources: getArgoServerResources(cr),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "ssh-known-hosts",
				MountPath: "/app/config/ssh",
			}, {
				Name:      "tls-certs",
				MountPath: "/app/config/tls",
			},
		},
	}}

	deploy.Spec.Template.Spec.ServiceAccountName = "argocd-server"

	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "ssh-known-hosts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDKnownHostsConfigMapName,
					},
				},
			},
		}, {
			Name: "tls-certs",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDTLSCertsConfigMapName,
					},
				},
			},
		},
	}

	ctrl.SetControllerReference(cr, deploy, r.Scheme)
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileServerHPA will ensure that the HorizontalPodAutoscaler is present for the Argo CD Server component.
func (r *ArgoClusterReconciler) reconcileServerHPA(cr *v1alpha1.ArgoCD) error {
	hpa := resources.NewHorizontalPodAutoscalerWithSuffix(cr.ObjectMeta, "server")
	if resources.IsObjectFound(r.Client, cr.Namespace, hpa.Name, hpa) {
		if !cr.Spec.Server.Autoscale.Enabled {
			return r.Client.Delete(context.TODO(), hpa) // HorizontalPodAutoscaler found but globally disabled, delete it.
		}
		return nil // HorizontalPodAutoscaler found and configured, nothing do to, move along...
	}

	if !cr.Spec.Server.Autoscale.Enabled {
		return nil // AutoScale not enabled, move along...
	}

	if cr.Spec.Server.Autoscale.HPA != nil {
		hpa.Spec = *cr.Spec.Server.Autoscale.HPA
	} else {
		hpa.Spec.MaxReplicas = 3

		var minrReplicas int32 = 1
		hpa.Spec.MinReplicas = &minrReplicas

		var tcup int32 = 50
		hpa.Spec.TargetCPUUtilizationPercentage = &tcup

		hpa.Spec.ScaleTargetRef = autoscaling.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       common.NameWithSuffix(cr.ObjectMeta, "server"),
		}
	}

	return r.Client.Create(context.TODO(), hpa)
}

// reconcileArgoServerIngress will ensure that the ArgoCD Server Ingress is present.
func (r *ArgoClusterReconciler) reconcileArgoServerIngress(cr *v1alpha1.ArgoCD) error {
	ingress := resources.NewIngressWithSuffix(cr.ObjectMeta, "server")
	if resources.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Server.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.Client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Server.Ingress.Enabled {
		return nil // Ingress not enabled, move along...
	}

	// Add annotations
	atns := common.DefaultIngressAnnotations()
	atns[common.ArgoCDKeyIngressSSLRedirect] = "true"
	atns[common.ArgoCDKeyIngressBackendProtocol] = "HTTP"

	// Override default annotations if specified
	if len(cr.Spec.Server.Ingress.Annotations) > 0 {
		atns = cr.Spec.Server.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	// Add rules
	ingress.Spec.Rules = []extv1beta1.IngressRule{
		{
			Host: getArgoServerHost(cr),
			IngressRuleValue: extv1beta1.IngressRuleValue{
				HTTP: &extv1beta1.HTTPIngressRuleValue{
					Paths: []extv1beta1.HTTPIngressPath{
						{
							Path: getPathOrDefault(cr.Spec.Server.Ingress.Path),
							Backend: extv1beta1.IngressBackend{
								ServiceName: common.NameWithSuffix(cr.ObjectMeta, "server"),
								ServicePort: intstr.FromString("http"),
							},
						},
					},
				},
			},
		},
	}

	// Add default TLS options
	ingress.Spec.TLS = []extv1beta1.IngressTLS{
		{
			Hosts: []string{
				getArgoServerHost(cr),
			},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// Allow override of TLS options if specified
	if len(cr.Spec.Server.Ingress.TLS) > 0 {
		ingress.Spec.TLS = cr.Spec.Server.Ingress.TLS
	}

	ctrl.SetControllerReference(cr, ingress, r.Scheme)
	return r.Client.Create(context.TODO(), ingress)
}

// reconcileArgoServerGRPCIngress will ensure that the ArgoCD Server GRPC Ingress is present.
func (r *ArgoClusterReconciler) reconcileArgoServerGRPCIngress(cr *v1alpha1.ArgoCD) error {
	ingress := resources.NewIngressWithSuffix(cr.ObjectMeta, "grpc")
	if resources.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Server.GRPC.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.Client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Server.GRPC.Ingress.Enabled {
		return nil // Ingress not enabled, move along...
	}

	// Add annotations
	atns := common.DefaultIngressAnnotations()
	atns[common.ArgoCDKeyIngressBackendProtocol] = "GRPC"

	// Override default annotations if specified
	if len(cr.Spec.Server.GRPC.Ingress.Annotations) > 0 {
		atns = cr.Spec.Server.GRPC.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	// Add rules
	ingress.Spec.Rules = []extv1beta1.IngressRule{
		{
			Host: getArgoServerGRPCHost(cr),
			IngressRuleValue: extv1beta1.IngressRuleValue{
				HTTP: &extv1beta1.HTTPIngressRuleValue{
					Paths: []extv1beta1.HTTPIngressPath{
						{
							Path: getPathOrDefault(cr.Spec.Server.GRPC.Ingress.Path),
							Backend: extv1beta1.IngressBackend{
								ServiceName: common.NameWithSuffix(cr.ObjectMeta, "server"),
								ServicePort: intstr.FromString("https"),
							},
						},
					},
				},
			},
		},
	}

	// Add TLS options
	ingress.Spec.TLS = []extv1beta1.IngressTLS{
		{
			Hosts: []string{
				getArgoServerGRPCHost(cr),
			},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// Allow override of TLS options if specified
	if len(cr.Spec.Server.GRPC.Ingress.TLS) > 0 {
		ingress.Spec.TLS = cr.Spec.Server.GRPC.Ingress.TLS
	}

	ctrl.SetControllerReference(cr, ingress, r.Scheme)
	return r.Client.Create(context.TODO(), ingress)
}

// reconcileServerMetricsServiceMonitor will ensure that the ServiceMonitor is present for the ArgoCD Server metrics Service.
func (r *ArgoClusterReconciler) reconcileServerMetricsServiceMonitor(cr *v1alpha1.ArgoCD) error {
	sm := resources.NewServiceMonitorWithSuffix(cr.ObjectMeta, "server-metrics")
	if resources.IsObjectFound(r.Client, cr.Namespace, sm.Name, sm) {
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
			common.ArgoCDKeyName: common.NameWithSuffix(cr.ObjectMeta, "server-metrics"),
		},
	}
	sm.Spec.Endpoints = []monitoringv1.Endpoint{
		{
			Port: common.ArgoCDKeyMetrics,
		},
	}

	ctrl.SetControllerReference(cr, sm, r.Scheme)
	return r.Client.Create(context.TODO(), sm)
}

// reconcileMetricsServiceMonitor will ensure that the ServiceMonitor is present for the ArgoCD metrics Service.
func (r *ArgoClusterReconciler) reconcileMetricsServiceMonitor(cr *v1alpha1.ArgoCD) error {
	sm := resources.NewServiceMonitorWithSuffix(cr.ObjectMeta, common.ArgoCDKeyMetrics)
	if resources.IsObjectFound(r.Client, cr.Namespace, sm.Name, sm) {
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
			common.ArgoCDKeyName: common.NameWithSuffix(cr.ObjectMeta, common.ArgoCDKeyMetrics),
		},
	}
	sm.Spec.Endpoints = []monitoringv1.Endpoint{
		{
			Port: common.ArgoCDKeyMetrics,
		},
	}

	ctrl.SetControllerReference(cr, sm, r.Scheme)
	return r.Client.Create(context.TODO(), sm)
}

// reconcileServerRoute will ensure that the ArgoCD Server Route is present.
func (r *ArgoClusterReconciler) reconcileServerRoute(cr *v1alpha1.ArgoCD) error {
	route := resources.NewRouteWithSuffix(cr.ObjectMeta, "server")
	if resources.IsObjectFound(r.Client, cr.Namespace, route.Name, route) {
		if !cr.Spec.Server.Route.Enabled {
			// Route exists but enabled flag has been set to false, delete the Route
			return r.Client.Delete(context.TODO(), route)
		}
		return nil // Route found, do nothing
	}

	if !cr.Spec.Server.Route.Enabled {
		return nil // Route not enabled, move along...
	}

	// Allow override of the Annotations for the Route.
	if len(cr.Spec.Server.Route.Annotations) > 0 {
		route.Annotations = cr.Spec.Server.Route.Annotations
	}

	// Allow override of the Host for the Route.
	if len(cr.Spec.Server.Host) > 0 {
		route.Spec.Host = cr.Spec.Server.Host // TODO: What additional role needed for this?
	}

	if cr.Spec.Server.Insecure {
		// Disable TLS and rely on the cluster certificate.
		route.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("http"),
		}
		route.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationEdge,
		}
	} else {
		// Server is using TLS configure passthrough.
		route.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("https"),
		}
		route.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationPassthrough,
		}
	}

	// Allow override of TLS options for the Route
	if cr.Spec.Server.Route.TLS != nil {
		route.Spec.TLS = cr.Spec.Server.Route.TLS
	}

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = common.NameWithSuffix(cr.ObjectMeta, "server")

	// Allow override of the WildcardPolicy for the Route
	if cr.Spec.Server.Route.WildcardPolicy != nil && len(*cr.Spec.Server.Route.WildcardPolicy) > 0 {
		route.Spec.WildcardPolicy = *cr.Spec.Server.Route.WildcardPolicy
	}

	ctrl.SetControllerReference(cr, route, r.Scheme)
	return r.Client.Create(context.TODO(), route)
}

// reconcileServerMetricsService will ensure that the Service for the Argo CD server metrics is present.
func (r *ArgoClusterReconciler) reconcileServerMetricsService(cr *v1alpha1.ArgoCD) error {
	svc := resources.NewServiceWithSuffix(cr.ObjectMeta, "server-metrics", "server")
	if resources.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: common.NameWithSuffix(cr.ObjectMeta, "server"),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       8083,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8083),
		},
	}

	ctrl.SetControllerReference(cr, svc, r.Scheme)
	return r.Client.Create(context.TODO(), svc)
}

// reconcileServerService will ensure that the Service is present for the Argo CD server component.
func (r *ArgoClusterReconciler) reconcileServerService(cr *v1alpha1.ArgoCD) error {
	svc := resources.NewServiceWithSuffix(cr.ObjectMeta, "server", "server")
	if resources.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
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
		common.ArgoCDKeyName: common.NameWithSuffix(cr.ObjectMeta, "server"),
	}

	svc.Spec.Type = getArgoServerServiceType(cr)

	ctrl.SetControllerReference(cr, svc, r.Scheme)
	return r.Client.Create(context.TODO(), svc)
}

// reconcileStatusServer will ensure that the Server status is updated for the given ArgoCD.
func (r *ArgoClusterReconciler) reconcileStatusServer(cr *v1alpha1.ArgoCD) error {
	status := "Unknown"

	deploy := resources.NewDeploymentWithSuffix(cr.ObjectMeta, "server", "server")
	if resources.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		status = "Pending"

		if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
			status = "Running"
		}
	}

	if cr.Status.Server != status {
		cr.Status.Server = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}
