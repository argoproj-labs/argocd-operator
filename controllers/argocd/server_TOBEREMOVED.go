package argocd

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// getArgoCDServerReplicas will return the size value for the argocd-server replica count if it
// has been set in argocd CR. Otherwise, nil is returned if the replicas is not set in the argocd CR or
// replicas value is < 0. If Autoscale is enabled, the value for replicas in the argocd CR will be ignored.
func getArgoCDServerReplicas(cr *argoproj.ArgoCD) *int32 {
	if !cr.Spec.Server.Autoscale.Enabled && cr.Spec.Server.Replicas != nil && *cr.Spec.Server.Replicas >= 0 {
		return cr.Spec.Server.Replicas
	}
	return nil
}

// getArgoServerCommand will return the command for the ArgoCD server component.
func getArgoServerCommand(cr *argoproj.ArgoCD, useTLSForRedis bool) []string {
	cmd := make([]string, 0)
	cmd = append(cmd, "argocd-server")

	if getArgoServerInsecure(cr) {
		cmd = append(cmd, "--insecure")
	}

	if isRepoServerTLSVerificationRequested(cr) {
		cmd = append(cmd, "--repo-server-strict-tls")
	}

	cmd = append(cmd, "--staticassets")
	cmd = append(cmd, "/shared/app")

	cmd = append(cmd, "--dex-server")
	cmd = append(cmd, getDexServerAddress(cr))

	if cr.Spec.Repo.IsEnabled() {
		cmd = append(cmd, "--repo-server", getRepoServerAddress(cr))
	} else {
		log.Info("Repo Server is disabled. This would affect the functioning of ArgoCD Server.")
	}

	if cr.Spec.Redis.IsEnabled() {
		cmd = append(cmd, "--redis", getRedisServerAddress(cr))
	} else {
		log.Info("Redis is Disabled. Skipping adding Redis configuration to ArgoCD Server.")
	}

	if useTLSForRedis {
		cmd = append(cmd, "--redis-use-tls")
		if isRedisTLSVerificationDisabled(cr) {
			cmd = append(cmd, "--redis-insecure-skip-tls-verify")
		} else {
			cmd = append(cmd, "--redis-ca-certificate", "/app/config/server/tls/redis/tls.crt")
		}
	}

	cmd = append(cmd, "--loglevel")
	cmd = append(cmd, getLogLevel(cr.Spec.Server.LogLevel))

	cmd = append(cmd, "--logformat")
	cmd = append(cmd, getLogFormat(cr.Spec.Server.LogFormat))

	extraArgs := cr.Spec.Server.ExtraCommandArgs
	err := isMergable(extraArgs, cmd)
	if err != nil {
		return cmd
	}
	if cr.Spec.SourceNamespaces != nil && len(cr.Spec.SourceNamespaces) > 0 {
		cmd = append(cmd, "--application-namespaces", fmt.Sprint(strings.Join(cr.Spec.SourceNamespaces, ",")))
	}

	cmd = append(cmd, extraArgs...)
	return cmd
}

