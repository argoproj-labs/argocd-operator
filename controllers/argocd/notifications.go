package argocd

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	amerr "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

	log.Info("reconciling notifications cluster role")
	clusterRole, err := r.reconcileNotificationsClusterRole(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling notifications cluster role binding")
	if err := r.reconcileNotificationsClusterRoleBinding(cr, clusterRole, sa); err != nil {
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

	// reconcile source namespace roles & rolebindings
	log.Info("reconciling notifications roles & rolebindings in source namespaces")
	if err := r.reconcileNotificationsSourceNamespacesResources(cr); err != nil {
		return err
	}

	// remove resources for namespaces not part of SourceNamespaces
	log.Info("performing cleanup for notifications source namespaces")
	if err := r.removeUnmanagedNotificationsSourceNamespaceResources(cr); err != nil {
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

	if err := argoutil.FetchObject(r.Client, cr.Namespace, DefaultNotificationsConfigurationInstanceName,
		defaultNotificationsConfigurationCR); err != nil {

		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the NotificationsConfiguration associated with %s : %s",
				cr.Name, err)
		}

		// NotificationsConfiguration doesn't exist and shouldn't, nothing to do here
		if !isNotificationsEnabled(cr) {
			return nil
		}

		argoutil.LogResourceCreation(log, defaultNotificationsConfigurationCR)
		err := r.Create(context.TODO(), defaultNotificationsConfigurationCR)
		if err != nil {
			return err
		}
	}

	if !isNotificationsEnabled(cr) {
		argoutil.LogResourceDeletion(log, defaultNotificationsConfigurationCR, "notifications are disabled")
		return r.Delete(context.TODO(), defaultNotificationsConfigurationCR)
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
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	if err := argoutil.FetchObject(r.Client, cr.Namespace, fmt.Sprintf("%s-%s", cr.Name, common.ArgoCDNotificationsControllerComponent), role); err != nil {
		if !apierrors.IsNotFound(err) {
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
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get the serviceAccount associated with %s : %s", sa.Name, err)
		}

		// SA doesn't exist and shouldn't, nothing to do here
		if !isNotificationsEnabled(cr) {
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
	if !isNotificationsEnabled(cr) {
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
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get the role associated with %s : %s", desiredRole.Name, err)
		}

		// role does not exist and shouldn't, nothing to do here
		if !isNotificationsEnabled(cr) {
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
	if !isNotificationsEnabled(cr) {
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
	if role != nil && role.Name != "" {
		desiredRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     role.Name,
		}
	}
	if sa != nil && sa.Name != "" {
		desiredRoleBinding.Subjects = []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		}
	}

	// fetch existing rolebinding by name
	existingRoleBinding := &rbacv1.RoleBinding{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: desiredRoleBinding.Name, Namespace: cr.Namespace}, existingRoleBinding); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s : %s", desiredRoleBinding.Name, err)
		}

		// roleBinding does not exist and shouldn't, nothing to do here
		if !isNotificationsEnabled(cr) {
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
	if !isNotificationsEnabled(cr) {
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

func (r *ReconcileArgoCD) reconcileNotificationsClusterRole(cr *argoproj.ArgoCD) (*rbacv1.ClusterRole, error) {

	allowed := argoutil.IsNamespaceClusterConfigNamespace(cr.Namespace)
	// controller disabled, don't create resources
	if !isNotificationsEnabled(cr) {
		allowed = false
	}

	policyRules := policyRuleForNotificationsControllerClusterRole()
	desiredClusterRole := newClusterRole(common.ArgoCDNotificationsControllerComponent, policyRules, cr)

	existingClusterRole := &rbacv1.ClusterRole{}
	if err := argoutil.FetchObject(r.Client, "", desiredClusterRole.Name, existingClusterRole); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get the cluster role associated with %s : %s", desiredClusterRole.Name, err)
		}

		// cluster role does not exist and shouldn't, nothing to do here
		if !allowed {
			return nil, nil
		}

		// cluster role does not exist but should, so it should be created
		argoutil.LogResourceCreation(log, desiredClusterRole)
		err := r.Create(context.TODO(), desiredClusterRole)
		if err != nil {
			return nil, err
		}
		return desiredClusterRole, nil
	}

	// cluster role exists but shouldn't, so it should be deleted
	if !allowed {
		argoutil.LogResourceDeletion(log, existingClusterRole, "notifications are disabled")
		return nil, r.Delete(context.TODO(), existingClusterRole)
	}

	// cluster role exists and should. Reconcile cluster role if changed
	if !reflect.DeepEqual(existingClusterRole.Rules, desiredClusterRole.Rules) {
		existingClusterRole.Rules = desiredClusterRole.Rules
		argoutil.LogResourceUpdate(log, existingClusterRole, "updating policy rules")
		return existingClusterRole, r.Update(context.TODO(), existingClusterRole)
	}

	return desiredClusterRole, nil
}

func (r *ReconcileArgoCD) reconcileNotificationsClusterRoleBinding(cr *argoproj.ArgoCD, clusterRole *rbacv1.ClusterRole, sa *corev1.ServiceAccount) error {

	allowed := argoutil.IsNamespaceClusterConfigNamespace(cr.Namespace)
	// controller disabled, don't create resources
	if !isNotificationsEnabled(cr) {
		allowed = false
	}

	desiredClusterRoleBinding := newClusterRoleBindingWithname(common.ArgoCDNotificationsControllerComponent, cr)
	if clusterRole != nil && clusterRole.Name != "" {
		desiredClusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		}
	}
	if sa != nil && sa.Name != "" {
		desiredClusterRoleBinding.Subjects = []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		}
	}

	// fetch existing cluster rolebinding by name
	existingClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: desiredClusterRoleBinding.Name}, existingClusterRoleBinding); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the cluster rolebinding associated with %s : %s", desiredClusterRoleBinding.Name, err)
		}

		// cluster roleBinding does not exist and shouldn't, nothing to do here
		if !allowed {
			return nil
		}

		// clusterrole does not exist but should, so it should be created first
		// Only create if clusterRole exists
		if clusterRole == nil || clusterRole.Name == "" {
			return nil
		}
		argoutil.LogResourceCreation(log, desiredClusterRoleBinding)
		return r.Create(context.TODO(), desiredClusterRoleBinding)
	}

	// cluster roleBinding exists but shouldn't, so it should be deleted
	if !allowed {
		argoutil.LogResourceDeletion(log, existingClusterRoleBinding, "notifications are disabled")
		return r.Delete(context.TODO(), existingClusterRoleBinding)
	}

	// cluster roleBinding exists and should. Reconcile cluster roleBinding if changed
	if !reflect.DeepEqual(existingClusterRoleBinding.RoleRef, desiredClusterRoleBinding.RoleRef) {
		// if the RoleRef changes, delete the existing cluster role binding and create a new one
		argoutil.LogResourceDeletion(log, existingClusterRoleBinding, "roleref changed, deleting cluster rolebinding in order to recreate it")
		if err := r.Delete(context.TODO(), existingClusterRoleBinding); err != nil {
			return err
		}
		argoutil.LogResourceCreation(log, desiredClusterRoleBinding)
		return r.Create(context.TODO(), desiredClusterRoleBinding)
	} else if !reflect.DeepEqual(existingClusterRoleBinding.Subjects, desiredClusterRoleBinding.Subjects) {
		existingClusterRoleBinding.Subjects = desiredClusterRoleBinding.Subjects
		argoutil.LogResourceUpdate(log, existingClusterRoleBinding, "updating subjects")
		return r.Update(context.TODO(), existingClusterRoleBinding)
	}

	return nil
}

