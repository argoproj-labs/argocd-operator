package argocd

import (
	"context"
	"fmt"
	"os"
	"reflect"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	ApplicationSetGitlabSCMTlsCertPath = "/app/tls/scm/cert"
)

// getArgoApplicationSetCommand will return the command for the ArgoCD ApplicationSet component.
func getArgoApplicationSetCommand(cr *argoproj.ArgoCD) []string {
	cmd := make([]string, 0)

	cmd = append(cmd, "entrypoint.sh")
	cmd = append(cmd, "argocd-applicationset-controller")

	if cr.Spec.Repo.IsEnabled() {
		cmd = append(cmd, "--argocd-repo-server", getRepoServerAddress(cr))
	} else {
		log.Info("Repo Server is disabled. This would affect the functioning of ApplicationSet Controller.")
	}

	cmd = append(cmd, "--loglevel")
	cmd = append(cmd, getLogLevel(cr.Spec.ApplicationSet.LogLevel))

	if cr.Spec.ApplicationSet.SCMRootCAConfigMap != "" {
		cmd = append(cmd, "--scm-root-ca-path")
		cmd = append(cmd, ApplicationSetGitlabSCMTlsCertPath)
	}

	// ApplicationSet command arguments provided by the user
	extraArgs := cr.Spec.ApplicationSet.ExtraCommandArgs
	err := isMergable(extraArgs, cmd)
	if err != nil {
		return cmd
	}

	cmd = append(cmd, extraArgs...)

	return cmd
}

func (r *ReconcileArgoCD) reconcileApplicationSetController(cr *argoproj.ArgoCD) error {

	log.Info("reconciling applicationset serviceaccounts")
	sa, err := r.reconcileApplicationSetServiceAccount(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling applicationset roles")
	role, err := r.reconcileApplicationSetRole(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling applicationset role bindings")
	if err := r.reconcileApplicationSetRoleBinding(cr, role, sa); err != nil {
		return err
	}

	log.Info("reconciling applicationset deployments")
	if err := r.reconcileApplicationSetDeployment(cr, sa); err != nil {
		return err
	}

	log.Info("reconciling applicationset service")
	if err := r.reconcileApplicationSetService(cr); err != nil {
		return err
	}

	return nil
}

// reconcileApplicationControllerDeployment will ensure the Deployment resource is present for the ArgoCD Application Controller component.
func (r *ReconcileArgoCD) reconcileApplicationSetDeployment(cr *argoproj.ArgoCD, sa *corev1.ServiceAccount) error {
	deploy := newDeploymentWithSuffix("applicationset-controller", "controller", cr)

	setAppSetLabels(&deploy.ObjectMeta)

	podSpec := &deploy.Spec.Template.Spec

	// sa would be nil when spec.applicationset.enabled = false
	if sa != nil {
		podSpec.ServiceAccountName = sa.ObjectMeta.Name
	}
	podSpec.Volumes = []corev1.Volume{
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
	addSCMGitlabVolumeMount := false
	if scmRootCAConfigMapName := getSCMRootCAConfigMapName(cr); scmRootCAConfigMapName != "" {
		cm := newConfigMapWithName(scmRootCAConfigMapName, cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, cr.Spec.ApplicationSet.SCMRootCAConfigMap, cm) {
			addSCMGitlabVolumeMount = true
			podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
				Name: "appset-gitlab-scm-tls-cert",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: common.ArgoCDAppSetGitlabSCMTLSCertsConfigMapName,
						},
					},
				},
			})
		}
	}

	podSpec.Containers = []corev1.Container{
		applicationSetContainer(cr, addSCMGitlabVolumeMount),
	}
	AddSeccompProfileForOpenShift(r.Client, podSpec)

	if existing := newDeploymentWithSuffix("applicationset-controller", "controller", cr); argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {

		if cr.Spec.ApplicationSet != nil && !cr.Spec.ApplicationSet.IsEnabled() {
			err := r.Client.Delete(context.TODO(), existing)
			return err
		}

		existingSpec := existing.Spec.Template.Spec

		deploymentsDifferent := !reflect.DeepEqual(existingSpec.Containers[0], podSpec.Containers) ||
			!reflect.DeepEqual(existingSpec.Volumes, podSpec.Volumes) ||
			existingSpec.ServiceAccountName != podSpec.ServiceAccountName ||
			!reflect.DeepEqual(existing.Labels, deploy.Labels) ||
			!reflect.DeepEqual(existing.Spec.Template.Labels, deploy.Spec.Template.Labels) ||
			!reflect.DeepEqual(existing.Spec.Selector, deploy.Spec.Selector) ||
			!reflect.DeepEqual(existing.Spec.Template.Spec.NodeSelector, deploy.Spec.Template.Spec.NodeSelector) ||
			!reflect.DeepEqual(existing.Spec.Template.Spec.Tolerations, deploy.Spec.Template.Spec.Tolerations)

		// If the Deployment already exists, make sure the values we care about are up-to-date
		if deploymentsDifferent {
			existing.Spec.Template.Spec.Containers = podSpec.Containers
			existing.Spec.Template.Spec.Volumes = podSpec.Volumes
			existing.Spec.Template.Spec.ServiceAccountName = podSpec.ServiceAccountName
			existing.Labels = deploy.Labels
			existing.Spec.Template.Labels = deploy.Spec.Template.Labels
			existing.Spec.Selector = deploy.Spec.Selector
			existing.Spec.Template.Spec.NodeSelector = deploy.Spec.Template.Spec.NodeSelector
			existing.Spec.Template.Spec.Tolerations = deploy.Spec.Template.Spec.Tolerations
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if !cr.Spec.ApplicationSet.IsEnabled() {
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)

}

func applicationSetContainer(cr *argoproj.ArgoCD, addSCMGitlabVolumeMount bool) corev1.Container {
	// Global proxy env vars go first
	appSetEnv := []corev1.EnvVar{{
		Name: "NAMESPACE",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
	}}

	// Merge ApplicationSet env vars provided by the user
	// User should be able to override the default NAMESPACE environmental variable
	appSetEnv = argoutil.EnvMerge(cr.Spec.ApplicationSet.Env, appSetEnv, true)
	// Environment specified in the CR take precedence over everything else
	appSetEnv = argoutil.EnvMerge(appSetEnv, proxyEnvVars(), false)

	container := corev1.Container{
		Command:         getArgoApplicationSetCommand(cr),
		Env:             appSetEnv,
		Image:           getApplicationSetContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "argocd-applicationset-controller",
		Resources:       getApplicationSetResources(cr),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "ssh-known-hosts",
				MountPath: "/app/config/ssh",
			},
			{
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
		},
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 7000,
				Name:          "webhook",
			},
			{
				ContainerPort: 8080,
				Name:          "metrics",
			},
		},
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			AllowPrivilegeEscalation: boolPtr(false),
			ReadOnlyRootFilesystem:   boolPtr(true),
			RunAsNonRoot:             boolPtr(true),
		},
	}
	if addSCMGitlabVolumeMount {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "appset-gitlab-scm-tls-cert",
			MountPath: ApplicationSetGitlabSCMTlsCertPath,
		})
	}
	return container
}