// reconcileServerDeployment will ensure the Deployment resource is present for the ArgoCD Server component.
func (r *ReconcileArgoCD) reconcileServerDeployment(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	deploy := newDeploymentWithSuffix("server", "server", cr)
	serverEnv := cr.Spec.Server.Env
	serverEnv = argoutil.EnvMerge(serverEnv, proxyEnvVars(), false)
	AddSeccompProfileForOpenShift(r.Client, &deploy.Spec.Template.Spec)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command:         getArgoServerCommand(cr, useTLSForRedis),
		Image:           getArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Env:             serverEnv,
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
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
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8080),
				},
			},
			InitialDelaySeconds: 3,
			PeriodSeconds:       30,
		},
		Resources: getArgoServerResources(cr),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			RunAsNonRoot: boolPtr(true),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "ssh-known-hosts",
				MountPath: "/app/config/ssh",
			}, {
				Name:      "tls-certs",
				MountPath: "/app/config/tls",
			},
			{
				Name:      "argocd-repo-server-tls",
				MountPath: "/app/config/server/tls",
			},
			{
				Name:      common.ArgoCDRedisServerTLSSecretName,
				MountPath: "/app/config/server/tls/redis",
			},
		},
	}}
	deploy.Spec.Template.Spec.ServiceAccountName = fmt.Sprintf("%s-%s", cr.Name, "argocd-server")
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
		},
		{
			Name: "tls-certs",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDTLSCertsConfigMapName,
					},
				},
			},
		},
		{
			Name: "argocd-repo-server-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRepoServerTLSSecretName,
					Optional:   boolPtr(true),
				},
			},
		},
		{
			Name: common.ArgoCDRedisServerTLSSecretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRedisServerTLSSecretName,
					Optional:   boolPtr(true),
				},
			},
		},
	}

	if replicas := getArgoCDServerReplicas(cr); replicas != nil {
		deploy.Spec.Replicas = replicas
	}

	existing := newDeploymentWithSuffix("server", "server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		if !cr.Spec.Server.IsEnabled() {
			log.Info("Existing ArgoCD Server found but should be disabled. Deleting ArgoCD Server")
			// Delete existing deployment for ArgoCD Server, if any ..
			return r.Client.Delete(context.TODO(), existing)
		}
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getArgoContainerImage(cr)
		changed := false
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}
		updateNodePlacement(existing, deploy, &changed)
		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			deploy.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			changed = true
		}
		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Command,
			deploy.Spec.Template.Spec.Containers[0].Command) {
			existing.Spec.Template.Spec.Containers[0].Command = deploy.Spec.Template.Spec.Containers[0].Command
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Volumes, existing.Spec.Template.Spec.Volumes) {
			existing.Spec.Template.Spec.Volumes = deploy.Spec.Template.Spec.Volumes
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].VolumeMounts,
			existing.Spec.Template.Spec.Containers[0].VolumeMounts) {
			existing.Spec.Template.Spec.Containers[0].VolumeMounts = deploy.Spec.Template.Spec.Containers[0].VolumeMounts
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Resources,
			existing.Spec.Template.Spec.Containers[0].Resources) {
			existing.Spec.Template.Spec.Containers[0].Resources = deploy.Spec.Template.Spec.Containers[0].Resources
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Replicas, existing.Spec.Replicas) {
			if !cr.Spec.Server.Autoscale.Enabled {
				existing.Spec.Replicas = deploy.Spec.Replicas
				changed = true
			}
		}
		if changed {
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if !cr.Spec.Server.IsEnabled() {
		log.Info("ArgoCD Server disabled. Skipping starting argocd server.")
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileServerHPA will ensure that the HorizontalPodAutoscaler is present for the Argo CD Server component, and reconcile any detected changes.
func (r *ReconcileArgoCD) reconcileServerHPA(cr *argoproj.ArgoCD) error {

	defaultHPA := newHorizontalPodAutoscalerWithSuffix("server", cr)
	defaultHPA.Spec = autoscaling.HorizontalPodAutoscalerSpec{
		MaxReplicas:                    maxReplicas,
		MinReplicas:                    &minReplicas,
		TargetCPUUtilizationPercentage: &tcup,
		ScaleTargetRef: autoscaling.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       nameWithSuffix("server", cr),
		},
	}

	existingHPA := newHorizontalPodAutoscalerWithSuffix("server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existingHPA.Name, existingHPA) {
		if !cr.Spec.Server.Autoscale.Enabled {
			return r.Client.Delete(context.TODO(), existingHPA) // HorizontalPodAutoscaler found but globally disabled, delete it.
		}

		changed := false
		// HorizontalPodAutoscaler found, reconcile if necessary changes detected
		if cr.Spec.Server.Autoscale.HPA != nil {
			if !reflect.DeepEqual(existingHPA.Spec, cr.Spec.Server.Autoscale.HPA) {
				existingHPA.Spec = *cr.Spec.Server.Autoscale.HPA
				changed = true
			}
		}

		if changed {
			return r.Client.Update(context.TODO(), existingHPA)
		}

		// HorizontalPodAutoscaler found, no changes detected
		return nil
	}

	if !cr.Spec.Server.Autoscale.Enabled {
		return nil // AutoScale not enabled, move along...
	}

	// AutoScale enabled, no existing HPA found, create
	if cr.Spec.Server.Autoscale.HPA != nil {
		defaultHPA.Spec = *cr.Spec.Server.Autoscale.HPA
	}

	return r.Client.Create(context.TODO(), defaultHPA)
}

// reconcileArgoServerIngress will ensure that the ArgoCD Server Ingress is present.
func (r *ReconcileArgoCD) reconcileArgoServerIngress(cr *argoproj.ArgoCD) error {
	ingress := newIngressWithSuffix("server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Server.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.Client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Server.Ingress.Enabled {
		return nil // Ingress not enabled, move along...
	}

	// Add default annotations
	atns := make(map[string]string)
	atns[common.ArgoCDKeyIngressSSLRedirect] = "true"
	atns[common.ArgoCDKeyIngressBackendProtocol] = "HTTP"

	// Override default annotations if specified
	if len(cr.Spec.Server.Ingress.Annotations) > 0 {
		atns = cr.Spec.Server.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	ingress.Spec.IngressClassName = cr.Spec.Server.Ingress.IngressClassName

	pathType := networkingv1.PathTypeImplementationSpecific
	// Add rules
	ingress.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: getArgoServerHost(cr),
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path: getPathOrDefault(cr.Spec.Server.Ingress.Path),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: nameWithSuffix("server", cr),
									Port: networkingv1.ServiceBackendPort{
										Name: "http",
									},
								},
							},
							PathType: &pathType,
						},
					},
				},
			},
		},
	}

	// Add default TLS options
	ingress.Spec.TLS = []networkingv1.IngressTLS{
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

	if err := controllerutil.SetControllerReference(cr, ingress, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ingress)
}