func (r *ReconcileArgoCD) reconcileNotificationsDeployment(cr *argoproj.ArgoCD, sa *corev1.ServiceAccount) error {

	desiredDeployment := newDeploymentWithSuffix("notifications-controller", "controller", cr)

	deplExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, desiredDeployment.Name, desiredDeployment)
	if err != nil {
		return err
	}

	if !isNotificationsEnabled(cr) {
		if deplExists {
			argoutil.LogResourceDeletion(log, desiredDeployment, "notifications not enabled")
			return r.Delete(context.TODO(), desiredDeployment)
		}
		return nil
	}

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
		Command:         r.getNotificationsCommand(cr),
		Image:           getArgoContainerImage(cr),
		ImagePullPolicy: argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy),
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
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the secret associated with %s : %s", desiredSecret.Name, err)
		}
		secretExists = false
	}

	if secretExists {
		// secret exists but shouldn't, so it should be deleted
		if !isNotificationsEnabled(cr) {
			argoutil.LogResourceDeletion(log, existingSecret, "notifications are disabled")
			return r.Delete(context.TODO(), existingSecret)
		}

		// secret exists and should, nothing to do here
		return nil
	}

	// secret doesn't exist and shouldn't, nothing to do here
	if !isNotificationsEnabled(cr) {
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

// reconcileNotificationsSourceNamespacesResources creates role & rolebinding in target source namespaces for notifications controller
// Notifications resources are only created if target source ns is subset of apps source namespaces
func (r *ReconcileArgoCD) reconcileNotificationsSourceNamespacesResources(cr *argoproj.ArgoCD) error {

	var reconciliationErrors []error

	// only cluster scoped argocd manages notifications source namespaces
	if !argoutil.IsNamespaceClusterConfigNamespace(cr.Namespace) {
		return nil
	}

	// controller disabled, nothing to do. cleanup handled by removeUnmanagedNotificationsSourceNamespaceResources()
	if !isNotificationsEnabled(cr) {
		return nil
	}

	// create resources for each notifications source namespace
	for _, sourceNamespace := range cr.Spec.Notifications.SourceNamespaces {

		// source ns should be part of app-in-any-ns
		appsNamespaces, err := r.getSourceNamespaces(cr)
		if err != nil {
			reconciliationErrors = append(reconciliationErrors, err)
			continue
		}
		if !contains(appsNamespaces, sourceNamespace) {
			log.Info(fmt.Sprintf("skipping reconciliation of Notification resources for sourceNamespace %s as Apps in target sourceNamespace is not enabled", sourceNamespace))
			continue
		}

		// skip source ns if doesn't exist
		namespace := &corev1.Namespace{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: sourceNamespace}, namespace); err != nil {
			errMsg := fmt.Errorf("failed to retrieve namespace %s", sourceNamespace)
			reconciliationErrors = append(reconciliationErrors, errors.Join(errMsg, err))
			continue
		}

		// No namespace can be managed by multiple argo-cd instances (cluster scoped or namespace scoped)
		// i.e, only one of either managed-by or notifications-managed-by-cluster-argocd labels can be applied to a given namespace.
		// prioritize managed-by label in case of a conflict.
		if value, ok := namespace.Labels[common.ArgoCDManagedByLabel]; ok && value != "" {
			log.Info(fmt.Sprintf("Skipping reconciling resources for namespace %s as it is already managed-by namespace %s.", namespace.Name, value))
			// remove any source namespace resources
			if val, ok1 := namespace.Labels[common.ArgoCDNotificationsManagedByClusterArgoCDLabel]; ok1 && val != cr.Namespace {
				delete(r.ManagedNotificationsSourceNamespaces, namespace.Name)
				if err := r.cleanupUnmanagedNotificationsSourceNamespaceResources(cr, namespace.Name); err != nil {
					log.Error(err, fmt.Sprintf("error cleaning up resources for namespace %s", namespace.Name))
				}
			}
			continue
		}

		log.Info(fmt.Sprintf("Reconciling notifications resources for %s", namespace.Name))
		// add notifications-managed-by-cluster-argocd label on namespace
		if _, ok := namespace.Labels[common.ArgoCDNotificationsManagedByClusterArgoCDLabel]; !ok {
			// Get the latest value of namespace before updating it
			if err := r.Get(context.TODO(), types.NamespacedName{Name: namespace.Name}, namespace); err != nil {
				return err
			}
			// Update namespace with notifications-managed-by-cluster-argocd label
			if namespace.Labels == nil {
				namespace.Labels = make(map[string]string)
			}
			namespace.Labels[common.ArgoCDNotificationsManagedByClusterArgoCDLabel] = cr.Namespace
			explanation := fmt.Sprintf("adding label '%s=%s'", common.ArgoCDNotificationsManagedByClusterArgoCDLabel, cr.Namespace)
			argoutil.LogResourceUpdate(log, namespace, explanation)
			if err := r.Update(context.TODO(), namespace); err != nil {
				log.Error(err, fmt.Sprintf("failed to add label from namespace [%s]", namespace.Name))
			}
		}

		// role & rolebinding for notifications controller in source namespace
		role := rbacv1.Role{
			ObjectMeta: v1.ObjectMeta{
				Name:      getResourceNameForNotificationsSourceNamespaces(cr),
				Namespace: sourceNamespace,
				Labels:    argoutil.LabelsForCluster(cr),
			},
			Rules: policyRuleForNotificationsController(),
		}
		err = r.reconcileSourceNamespaceRole(role, cr)
		if err != nil {
			reconciliationErrors = append(reconciliationErrors, err)
		}

		roleBinding := rbacv1.RoleBinding{
			ObjectMeta: v1.ObjectMeta{
				Name:        getResourceNameForNotificationsSourceNamespaces(cr),
				Labels:      argoutil.LabelsForCluster(cr),
				Annotations: argoutil.AnnotationsForCluster(cr),
				Namespace:   sourceNamespace,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     getResourceNameForNotificationsSourceNamespaces(cr),
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      getServiceAccountName(cr.Name, "notifications-controller"),
					Namespace: cr.Namespace,
				},
			},
		}
		err = r.reconcileSourceNamespaceRoleBinding(roleBinding, cr)
		if err != nil {
			reconciliationErrors = append(reconciliationErrors, err)
		}

		// ensure NotificationsConfiguration CR exists in the source namespace
		if err := r.reconcileSourceNamespaceNotificationsConfigurationCR(cr, sourceNamespace); err != nil {
			reconciliationErrors = append(reconciliationErrors, err)
		}

		// notifications permissions for argocd server in source namespaces are handled by apps-in-any-ns code

		if _, ok := r.ManagedNotificationsSourceNamespaces[sourceNamespace]; !ok {
			if r.ManagedNotificationsSourceNamespaces == nil {
				r.ManagedNotificationsSourceNamespaces = make(map[string]string)
			}
			r.ManagedNotificationsSourceNamespaces[sourceNamespace] = ""
		}
	}

	return amerr.NewAggregate(reconciliationErrors)
}

