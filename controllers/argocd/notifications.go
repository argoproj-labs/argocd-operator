package argocd

import (
	"context"
	"fmt"
	"reflect"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

const (
	DefaultNotificationsConfigurationInstanceName = "default-notifications-configuration"
)

func (r *ReconcileArgoCD) reconcileNotificationsController(cr *argoproj.ArgoCD) error {

	log.Info("reconciling notifications serviceaccount")
	sa, err := r.reconcileNotificationsServiceAccount(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling notifications role")
	role, err := r.reconcileNotificationsRole(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling notifications role binding")
	if err := r.reconcileNotificationsRoleBinding(cr, role, sa); err != nil {
		return err
	}

	log.Info("reconciling NotificationsConfiguration")
	if err := r.reconcileNotificationsConfigurationCR(cr); err != nil {
		return err
	}

	log.Info("reconciling notifications secret")
	if err := r.reconcileNotificationsSecret(cr); err != nil {
		return err
	}

	log.Info("reconciling notifications deployment")
	if err := r.reconcileNotificationsDeployment(cr, sa); err != nil {
		return err
	}

	log.Info("reconciling notifications metrics service")
	if err := r.reconcileNotificationsMetricsService(cr); err != nil {
		return err
	}

	if prometheusAPIFound {
		log.Info("reconciling notifications metrics service monitor")
		if err := r.reconcileNotificationsServiceMonitor(cr); err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileArgoCD) reconcileNotificationsConfigurationCR(cr *argoproj.ArgoCD) error {

	defaultNotificationsConfigurationCR := &v1alpha1.NotificationsConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name:      DefaultNotificationsConfigurationInstanceName,
			Namespace: cr.Namespace,
		},
		Spec: v1alpha1.NotificationsConfigurationSpec{
			Context:   getDefaultNotificationsContext(),
			Triggers:  getDefaultNotificationsTriggers(),
			Templates: getDefaultNotificationsTemplates(),
		},
	}

	if !cr.Spec.Notifications.Enabled {
		argoutil.LogResourceDeletion(log, defaultNotificationsConfigurationCR, "notifications are disabled")
		return r.Delete(context.TODO(), defaultNotificationsConfigurationCR)
	}

	if err := argoutil.FetchObject(r.Client, cr.Namespace, DefaultNotificationsConfigurationInstanceName,
		defaultNotificationsConfigurationCR); err != nil {

		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get the NotificationsConfiguration associated with %s : %s",
				cr.Name, err)
		}

		// NotificationsConfiguration doesn't exist and shouldn't, nothing to do here
		if !cr.Spec.Notifications.Enabled {
			return nil
		}

		argoutil.LogResourceCreation(log, defaultNotificationsConfigurationCR)
		err := r.Create(context.TODO(), defaultNotificationsConfigurationCR)
		if err != nil {
			return err
		}
	}

	return nil
}

