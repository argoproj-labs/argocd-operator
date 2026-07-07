package argocd

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// UseCommitServer automagially decides if the commit server should be deployed based on acual use-cases using it.
// Currently, it is only used by the Source Hydrator.
func UseCommitServer(cr *argoproj.ArgoCD) bool {
	return cr.Spec.SourceHydrator.IsEnabled()
}

func getCommitServerCommand(cr *argoproj.ArgoCD) []string {
	cmd := []string{
		"/usr/local/bin/argocd-commit-server",
	}

	if cr.Spec.CommitServer.LogLevel != "" {
		cmd = append(cmd, "--loglevel", getLogLevel(cr.Spec.CommitServer.LogLevel))
	}

	if cr.Spec.CommitServer.LogFormat != "" {
		cmd = append(cmd, "--logformat", getLogFormat(cr.Spec.CommitServer.LogFormat))
	}

	return cmd
}

func policyRuleForCommitServer() []v1.PolicyRule {
	return []v1.PolicyRule{}
}

func (r *ReconcileArgoCD) reconcileCommitServerDeployment(cr *argoproj.ArgoCD) error {
	deploy := newDeploymentWithSuffix("commit-server", "commit-server", cr)
	AddSeccompProfileForOpenShift(r.Client, &deploy.Spec.Template.Spec)

	commitServerVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "ssh-known-hosts",
			MountPath: "/app/config/ssh",
		}, {
			Name:      "tls-certs",
			MountPath: "/app/config/tls",
		},
		{
			Name:      "gpg-keys",
			MountPath: "/app/config/gpg/source",
		},
		{
			Name:      "gpg-keyring",
			MountPath: "/app/config/gpg/keys",
		},
		{
			Name:      "tmp",
			MountPath: "/tmp",
		},
	}

	env := argoutil.EnvMerge(proxyEnvVars(), cr.Spec.CommitServer.Env, true)

	resources := cr.Spec.CommitServer.Resources
	if resources == nil {
		resources = &corev1.ResourceRequirements{}
	}

	if cr.Spec.CommitServer.Annotations != nil {
		for key, value := range cr.Spec.CommitServer.Annotations {
			deploy.Spec.Template.Annotations[key] = value
		}
	}

	if cr.Spec.CommitServer.Labels != nil {
		for key, value := range cr.Spec.CommitServer.Labels {
			deploy.Spec.Template.Labels[key] = value
		}
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Name: "argocd-commit-server",
		Command:         getCommitServerCommand(cr),
		Image:           getArgoContainerImage(cr),
		ImagePullPolicy: argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy),
		Env:             env,
		Resources:       *resources,
		SecurityContext: argoutil.DefaultSecurityContext(),
		VolumeMounts:    commitServerVolumeMounts,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultCommitServerPort,
			}, {
				ContainerPort: common.ArgoCDDefaultCommitServerMetricsPort,
			},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz?full=true",
					Port: intstr.FromInt32(common.ArgoCDDefaultCommitServerMetricsPort),
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       30,
			TimeoutSeconds:      5,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt32(common.ArgoCDDefaultCommitServerMetricsPort),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
	}}
	deploy.Spec.Template.Spec.ServiceAccountName = fmt.Sprintf("%s-%s", cr.Name, "argocd-commit-server")

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
			Name: "gpg-keys",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDGPGKeysConfigMapName,
					},
				},
			},
		},
		{
			Name: "gpg-keyring",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "tmp",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	if cr.Spec.CommitServer.InitContainers != nil {
		deploy.Spec.Template.Spec.InitContainers = append(deploy.Spec.Template.Spec.InitContainers, cr.Spec.CommitServer.InitContainers...)
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}

	shouldExist := UseCommitServer(cr)
	existing := newDeploymentWithSuffix("commit-server", "commit-server", cr)

	deplExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing)
	if err != nil {
		return err
	}
	if deplExists {
		if !shouldExist {
			argoutil.LogResourceDeletion(log, existing, "disabled")
			return r.Delete(context.TODO(), existing)
		}

		var changes []string
		// Add Kubernetes-specific labels/annotations from the live object in the source to preserve metadata.
		addKubernetesData(deploy.Spec.Template.Labels, existing.Spec.Template.Labels)
		addKubernetesData(deploy.Spec.Template.Annotations, existing.Spec.Template.Annotations)

		if !reflect.DeepEqual(deploy.Spec.Template.Annotations, existing.Spec.Template.Annotations) {
			existing.Spec.Template.Annotations = deploy.Spec.Template.Annotations
			changes = append(changes, "annotations")
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Labels, existing.Spec.Template.Labels) {
			existing.Spec.Template.Labels = deploy.Spec.Template.Labels
			changes = append(changes, "labels")
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.InitContainers, existing.Spec.Template.Spec.InitContainers) {
			existing.Spec.Template.Spec.InitContainers = deploy.Spec.Template.Spec.InitContainers
			changes = append(changes, "init containers")
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Env, existing.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			changes = append(changes, "container env")
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Resources, existing.Spec.Template.Spec.Containers[0].Resources) {
			existing.Spec.Template.Spec.Containers[0].Resources = deploy.Spec.Template.Spec.Containers[0].Resources
			changes = append(changes, "container resources")
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Command, existing.Spec.Template.Spec.Containers[0].Command) {
			existing.Spec.Template.Spec.Containers[0].Command = deploy.Spec.Template.Spec.Containers[0].Command
			changes = append(changes, "container command")
		}

		if len(changes) > 0 {
			argoutil.LogResourceUpdate(log, existing, "updating", strings.Join(changes, ", "))
			return r.Update(context.TODO(), existing)
		}
		return nil
	}

	if shouldExist {
		argoutil.LogResourceCreation(log, deploy, "enabled")
		return r.Create(context.TODO(), deploy)
	}

	return nil
}