// reconcileArgoServerGRPCIngress will ensure that the ArgoCD Server GRPC Ingress is present.
func (r *ReconcileArgoCD) reconcileArgoServerGRPCIngress(cr *argoproj.ArgoCD) error {
	ingress := newIngressWithSuffix("grpc", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Server.GRPC.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.Client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Server.GRPC.Ingress.Enabled {
		return nil // Ingress not enabled, move along...
	}

	// Add default annotations
	atns := make(map[string]string)
	atns[common.ArgoCDKeyIngressBackendProtocol] = "GRPC"

	// Override default annotations if specified
	if len(cr.Spec.Server.GRPC.Ingress.Annotations) > 0 {
		atns = cr.Spec.Server.GRPC.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	ingress.Spec.IngressClassName = cr.Spec.Server.GRPC.Ingress.IngressClassName

	pathType := networkingv1.PathTypeImplementationSpecific
	// Add rules
	ingress.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: getArgoServerGRPCHost(cr),
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path: getPathOrDefault(cr.Spec.Server.GRPC.Ingress.Path),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: nameWithSuffix("server", cr),
									Port: networkingv1.ServiceBackendPort{
										Name: "https",
									},
								},
							},
							PathType: &pathType,
						},
					},
				},
			},
		},
	}

	// Add TLS options
	ingress.Spec.TLS = []networkingv1.IngressTLS{
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

	if err := controllerutil.SetControllerReference(cr, ingress, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ingress)
}

func policyRuleForServer() []v1.PolicyRule {
	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"*",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"get",
				"patch",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"secrets",
				"configmaps",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"patch",
				"delete",
			},
		}, {
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"applications",
				"applicationsets",
				"appprojects",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"delete",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"create",
				"list",
			},
		},
		{
			APIGroups: []string{
				"batch",
			},
			Resources: []string{
				"jobs",
			},
			Verbs: []string{
				"create",
			},
		},
	}
}

func policyRuleForServerClusterRole() []v1.PolicyRule {
	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"*",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"get",
				"delete",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"applications",
			},
			Verbs: []string{
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"list",
			},
		},
		{
			APIGroups: []string{
				"batch",
			},
			Resources: []string{
				"jobs",
			},
			Verbs: []string{
				"create",
			},
		},
	}
}