func (r *ReconcileArgoCD) reconcileApplicationSetServiceAccount(cr *argoproj.ArgoCD) (*corev1.ServiceAccount, error) {

	sa := newServiceAccountWithName("applicationset-controller", cr)
	setAppSetLabels(&sa.ObjectMeta)

	exists := true
	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		exists = false
	}

	if exists {
		if cr.Spec.ApplicationSet != nil && !cr.Spec.ApplicationSet.IsEnabled() {
			err := r.Client.Delete(context.TODO(), sa)
			return nil, err
		}
		return sa, nil
	}

	if err := controllerutil.SetControllerReference(cr, sa, r.Scheme); err != nil {
		return nil, err
	}

	if cr.Spec.ApplicationSet != nil && !cr.Spec.ApplicationSet.IsEnabled() {
		return nil, nil
	}

	err := r.Client.Create(context.TODO(), sa)
	if err != nil {
		return nil, err
	}

	return sa, err
}

func (r *ReconcileArgoCD) reconcileApplicationSetRole(cr *argoproj.ArgoCD) (*rbacv1.Role, error) {

	policyRules := []rbacv1.PolicyRule{

		// ApplicationSet
		{
			APIGroups: []string{"argoproj.io"},
			Resources: []string{
				"applications",
				"applicationsets",
				"appprojects",
				"applicationsets/finalizers",
			},
			Verbs: []string{
				"create",
				"delete",
				"get",
				"list",
				"patch",
				"update",
				"watch",
			},
		},
		// ApplicationSet Status
		{
			APIGroups: []string{"argoproj.io"},
			Resources: []string{
				"applicationsets/status",
			},
			Verbs: []string{
				"get",
				"patch",
				"update",
			},
		},

		// Events
		{
			APIGroups: []string{""},
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"create",
				"delete",
				"get",
				"list",
				"patch",
				"update",
				"watch",
			},
		},

		// Read Secrets/ConfigMaps
		{
			APIGroups: []string{""},
			Resources: []string{
				"secrets",
				"configmaps",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},

		// Read Deployments
		{
			APIGroups: []string{"apps", "extensions"},
			Resources: []string{
				"deployments",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
	}

	role := newRole("applicationset-controller", policyRules, cr)
	setAppSetLabels(&role.ObjectMeta)

	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: cr.Namespace}, role)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the role for the service account associated with %s : %s", role.Name, err)
		}
		if apierrors.IsNotFound(err) && cr.Spec.ApplicationSet != nil && !cr.Spec.ApplicationSet.IsEnabled() {
			return nil, nil
		}
		if err = controllerutil.SetControllerReference(cr, role, r.Scheme); err != nil {
			return nil, err
		}
		return role, r.Client.Create(context.TODO(), role)
	}
	if cr.Spec.ApplicationSet != nil && !cr.Spec.ApplicationSet.IsEnabled() {
		return nil, r.Client.Delete(context.TODO(), role)
	}

	role.Rules = policyRules
	if err = controllerutil.SetControllerReference(cr, role, r.Scheme); err != nil {
		return nil, err
	}
	return role, r.Client.Update(context.TODO(), role)
}

