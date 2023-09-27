package argocd

import (
	"context"
	"fmt"
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
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

	log.Info("reconciling notifications configmap")
	if err := r.reconcileNotificationsConfigMap(cr); err != nil {
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

	log.Info("reconciling notifications secret")
	if err := r.reconcileNotificationsSecret(cr); err != nil {
		return err
	}

	log.Info("reconciling notifications configmap")
	if err := r.reconcileNotificationsConfigMap(cr); err != nil {
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

		log.Info(fmt.Sprintf("Creating serviceaccount %s", sa.Name))
		err := r.Client.Create(context.TODO(), sa)
		if err != nil {
			return nil, err
		}
	}

	// SA exists but shouldn't, so it should be deleted
	if !cr.Spec.Notifications.Enabled {
		log.Info(fmt.Sprintf("Deleting serviceaccount %s as notifications is disabled", sa.Name))
		return nil, r.Client.Delete(context.TODO(), sa)
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

		log.Info(fmt.Sprintf("Creating role %s", desiredRole.Name))
		err := r.Client.Create(context.TODO(), desiredRole)
		if err != nil {
			return nil, err
		}
		return desiredRole, nil
	}

	// role exists but shouldn't, so it should be deleted
	if !cr.Spec.Notifications.Enabled {
		log.Info(fmt.Sprintf("Deleting role %s as notifications is disabled", existingRole.Name))
		return nil, r.Client.Delete(context.TODO(), existingRole)
	}

	// role exists and should. Reconcile role if changed
	if !reflect.DeepEqual(existingRole.Rules, desiredRole.Rules) {
		existingRole.Rules = desiredRole.Rules
		if err := controllerutil.SetControllerReference(cr, existingRole, r.Scheme); err != nil {
			return nil, err
		}
		return existingRole, r.Client.Update(context.TODO(), existingRole)
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
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: desiredRoleBinding.Name, Namespace: cr.Namespace}, existingRoleBinding); err != nil {
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

		log.Info(fmt.Sprintf("Creating roleBinding %s", desiredRoleBinding.Name))
		return r.Client.Create(context.TODO(), desiredRoleBinding)
	}

	// roleBinding exists but shouldn't, so it should be deleted
	if !cr.Spec.Notifications.Enabled {
		log.Info(fmt.Sprintf("Deleting roleBinding %s as notifications is disabled", existingRoleBinding.Name))
		return r.Client.Delete(context.TODO(), existingRoleBinding)
	}

	// roleBinding exists and should. Reconcile roleBinding if changed
	if !reflect.DeepEqual(existingRoleBinding.RoleRef, desiredRoleBinding.RoleRef) {
		// if the RoleRef changes, delete the existing role binding and create a new one
		if err := r.Client.Delete(context.TODO(), existingRoleBinding); err != nil {
			return err
		}
	} else if !reflect.DeepEqual(existingRoleBinding.Subjects, desiredRoleBinding.Subjects) {
		existingRoleBinding.Subjects = desiredRoleBinding.Subjects
		if err := controllerutil.SetControllerReference(cr, existingRoleBinding, r.Scheme); err != nil {
			return err
		}
		return r.Client.Update(context.TODO(), existingRoleBinding)
	}

	return nil
}