func (r *ReconcileArgoCD) reconcileArgoCDCommitServerNetworkPolicy(cr *argoproj.ArgoCD) error {
	desired := returnNetworkPolicyHeaders(cr, ArgoCDCommitServerNetworkPolicy)
	desired.Spec = networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/name": nameWithSuffix("commit-server", cr),
			},
		},
		PolicyTypes: []networkingv1.PolicyType{
			networkingv1.PolicyTypeIngress,
		},
		Ingress: []networkingv1.NetworkPolicyIngressRule{
			{
				From: []networkingv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app.kubernetes.io/name": nameWithSuffix("application-controller", cr),
							},
						},
					},
				},
				Ports: []networkingv1.NetworkPolicyPort{
					{
						Protocol: TCPProtocol,
						Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: common.ArgoCDDefaultCommitServerPort},
					},
				},
			},
			{
				From: []networkingv1.NetworkPolicyPeer{
					{
						NamespaceSelector: &metav1.LabelSelector{},
					},
				},
				Ports: []networkingv1.NetworkPolicyPort{
					{
						Protocol: TCPProtocol,
						Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: common.ArgoCDDefaultCommitServerMetricsPort},
					},
				},
			},
		},
	}

	shouldExist := UseCommitServer(cr)

	existing := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      desired.Name,
			Namespace: cr.Namespace,
		},
	}

	if err := controllerutil.SetControllerReference(cr, desired, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference on argocd commit server network policy")
		return fmt.Errorf("failed to set controller reference on argocd commit server network policy. error: %w", err)
	}

	npExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing)
	if err != nil {
		return err
	}

	if npExists {
		if !shouldExist {
			argoutil.LogResourceDeletion(log, existing, "disabled")
			return r.Delete(context.TODO(), existing)
		}

		var changes []string
		if !reflect.DeepEqual(existing.Spec.PodSelector, desired.Spec.PodSelector) {
			existing.Spec.PodSelector = desired.Spec.PodSelector
			changes = append(changes, "pod selector")
		}
		if !reflect.DeepEqual(existing.Spec.PolicyTypes, desired.Spec.PolicyTypes) {
			existing.Spec.PolicyTypes = desired.Spec.PolicyTypes
			changes = append(changes, "policy types")
		}
		if !reflect.DeepEqual(existing.Spec.Ingress, desired.Spec.Ingress) {
			existing.Spec.Ingress = desired.Spec.Ingress
			changes = append(changes, "ingress rules")
		}

		if len(changes) > 0 {
			argoutil.LogResourceUpdate(log, existing, "updating", strings.Join(changes, ", "))
			if err := r.Update(context.TODO(), existing); err != nil {
				log.Error(err, "Failed to update %s network policy in namespace %s", existing.Name, cr.Namespace)
				return fmt.Errorf("failed to update %s network policy in namespace %s. error: %w", existing.Name, cr.Namespace, err)
			}
		}
		return nil
	}

	if shouldExist {
		argoutil.LogResourceCreation(log, desired)
		if err := r.Create(context.TODO(), desired); err != nil {
			log.Error(err, "Failed to create %s network policy in namespace %s", existing.Name, cr.Namespace)
			return fmt.Errorf("failed to create %s network policy in namespace %s. error: %w", existing.Name, cr.Namespace, err)
		}
	}

	return nil
}

func (r *ReconcileArgoCD) reconcileCommitServerService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("commit-server", "commit-server", cr)

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "server",
			Port:       common.ArgoCDDefaultCommitServerPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt32(common.ArgoCDDefaultCommitServerPort),
		}, {
			Name:       "metrics",
			Port:       common.ArgoCDDefaultCommitServerMetricsPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt32(common.ArgoCDDefaultCommitServerMetricsPort),
		},
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("commit-server", cr),
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}

	existingSVC := &corev1.Service{}
	svcExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, existingSVC)
	if err != nil {
		return err
	}

	shouldExist := UseCommitServer(cr)

	if svcExists {
		if !shouldExist {
			argoutil.LogResourceDeletion(log, svc, "disabled")
			return r.Delete(context.TODO(), svc)
		}
		
		var changes []string
		if !reflect.DeepEqual(svc.Spec.Type, existingSVC.Spec.Type) {
			existingSVC.Spec.Type = svc.Spec.Type
			changes = append(changes, "service type")
		}
		if len(changes) > 0 {
			argoutil.LogResourceUpdate(log, existingSVC, "updating", strings.Join(changes, ", "))
			return r.Update(context.TODO(), existingSVC)
		}
		return nil
	}

	if shouldExist {
		argoutil.LogResourceCreation(log, svc, "enabled")
		return r.Create(context.TODO(), svc)
	}

	return nil
}