// The code to create/delete notifications resources is written within the reconciliation logic itself. However, these functions must be called
// in the right order depending on whether resources are getting created or deleted. During creation we must create the role and sa first.
// RoleBinding and deployment are dependent on these resouces. During deletion the order is reversed.
// Deployment and RoleBinding must be deleted before the role and sa. deleteNotificationsResources will only be called during
// delete events, so we don't need to worry about duplicate, recurring reconciliation calls
func (r *ReconcileArgoCD) deleteNotificationsResources(cr *argoproj.ArgoCD) error {

	sa := &corev1.ServiceAccount{}
	role := &rbacv1.Role{}

	if err := argoutil.FetchObject(r.Client, cr.Namespace, fmt.Sprintf("%s-%s", cr.Name, common.ArgoCDNotificationsControllerComponent), sa); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}
	if err := argoutil.FetchObject(r.Client, cr.Namespace, fmt.Sprintf("%s-%s", cr.Name, common.ArgoCDNotificationsControllerComponent), role); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	log.Info("reconciling notifications deployment")
	if err := r.reconcileNotificationsDeployment(cr, sa); err != nil {
		return err
	}

	log.Info("reconciling notifications service")
	if err := r.reconcileNotificationsMetricsService(cr); err != nil {
		return err
	}

	log.Info("reconciling notifications service monitor")
	if err := r.reconcileNotificationsServiceMonitor(cr); err != nil {
		return err
	}

	log.Info("reconciling notifications secret")
	if err := r.reconcileNotificationsSecret(cr); err != nil {
		return err
	}

	log.Info("reconciling notifications role binding")
	if err := r.reconcileNotificationsRoleBinding(cr, role, sa); err != nil {
		return err
	}

	log.Info("reconciling notifications role")
	_, err := r.reconcileNotificationsRole(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling notifications serviceaccount")
	_, err = r.reconcileNotificationsServiceAccount(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling notificationsconfiguration")
	err = r.reconcileNotificationsConfigurationCR(cr)
	if err != nil {
		return err
	}

	return nil
}

func (r *ReconcileArgoCD) reconcileNotificationsServiceAccount(cr *argoproj.ArgoCD) (*corev1.ServiceAccount, error) {

	sa := newServiceAccountWithName(common.ArgoCDNotificationsControllerComponent, cr)

	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get the serviceAccount associated with %s : %s", sa.Name, err)
		}

		// SA doesn't exist and shouldn't, nothing to do here
		if !cr.Spec.Notifications.Enabled {
			return nil, nil
		}

		// SA doesn't exist but should, so it should be created
		if err := controllerutil.SetControllerReference(cr, sa, r.Scheme); err != nil {
			return nil, err
		}

		argoutil.LogResourceCreation(log, sa)
		err := r.Create(context.TODO(), sa)
		if err != nil {
			return nil, err
		}
	}

	// SA exists but shouldn't, so it should be deleted
	if !cr.Spec.Notifications.Enabled {
		argoutil.LogResourceDeletion(log, sa, "notifications are disabled")
		return nil, r.Delete(context.TODO(), sa)
	}

	return sa, nil
}

func (r *ReconcileArgoCD) reconcileNotificationsRole(cr *argoproj.ArgoCD) (*rbacv1.Role, error) {

	policyRules := policyRuleForNotificationsController()
	desiredRole := newRole(common.ArgoCDNotificationsControllerComponent, policyRules, cr)

	existingRole := &rbacv1.Role{}
	if err := argoutil.FetchObject(r.Client, cr.Namespace, desiredRole.Name, existingRole); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get the role associated with %s : %s", desiredRole.Name, err)
		}

		// role does not exist and shouldn't, nothing to do here
		if !cr.Spec.Notifications.Enabled {
			return nil, nil
		}

		// role does not exist but should, so it should be created
		if err := controllerutil.SetControllerReference(cr, desiredRole, r.Scheme); err != nil {
			return nil, err
		}

		argoutil.LogResourceCreation(log, desiredRole)
		err := r.Create(context.TODO(), desiredRole)
		if err != nil {
			return nil, err
		}
		return desiredRole, nil
	}

	// role exists but shouldn't, so it should be deleted
	if !cr.Spec.Notifications.Enabled {
		argoutil.LogResourceDeletion(log, existingRole, "notifications are disabled")
		return nil, r.Delete(context.TODO(), existingRole)
	}

	// role exists and should. Reconcile role if changed
	if !reflect.DeepEqual(existingRole.Rules, desiredRole.Rules) {
		existingRole.Rules = desiredRole.Rules
		if err := controllerutil.SetControllerReference(cr, existingRole, r.Scheme); err != nil {
			return nil, err
		}
		argoutil.LogResourceUpdate(log, existingRole, "updating policy rules")
		return existingRole, r.Update(context.TODO(), existingRole)
	}

	return desiredRole, nil
}