func (r *ReconcileArgoCD) reconcileNotificationsDeployment(cr *argoproj.ArgoCD, sa *corev1.ServiceAccount) error {

	desiredDeployment := newDeploymentWithSuffix("notifications-controller", "controller", cr)

	desiredDeployment.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RecreateDeploymentStrategyType,
	}

	if replicas := getArgoCDNotificationsControllerReplicas(cr); replicas != nil {
		desiredDeployment.Spec.Replicas = replicas
	}

	notificationEnv := cr.Spec.Notifications.Env
	// Let user specify their own environment first
	notificationEnv = argoutil.EnvMerge(notificationEnv, proxyEnvVars(), false)

	podSpec := &desiredDeployment.Spec.Template.Spec
	podSpec.SecurityContext = &corev1.PodSecurityContext{
		RunAsNonRoot: boolPtr(true),
	}
	AddSeccompProfileForOpenShift(r.Client, podSpec)
	podSpec.ServiceAccountName = sa.ObjectMeta.Name
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
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
		},
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

	// fetch existing deployment by name
	deploymentChanged := false
	existingDeployment := &appsv1.Deployment{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: desiredDeployment.Name, Namespace: cr.Namespace}, existingDeployment); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get the deployment associated with %s : %s", existingDeployment.Name, err)
		}

		// deployment does not exist and shouldn't, nothing to do here
		if !cr.Spec.Notifications.Enabled {
			return nil
		}

		// deployment does not exist but should, so it should be created
		if err := controllerutil.SetControllerReference(cr, desiredDeployment, r.Scheme); err != nil {
			return err
		}

		log.Info(fmt.Sprintf("Creating deployment %s", desiredDeployment.Name))
		return r.Client.Create(context.TODO(), desiredDeployment)
	}

	// deployment exists but shouldn't, so it should be deleted
	if !cr.Spec.Notifications.Enabled {
		log.Info(fmt.Sprintf("Deleting deployment %s as notifications is disabled", existingDeployment.Name))
		return r.Client.Delete(context.TODO(), existingDeployment)
	}

	// deployment exists and should. Reconcile deployment if changed
	updateNodePlacement(existingDeployment, desiredDeployment, &deploymentChanged)

	if existingDeployment.Spec.Template.Spec.Containers[0].Image != desiredDeployment.Spec.Template.Spec.Containers[0].Image {
		existingDeployment.Spec.Template.Spec.Containers[0].Image = desiredDeployment.Spec.Template.Spec.Containers[0].Image
		existingDeployment.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Spec.Template.Spec.Containers[0].Command, desiredDeployment.Spec.Template.Spec.Containers[0].Command) {
		existingDeployment.Spec.Template.Spec.Containers[0].Command = desiredDeployment.Spec.Template.Spec.Containers[0].Command
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Spec.Template.Spec.Containers[0].Env,
		desiredDeployment.Spec.Template.Spec.Containers[0].Env) {
		existingDeployment.Spec.Template.Spec.Containers[0].Env = desiredDeployment.Spec.Template.Spec.Containers[0].Env
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Spec.Template.Spec.Volumes, desiredDeployment.Spec.Template.Spec.Volumes) {
		existingDeployment.Spec.Template.Spec.Volumes = desiredDeployment.Spec.Template.Spec.Volumes
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Spec.Replicas, desiredDeployment.Spec.Replicas) {
		existingDeployment.Spec.Replicas = desiredDeployment.Spec.Replicas
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, desiredDeployment.Spec.Template.Spec.Containers[0].VolumeMounts) {
		existingDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = desiredDeployment.Spec.Template.Spec.Containers[0].VolumeMounts
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Spec.Template.Spec.Containers[0].Resources, desiredDeployment.Spec.Template.Spec.Containers[0].Resources) {
		existingDeployment.Spec.Template.Spec.Containers[0].Resources = desiredDeployment.Spec.Template.Spec.Containers[0].Resources
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Spec.Template.Spec.ServiceAccountName, desiredDeployment.Spec.Template.Spec.ServiceAccountName) {
		existingDeployment.Spec.Template.Spec.ServiceAccountName = desiredDeployment.Spec.Template.Spec.ServiceAccountName
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Labels, desiredDeployment.Labels) {
		existingDeployment.Labels = desiredDeployment.Labels
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Spec.Template.Labels, desiredDeployment.Spec.Template.Labels) {
		existingDeployment.Spec.Template.Labels = desiredDeployment.Spec.Template.Labels
		deploymentChanged = true
	}

	if !reflect.DeepEqual(existingDeployment.Spec.Selector, desiredDeployment.Spec.Selector) {
		existingDeployment.Spec.Selector = desiredDeployment.Spec.Selector
		deploymentChanged = true
	}

	if deploymentChanged {
		return r.Client.Update(context.TODO(), existingDeployment)
	}

	return nil

}

// reconcileNotificationsConfigMap only creates/deletes the argocd-notifications-cm based on whether notifications is enabled/disabled in the CR
// It does not reconcile/overwrite any fields or information in the configmap itself
func (r *ReconcileArgoCD) reconcileNotificationsConfigMap(cr *argoproj.ArgoCD) error {

	desiredConfigMap := newConfigMapWithName("argocd-notifications-cm", cr)
	desiredConfigMap.Data = getDefaultNotificationsConfig()

	cmExists := true
	existingConfigMap := &corev1.ConfigMap{}
	if err := argoutil.FetchObject(r.Client, cr.Namespace, desiredConfigMap.Name, existingConfigMap); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get the configmap associated with %s : %s", desiredConfigMap.Name, err)
		}
		cmExists = false
	}

	if cmExists {
		// CM exists but shouldn't, so it should be deleted
		if !cr.Spec.Notifications.Enabled {
			log.Info(fmt.Sprintf("Deleting configmap %s as notifications is disabled", existingConfigMap.Name))
			return r.Client.Delete(context.TODO(), existingConfigMap)
		}

		// CM exists and should, nothing to do here
		return nil
	}

	// CM doesn't exist and shouldn't, nothing to do here
	if !cr.Spec.Notifications.Enabled {
		return nil
	}

	// CM doesn't exist but should, so it should be created
	if err := controllerutil.SetControllerReference(cr, desiredConfigMap, r.Scheme); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Creating configmap %s", desiredConfigMap.Name))
	err := r.Client.Create(context.TODO(), desiredConfigMap)
	if err != nil {
		return err
	}

	return nil
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
			log.Info(fmt.Sprintf("Deleting secret %s as notifications is disabled", existingSecret.Name))
			return r.Client.Delete(context.TODO(), existingSecret)
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

	log.Info(fmt.Sprintf("Creating secret %s", desiredSecret.Name))
	err := r.Client.Create(context.TODO(), desiredSecret)
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