// reconcileSourceNamespaceNotificationsConfigurationCR ensures a NotificationsConfiguration CR exists in the given source namespace.
func (r *ReconcileArgoCD) reconcileSourceNamespaceNotificationsConfigurationCR(cr *argoproj.ArgoCD, sourceNamespace string) error {
	if !isNotificationsEnabled(cr) {
		return nil
	}

	// Check if NotificationsConfiguration exists in source namespace
	sourceNotifCfg := &v1alpha1.NotificationsConfiguration{}
	err := argoutil.FetchObject(r.Client, sourceNamespace, DefaultNotificationsConfigurationInstanceName, sourceNotifCfg)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the NotificationsConfiguration from source namespace %s : %s", sourceNamespace, err)
		}
		// Not found in source namespace, create it with propagated/default spec
		newCfg := &v1alpha1.NotificationsConfiguration{
			ObjectMeta: v1.ObjectMeta{
				Name:      DefaultNotificationsConfigurationInstanceName,
				Namespace: sourceNamespace,
			},
		}
		argoutil.LogResourceCreation(log, sourceNotifCfg, "creating NotificationsConfiguration for namespace")
		return r.Create(context.TODO(), newCfg)
	}

	return nil
}

func (r *ReconcileArgoCD) getNotificationsCommand(cr *argoproj.ArgoCD) []string {

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

	// notifications source namespaces should be subset of apps source namespaces
	notificationsSourceNamespaces := []string{}
	appsNamespaces, err := r.getSourceNamespaces(cr)
	if err == nil {
		for _, ns := range cr.Spec.Notifications.SourceNamespaces {
			if contains(appsNamespaces, ns) {
				notificationsSourceNamespaces = append(notificationsSourceNamespaces, ns)
			} else {
				log.V(1).Info(fmt.Sprintf("Apps in target sourceNamespace %s is not enabled, thus skipping the namespace in deployment command.", ns))
			}
		}
	}

	if len(notificationsSourceNamespaces) > 0 && argoutil.IsNamespaceClusterConfigNamespace(cr.Namespace) {
		cmd = append(cmd, "--application-namespaces", strings.Join(notificationsSourceNamespaces, ","))
		cmd = append(cmd, "--self-service-notification-enabled", "true")
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

// Returns the name of the role/rolebinding for the source namespaces for notifications-controller in the format of "argocdName-argocdNamespace-notifications"
func getResourceNameForNotificationsSourceNamespaces(cr *argoproj.ArgoCD) string {
	return fmt.Sprintf("%s-%s-notifications", cr.Name, cr.Namespace)
}

// setManagedNotificationSourceNamespaces populates ManagedNotificationsSourceNamespaces var with namespaces
// with "argocd.argoproj.io/notifications-managed-by-cluster-argocd" label.
func (r *ReconcileArgoCD) setManagedNotificationsSourceNamespaces(cr *argoproj.ArgoCD) error {
	if r.ManagedNotificationsSourceNamespaces == nil {
		r.ManagedNotificationsSourceNamespaces = make(map[string]string)
	}
	namespaces := &corev1.NamespaceList{}
	listOption := client.MatchingLabels{
		common.ArgoCDNotificationsManagedByClusterArgoCDLabel: cr.Namespace,
	}

	// get the list of namespaces managed with "argocd.argoproj.io/notifications-managed-by-cluster-argocd" label
	if err := r.List(context.TODO(), namespaces, listOption); err != nil {
		return err
	}

	for _, namespace := range namespaces.Items {
		r.ManagedNotificationsSourceNamespaces[namespace.Name] = ""
	}

	return nil
}

// removeUnmanagedNotificationsSourceNamespaceResources cleans up resources from NotificationsSourceNamespaces if namespace is not managed by argocd instance.
// ManagedNotificationsSourceNamespaces var keeps track of namespaces with notifications resources.
func (r *ReconcileArgoCD) removeUnmanagedNotificationsSourceNamespaceResources(cr *argoproj.ArgoCD) error {

	for ns := range r.ManagedNotificationsSourceNamespaces {

		// Retrieve the namespace object in the 'managed application source namespaces' list
		ns_namespace := &corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{Name: ns},
		}
		if err := r.Get(context.Background(), client.ObjectKeyFromObject(ns_namespace), ns_namespace); err != nil {
			if apierrors.IsNotFound(err) {
				continue // skip if not found
			} else {
				return fmt.Errorf("unable to get ns: %v", err)
			}
		}
		// We want to determine the Argo CD namespace that manages ns. We use labels to determine that.
		var argocdNamespaceThatManagesNamespace string

		// First try to notifications managed by label value
		if val, ok := ns_namespace.Labels[common.ArgoCDNotificationsManagedByClusterArgoCDLabel]; ok {
			argocdNamespaceThatManagesNamespace = val

		} else if val, ok := ns_namespace.Labels[common.ArgoCDApplicationSetManagedByClusterArgoCDLabel]; ok {
			// Next try to label applicationset managed by label value
			argocdNamespaceThatManagesNamespace = val

		} else if val, ok := ns_namespace.Labels[common.ArgoCDManagedByLabel]; ok {
			// Next try to generic managed by label
			argocdNamespaceThatManagesNamespace = val
		} else {
			// Give up and continue
			log.Info("could not locate owner for " + ns)
			continue
		}

		// For the following logic, the CR must be the one that owns the namespace
		if argocdNamespaceThatManagesNamespace != cr.Namespace || argocdNamespaceThatManagesNamespace == "" {
			continue
		}

		managedNamespace := false
		if isNotificationsEnabled(cr) && cr.GetDeletionTimestamp() == nil {
			notificationsNamespaces, err := r.getSourceNamespaces(cr)
			if err != nil {
				return err
			}
			for _, namespace := range cr.Spec.Notifications.SourceNamespaces {
				// notifications ns should be part of general ns
				if namespace == ns && contains(notificationsNamespaces, namespace) {
					managedNamespace = true
					break
				}
			}
		}

		if !argoutil.IsNamespaceClusterConfigNamespace(cr.Namespace) {
			managedNamespace = false
		}

		if !managedNamespace {
			if err := r.cleanupUnmanagedNotificationsSourceNamespaceResources(cr, ns); err != nil {
				log.Error(err, fmt.Sprintf("error cleaning up notifications resources for namespace %s", ns))
				continue
			}
			delete(r.ManagedNotificationsSourceNamespaces, ns)
		}
	}
	return nil
}

// isNotificationsEnabled returns true if notifications are configured and enabled in the ArgoCD CR
func isNotificationsEnabled(cr *argoproj.ArgoCD) bool {
	return !reflect.DeepEqual(cr.Spec.Notifications, argoproj.ArgoCDNotifications{}) && cr.Spec.Notifications.Enabled
}

// cleanupUnmanagedNotificationsSourceNamespaceResources removes the notifications resources from target namespace
func (r *ReconcileArgoCD) cleanupUnmanagedNotificationsSourceNamespaceResources(cr *argoproj.ArgoCD, ns string) error {
	namespace := corev1.Namespace{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: ns}, &namespace); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	// Delete notifications role & rolebinding
	existingRole := rbacv1.Role{}
	roleName := getResourceNameForNotificationsSourceNamespaces(cr)
	if err := r.Get(context.TODO(), types.NamespacedName{Name: roleName, Namespace: namespace.Name}, &existingRole); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to fetch the role for the service account associated with %s : %s", common.ArgoCDNotificationsControllerComponent, err)
		}
	}
	if existingRole.Name != "" {
		argoutil.LogResourceDeletion(log, &existingRole, "cleaning up unmanaged notifications resources")
		err := r.Delete(context.TODO(), &existingRole)
		if err != nil {
			return err
		}
	}

	existingRoleBinding := &rbacv1.RoleBinding{}
	roleBindingName := getResourceNameForNotificationsSourceNamespaces(cr)
	if err := r.Get(context.TODO(), types.NamespacedName{Name: roleBindingName, Namespace: namespace.Name}, existingRoleBinding); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s : %s", common.ArgoCDNotificationsControllerComponent, err)
		}
	}
	if existingRoleBinding.Name != "" {
		argoutil.LogResourceDeletion(log, existingRoleBinding, "cleaning up unmanaged notifications resources")
		if err := r.Delete(context.TODO(), existingRoleBinding); err != nil {
			return err
		}
	}

	// Delete NotificationsConfiguration CR in source namespace
	notifCfg := &v1alpha1.NotificationsConfiguration{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: DefaultNotificationsConfigurationInstanceName, Namespace: namespace.Name}, notifCfg); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the NotificationsConfiguration in namespace %s : %s", namespace.Name, err)
		}
	} else {
		argoutil.LogResourceDeletion(log, notifCfg, "cleaning up unmanaged notifications resources")
		if err := r.Delete(context.TODO(), notifCfg); err != nil {
			return fmt.Errorf("failed to delete the NotificationsConfiguration in namespace %s : %s", namespace.Name, err)
		}
	}

	// Remove notifications-managed-by-cluster-argocd label from the namespace
	argoutil.LogResourceUpdate(log, &namespace, "removing label", common.ArgoCDNotificationsManagedByClusterArgoCDLabel)
	delete(namespace.Labels, common.ArgoCDNotificationsManagedByClusterArgoCDLabel)
	if err := r.Update(context.TODO(), &namespace); err != nil {
		return fmt.Errorf("failed to remove notifications label from namespace %s : %s", namespace.Name, err)
	}

	return nil
}

// getNotificationsSetSourceNamespaces return list of namespaces from .spec.Notifications.SourceNamespaces
func (r *ReconcileArgoCD) getNotificationsSourceNamespaces(cr *argoproj.ArgoCD) []string {
	if isNotificationsEnabled(cr) {
		return cr.Spec.Notifications.SourceNamespaces
	}
	return []string(nil)
}