func (r *ReconcileArgoCD) reconcileNotificationsRoleBinding(cr *argoproj.ArgoCD, role *rbacv1.Role, sa *corev1.ServiceAccount) error {

	desiredRoleBinding := newRoleBindingWithname(common.ArgoCDNotificationsControllerComponent, cr)
	desiredRoleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     role.Name,
	}

	desiredRoleBinding.Subjects = []rbacv1.Subject{
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
	}

	// fetch existing rolebinding by name
	existingRoleBinding := &rbacv1.RoleBinding{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: desiredRoleBinding.Name, Namespace: cr.Namespace}, existingRoleBinding); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s : %s", desiredRoleBinding.Name, err)
		}

		// roleBinding does not exist and shouldn't, nothing to do here
		if !cr.Spec.Notifications.Enabled {
			return nil
		}

		// roleBinding does not exist but should, so it should be created
		if err := controllerutil.SetControllerReference(cr, desiredRoleBinding, r.Scheme); err != nil {
			return err
		}

		argoutil.LogResourceCreation(log, desiredRoleBinding)
		return r.Create(context.TODO(), desiredRoleBinding)
	}

	// roleBinding exists but shouldn't, so it should be deleted
	if !cr.Spec.Notifications.Enabled {
		argoutil.LogResourceDeletion(log, existingRoleBinding, "notifications are disabled")
		return r.Delete(context.TODO(), existingRoleBinding)
	}

	// roleBinding exists and should. Reconcile roleBinding if changed
	if !reflect.DeepEqual(existingRoleBinding.RoleRef, desiredRoleBinding.RoleRef) {
		// if the RoleRef changes, delete the existing role binding and create a new one
		argoutil.LogResourceDeletion(log, existingRoleBinding, "roleref changed, deleting rolebinding in order to recreate it")
		if err := r.Delete(context.TODO(), existingRoleBinding); err != nil {
			return err
		}
	} else if !reflect.DeepEqual(existingRoleBinding.Subjects, desiredRoleBinding.Subjects) {
		existingRoleBinding.Subjects = desiredRoleBinding.Subjects
		if err := controllerutil.SetControllerReference(cr, existingRoleBinding, r.Scheme); err != nil {
			return err
		}
		argoutil.LogResourceUpdate(log, existingRoleBinding, "updating subjects")
		return r.Update(context.TODO(), existingRoleBinding)
	}

	return nil
}

func (r *ReconcileArgoCD) reconcileNotificationsDeployment(cr *argoproj.ArgoCD, sa *corev1.ServiceAccount) error {

	desiredDeployment := newDeploymentWithSuffix("notifications-controller", "controller", cr)

	desiredDeployment.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RecreateDeploymentStrategyType,
	}

	notificationEnv := cr.Spec.Notifications.Env
	// Let user specify their own environment first
	notificationEnv = argoutil.EnvMerge(notificationEnv, proxyEnvVars(), false)

	podSpec := &desiredDeployment.Spec.Template.Spec
	podSpec.SecurityContext = &corev1.PodSecurityContext{
		RunAsNonRoot: boolPtr(true),
	}
	AddSeccompProfileForOpenShift(r.Client, podSpec)
	podSpec.ServiceAccountName = sa.Name
	podSpec.Volumes = []corev1.Volume{
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
	}

	podSpec.Containers = []corev1.Container{{
		Command:         getNotificationsCommand(cr),
		Image:           getArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            common.ArgoCDNotificationsControllerComponent,
		Env:             notificationEnv,
		Resources:       getNotificationsResources(cr),
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.IntOrString{
						IntVal: int32(9001),
					},
				},
			},
		},
		SecurityContext: argoutil.DefaultSecurityContext(),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "tls-certs",
				MountPath: "/app/config/tls",
			},
			{
				Name:      "argocd-repo-server-tls",
				MountPath: "/app/config/reposerver/tls",
			},
		},
		WorkingDir: "/app",
	}}

	return r.reconcileDeploymentHelper(cr, desiredDeployment, "notifications", cr.Spec.Notifications.Enabled)
}