func (r *ReconcileArgoCD) reconcileApplicationSetRoleBinding(cr *argoproj.ArgoCD, role *rbacv1.Role, sa *corev1.ServiceAccount) error {

	name := "applicationset-controller"

	// get expected name
	roleBinding := newRoleBindingWithname(name, cr)

	// fetch existing rolebinding by name
	roleBindingExists := true
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: cr.Namespace}, roleBinding); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s : %s", name, err)
		}
		if apierrors.IsNotFound(err) && cr.Spec.ApplicationSet != nil && !cr.Spec.ApplicationSet.IsEnabled() {
			return nil
		}
		roleBindingExists = false
	}

	if cr.Spec.ApplicationSet != nil && !cr.Spec.ApplicationSet.IsEnabled() {
		return r.Client.Delete(context.TODO(), roleBinding)
	}

	setAppSetLabels(&roleBinding.ObjectMeta)

	roleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     role.Name,
	}

	roleBinding.Subjects = []rbacv1.Subject{
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
	}

	if err := controllerutil.SetControllerReference(cr, roleBinding, r.Scheme); err != nil {
		return err
	}

	if roleBindingExists {
		return r.Client.Update(context.TODO(), roleBinding)
	}

	return r.Client.Create(context.TODO(), roleBinding)
}

func getApplicationSetContainerImage(cr *argoproj.ArgoCD) string {
	defaultImg, defaultTag := false, false

	img := ""
	tag := ""

	// First pull from spec, if it exists
	if cr.Spec.ApplicationSet != nil {
		img = cr.Spec.ApplicationSet.Image
		tag = cr.Spec.ApplicationSet.Version
	}

	// If spec is empty, use the defaults
	if img == "" {
		img = common.ArgoCDDefaultArgoImage
		defaultImg = true
	}
	if tag == "" {
		tag = common.ArgoCDDefaultArgoVersion
		defaultTag = true
	}

	// If an env var is specified then use that, but don't override the spec values (if they are present)
	if e := os.Getenv(common.ArgoCDImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}
	return argoutil.CombineImageTag(img, tag)
}

// getApplicationSetResources will return the ResourceRequirements for the Application Sets container.
func getApplicationSetResources(cr *argoproj.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.ApplicationSet.Resources != nil {
		resources = *cr.Spec.ApplicationSet.Resources
	}

	return resources
}

func setAppSetLabels(obj *metav1.ObjectMeta) {
	obj.Labels["app.kubernetes.io/name"] = "argocd-applicationset-controller"
	obj.Labels["app.kubernetes.io/part-of"] = "argocd-applicationset"
	obj.Labels["app.kubernetes.io/component"] = "controller"
}