// reconcileServerRoute will ensure that the ArgoCD Server Route is present.
func (r *ReconcileArgoCD) reconcileServerRoute(cr *argoproj.ArgoCD) error {

	route := newRouteWithSuffix("server", cr)
	found := argoutil.IsObjectFound(r.Client, cr.Namespace, route.Name, route)
	if found {
		if !cr.Spec.Server.Route.Enabled {
			// Route exists but enabled flag has been set to false, delete the Route
			return r.Client.Delete(context.TODO(), route)
		}
	}

	if !cr.Spec.Server.Route.Enabled {
		return nil // Route not enabled, move along...
	}

	// Allow override of the Annotations for the Route.
	if len(cr.Spec.Server.Route.Annotations) > 0 {
		route.Annotations = cr.Spec.Server.Route.Annotations
	}

	// Allow override of the Labels for the Route.
	if len(cr.Spec.Server.Route.Labels) > 0 {
		labels := route.Labels
		for key, val := range cr.Spec.Server.Route.Labels {
			labels[key] = val
		}
		route.Labels = labels
	}

	// Allow override of the Host for the Route.
	if len(cr.Spec.Server.Host) > 0 {
		route.Spec.Host = cr.Spec.Server.Host // TODO: What additional role needed for this?
	}

	hostname, err := shortenHostname(route.Spec.Host)
	if err != nil {
		return err
	}

	route.Spec.Host = hostname

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
	route.Spec.To.Name = nameWithSuffix("server", cr)

	// Allow override of the WildcardPolicy for the Route
	if cr.Spec.Server.Route.WildcardPolicy != nil && len(*cr.Spec.Server.Route.WildcardPolicy) > 0 {
		route.Spec.WildcardPolicy = *cr.Spec.Server.Route.WildcardPolicy
	}

	if err := controllerutil.SetControllerReference(cr, route, r.Scheme); err != nil {
		return err
	}
	if !found {
		return r.Client.Create(context.TODO(), route)
	}
	return r.Client.Update(context.TODO(), route)
}

// getArgoServerServiceType will return the server Service type for the ArgoCD.
func getArgoServerServiceType(cr *argoproj.ArgoCD) corev1.ServiceType {
	if len(cr.Spec.Server.Service.Type) > 0 {
		return cr.Spec.Server.Service.Type
	}
	return corev1.ServiceTypeClusterIP
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

// reconcileStatusServer will ensure that the Server status is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusServer(cr *argoproj.ArgoCD) error {
	status := "Unknown"

	deploy := newDeploymentWithSuffix("server", "server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		status = "Pending"

		// TODO: Refactor these checks.
		if deploy.Spec.Replicas != nil {
			if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
				status = "Running"
			} else if deploy.Status.Conditions != nil {
				for _, condition := range deploy.Status.Conditions {
					if condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue {
						// Deployment has failed
						status = "Failed"
						break
					}
				}
			}
		}
	}

	if cr.Status.Server != status {
		cr.Status.Server = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}

// getArgoServerGRPCHost will return the GRPC host for the given ArgoCD.
func getArgoServerGRPCHost(cr *argoproj.ArgoCD) string {
	host := nameWithSuffix("grpc", cr)
	if len(cr.Spec.Server.GRPC.Host) > 0 {
		host = cr.Spec.Server.GRPC.Host
	}
	return host
}

// getArgoServerHost will return the host for the given ArgoCD.
func getArgoServerHost(cr *argoproj.ArgoCD) string {
	host := cr.Name
	if len(cr.Spec.Server.Host) > 0 {
		host = cr.Spec.Server.Host
	}
	return host
}

// getArgoServerResources will return the ResourceRequirements for the Argo CD server container.
func getArgoServerResources(cr *argoproj.ArgoCD) corev1.ResourceRequirements {
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

// getArgoServerInsecure returns the insecure value for the ArgoCD Server component.
func getArgoServerInsecure(cr *argoproj.ArgoCD) bool {
	return cr.Spec.Server.Insecure
}

var (
	maxReplicas int32 = 3
	minReplicas int32 = 1
	tcup        int32 = 50
)

func newHorizontalPodAutoscaler(cr *argoproj.ArgoCD) *autoscaling.HorizontalPodAutoscaler {
	return &autoscaling.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
	}
}

func newHorizontalPodAutoscalerWithName(name string, cr *argoproj.ArgoCD) *autoscaling.HorizontalPodAutoscaler {
	hpa := newHorizontalPodAutoscaler(cr)
	hpa.ObjectMeta.Name = name

	lbls := hpa.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	hpa.ObjectMeta.Labels = lbls

	return hpa
}

func newHorizontalPodAutoscalerWithSuffix(suffix string, cr *argoproj.ArgoCD) *autoscaling.HorizontalPodAutoscaler {
	return newHorizontalPodAutoscalerWithName(nameWithSuffix(suffix, cr), cr)
}

// reconcileAutoscalers will ensure that all HorizontalPodAutoscalers are present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileAutoscalers(cr *argoproj.ArgoCD) error {
	if err := r.reconcileServerHPA(cr); err != nil {
		return err
	}
	return nil
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
			Port:       common.ServerMetricsPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ServerMetricsPort),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}