// reconcileNotificationsService will ensure that the Service for the Notifications controller metrics is present.
func (r *ReconcileArgoCD) reconcileNotificationsMetricsService(cr *argoproj.ArgoCD) error {

	var component = "notifications-controller"
	var suffix = "notifications-controller-metrics"

	svc := newServiceWithSuffix(suffix, component, cr)
	svcExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc)
	if err != nil {
		return err
	}
	if svcExists {
		// Service found, do nothing
		return nil
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix(component, cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       common.NotificationsControllerMetricsPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.NotificationsControllerMetricsPort),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, svc)
	return r.Create(context.TODO(), svc)
}

// reconcileNotificationsServiceMonitor will ensure that the ServiceMonitor for the Notifications controller metrics is present.
func (r *ReconcileArgoCD) reconcileNotificationsServiceMonitor(cr *argoproj.ArgoCD) error {

	if !IsPrometheusAPIAvailable() {
		return nil
	}

	name := fmt.Sprintf("%s-%s", cr.Name, "notifications-controller-metrics")
	serviceMonitor := newServiceMonitorWithName(name, cr)
	smExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, serviceMonitor.Name, serviceMonitor)
	if err != nil {
		return err
	}
	if smExists {
		// Service found, do nothing
		return nil
	}

	serviceMonitor.Spec.Selector = v1.LabelSelector{
		MatchLabels: map[string]string{
			common.ArgoCDKeyName: name,
		},
	}

	serviceMonitor.Spec.Endpoints = []monitoringv1.Endpoint{
		{
			Port:     "metrics",
			Scheme:   "http",
			Interval: "30s",
		},
	}

	argoutil.LogResourceCreation(log, serviceMonitor)
	return r.Create(context.TODO(), serviceMonitor)
}

// reconcileNotificationsSecret only creates/deletes the argocd-notifications-secret based on whether notifications is enabled/disabled in the CR
// It does not reconcile/overwrite any fields or information in the secret itself
func (r *ReconcileArgoCD) reconcileNotificationsSecret(cr *argoproj.ArgoCD) error {

	desiredSecret := argoutil.NewSecretWithName(cr, "argocd-notifications-secret")

	secretExists := true
	existingSecret := &corev1.Secret{}
	if err := argoutil.FetchObject(r.Client, cr.Namespace, desiredSecret.Name, existingSecret); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get the secret associated with %s : %s", desiredSecret.Name, err)
		}
		secretExists = false
	}

	if secretExists {
		// secret exists but shouldn't, so it should be deleted
		if !cr.Spec.Notifications.Enabled {
			argoutil.LogResourceDeletion(log, existingSecret, "notifications are disabled")
			return r.Delete(context.TODO(), existingSecret)
		}

		// secret exists and should, nothing to do here
		return nil
	}

	// secret doesn't exist and shouldn't, nothing to do here
	if !cr.Spec.Notifications.Enabled {
		return nil
	}

	// secret doesn't exist but should, so it should be created
	if err := controllerutil.SetControllerReference(cr, desiredSecret, r.Scheme); err != nil {
		return err
	}

	argoutil.LogResourceCreation(log, desiredSecret)
	err := r.Create(context.TODO(), desiredSecret)
	if err != nil {
		return err
	}

	return nil
}

func getNotificationsCommand(cr *argoproj.ArgoCD) []string {

	cmd := make([]string, 0)
	cmd = append(cmd, "argocd-notifications")

	cmd = append(cmd, "--loglevel")
	cmd = append(cmd, getLogLevel(cr.Spec.Notifications.LogLevel))

	cmd = append(cmd, "--logformat")
	cmd = append(cmd, getLogFormat(cr.Spec.Notifications.LogFormat))

	if cr.Spec.Repo.IsEnabled() {
		cmd = append(cmd, "--argocd-repo-server", getRepoServerAddress(cr))
	} else {
		log.Info("Repo Server is disabled. This would affect the functioning of Notification Controller.")
	}

	return cmd
}

// getNotificationsResources will return the ResourceRequirements for the Notifications container.
func getNotificationsResources(cr *argoproj.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Notifications.Resources != nil {
		resources = *cr.Spec.Notifications.Resources
	}

	return resources
}