// reconcileApplicationSetService will ensure that the Service is present for the ApplicationSet webhook and metrics component.
func (r *ReconcileArgoCD) reconcileApplicationSetService(cr *argoproj.ArgoCD) error {
	log.Info("reconciling applicationset service")

	svc := newServiceWithSuffix(common.ApplicationSetServiceNameSuffix, common.ApplicationSetServiceNameSuffix, cr)
	if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.IsEnabled() {

		if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
			err := argoutil.FetchObject(r.Client, cr.Namespace, svc.Name, svc)
			if err != nil {
				return err
			}
			log.Info(fmt.Sprintf("Deleting applicationset controller service %s as applicationset is disabled", svc.Name))
			err = r.Delete(context.TODO(), svc)
			if err != nil {
				return err
			}
		}
		return nil
	} else {
		if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
			return nil // Service found, do nothing
		}
	}
	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "webhook",
			Port:       7000,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(7000),
		}, {
			Name:       "metrics",
			Port:       8080,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8080),
		},
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix(common.ApplicationSetServiceNameSuffix, cr),
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileApplicationSetControllerWebhookRoute will ensure that the ArgoCD Server Route is present.
func (r *ReconcileArgoCD) reconcileApplicationSetControllerWebhookRoute(cr *argoproj.ArgoCD) error {
	name := fmt.Sprintf("%s-%s", common.ApplicationSetServiceNameSuffix, "webhook")
	route := newRouteWithSuffix(name, cr)
	found := argoutil.IsObjectFound(r.Client, cr.Namespace, route.Name, route)
	if found {
		if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.WebhookServer.Route.Enabled {
			// Route exists but enabled flag has been set to false, delete the Route
			return r.Client.Delete(context.TODO(), route)
		}
	}

	if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.WebhookServer.Route.Enabled {
		return nil // Route not enabled, move along...
	}

	// Allow override of the Annotations for the Route.
	if len(cr.Spec.ApplicationSet.WebhookServer.Route.Annotations) > 0 {
		route.Annotations = cr.Spec.Server.Route.Annotations
	}

	// Allow override of the Labels for the Route.
	if len(cr.Spec.ApplicationSet.WebhookServer.Route.Labels) > 0 {
		labels := route.Labels
		for key, val := range cr.Spec.Server.Route.Labels {
			labels[key] = val
		}
		route.Labels = labels
	}

	// Allow override of the Host for the Route.
	if len(cr.Spec.Server.Host) > 0 {
		route.Spec.Host = cr.Spec.ApplicationSet.WebhookServer.Host
	}

	hostname, err := shortenHostname(route.Spec.Host)
	if err != nil {
		return err
	}

	route.Spec.Host = hostname

	if cr.Spec.Server.Insecure {
		// Disable TLS and rely on the cluster certificate.
		route.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("webhook"),
		}
		route.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationEdge,
		}
	} else {
		// Server is using TLS configure passthrough.
		route.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("webhook"),
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
	route.Spec.To.Name = nameWithSuffix(common.ApplicationSetServiceNameSuffix, cr)

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

// reconcileApplicationSetControllerIngress will ensure that the ApplicationSetController Ingress is present.
func (r *ReconcileArgoCD) reconcileApplicationSetControllerIngress(cr *argoproj.ArgoCD) error {
	ingress := newIngressWithSuffix(common.ApplicationSetServiceNameSuffix, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress) {
		if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.WebhookServer.Ingress.Enabled {
			return r.Client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.WebhookServer.Ingress.Enabled {
		log.Info("not enabled")
		return nil // Ingress not enabled, move along...
	}

	// Add annotations
	atns := make(map[string]string)
	atns[common.ArgoCDKeyIngressSSLRedirect] = "true"
	atns[common.ArgoCDKeyIngressBackendProtocol] = "HTTP"

	// Override default annotations if specified
	if len(cr.Spec.ApplicationSet.WebhookServer.Ingress.Annotations) > 0 {
		atns = cr.Spec.ApplicationSet.WebhookServer.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	pathType := networkingv1.PathTypeImplementationSpecific
	httpServerHost, err := getApplicationSetHTTPServerHost(cr)
	if err != nil {
		return err
	}

	// Add rules
	ingress.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: httpServerHost,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path: "/api/webhook",
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: nameWithSuffix(common.ApplicationSetServiceNameSuffix, cr),
									Port: networkingv1.ServiceBackendPort{
										Name: "webhook",
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

	// Allow override of TLS options if specified
	if len(cr.Spec.ApplicationSet.WebhookServer.Ingress.TLS) > 0 {
		ingress.Spec.TLS = cr.Spec.ApplicationSet.WebhookServer.Ingress.TLS
	}

	if err := controllerutil.SetControllerReference(cr, ingress, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ingress)
}

// getApplicationSetHTTPServerHost will return the host for the given ArgoCD.
func getApplicationSetHTTPServerHost(cr *argoproj.ArgoCD) (string, error) {
	host := cr.Name
	if len(cr.Spec.ApplicationSet.WebhookServer.Host) > 0 {
		hostname, err := shortenHostname(cr.Spec.ApplicationSet.WebhookServer.Host)
		if err != nil {
			return "", err
		}
		host = hostname
	}
	return host, nil
}

// reconcileStatusApplicationSetController will ensure that the ApplicationSet controller status is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusApplicationSetController(cr *argoproj.ArgoCD) error {
	status := "Unknown"

	deploy := newDeploymentWithSuffix("applicationset-controller", "controller", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		status = "Pending"

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

	if cr.Status.ApplicationSetController != status {
		cr.Status.ApplicationSetController = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}