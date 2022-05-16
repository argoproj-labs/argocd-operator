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

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

func (r *ReconcileArgoCD) reconcileNotificationsController(cr *argoprojv1a1.ArgoCD) error {

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

	log.Info("reconciling notifications deployment")
	if err := r.reconcileNotificationsDeployment(cr, sa); err != nil {
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

	return nil
}

// The code to create/delete notifications resources is written within the reconciliation logic itself. However, these functions must be called
// in the right order depending on whether resources are getting created or deleted. During creation we must create the role and sa first.
// RoleBinding and deployment are dependent on these resouces. During deletion the order is reversed.
// Deployment and RoleBinding must be deleted before the role and sa. deleteNotificationsResources will only be called during
// delete events, so we don't need to worry about duplicate, recurring reconciliation calls
func (r *ReconcileArgoCD) deleteNotificationsResources(cr *argoprojv1a1.ArgoCD) error {

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

	log.Info("reconciling notifications role binding")
	if err := r.reconcileNotificationsRoleBinding(cr, role, sa); err != nil {
		return err
	}

	log.Info("reconciling notifications serviceaccount")
	_, err := r.reconcileNotificationsServiceAccount(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling notifications role")
	_, err = r.reconcileNotificationsRole(cr)
	if err != nil {
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

	return nil
}

func (r *ReconcileArgoCD) reconcileNotificationsServiceAccount(cr *argoprojv1a1.ArgoCD) (*corev1.ServiceAccount, error) {

	sa := newServiceAccountWithName(common.ArgoCDNotificationsControllerComponent, cr)

	saExists := true
	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get the serviceAccount associated with %s : %s", sa.Name, err)
		}
		saExists = false
	}

	if saExists {
		// SA exists but shouldn't, so it should be deleted
		if !cr.Spec.Notifications.Enabled {
			log.Info(fmt.Sprintf("Deleting serviceaccount %s as notifications is disabled", sa.Name))
			return nil, r.Client.Delete(context.TODO(), sa)
		}
		return sa, nil
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

	return sa, nil
}

func (r *ReconcileArgoCD) reconcileNotificationsRole(cr *argoprojv1a1.ArgoCD) (*rbacv1.Role, error) {

	policyRules := policyRuleForNotificationsController()
	desiredRole := newRole(common.ArgoCDNotificationsControllerComponent, policyRules, cr)

	roleExists := true
	existingRole := &rbacv1.Role{}
	if err := argoutil.FetchObject(r.Client, cr.Namespace, desiredRole.Name, existingRole); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get the role associated with %s : %s", desiredRole.Name, err)
		}
		log.Info(fmt.Sprintf("Role %s not found", desiredRole.Name))
		roleExists = false
	}

	if roleExists {
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

func (r *ReconcileArgoCD) reconcileNotificationsRoleBinding(cr *argoprojv1a1.ArgoCD, role *rbacv1.Role, sa *corev1.ServiceAccount) error {

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
	roleBindingExists := true
	existingRoleBinding := &rbacv1.RoleBinding{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: desiredRoleBinding.Name, Namespace: cr.Namespace}, existingRoleBinding); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s : %s", desiredRoleBinding.Name, err)
		}
		roleBindingExists = false
	}

	if roleBindingExists {
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
		}
		if !reflect.DeepEqual(existingRoleBinding.Subjects, desiredRoleBinding.Subjects) {
			existingRoleBinding.Subjects = desiredRoleBinding.Subjects
			if err := controllerutil.SetControllerReference(cr, existingRoleBinding, r.Scheme); err != nil {
				return err
			}
			return r.Client.Update(context.TODO(), existingRoleBinding)
		}
	}

	// roleBinding does not exist and shouldn't, nothing to do here
	if !cr.Spec.Notifications.Enabled {
		return nil
	}

	// roleBinding does not exist but should, so it should be created
	if err := controllerutil.SetControllerReference(cr, desiredRoleBinding, r.Scheme); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Creating role %s", desiredRoleBinding.Name))
	return r.Client.Create(context.TODO(), desiredRoleBinding)
}

func (r *ReconcileArgoCD) reconcileNotificationsDeployment(cr *argoprojv1a1.ArgoCD, sa *corev1.ServiceAccount) error {

	var runAsNonRoot bool = true
	desiredDeployment := newDeploymentWithSuffix("notifications-controller", "controller", cr)

	desiredDeployment.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RecreateDeploymentStrategyType,
	}

	podSpec := &desiredDeployment.Spec.Template.Spec
	podSpec.SecurityContext = &corev1.PodSecurityContext{
		RunAsNonRoot: &runAsNonRoot,
	}
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
		Command:         getNotificationsCommand(),
		Image:           getArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            common.ArgoCDNotificationsControllerComponent,
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
	deploymentExists := true
	deploymentChanged := false
	existingDeployment := &appsv1.Deployment{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: desiredDeployment.Name, Namespace: cr.Namespace}, existingDeployment); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get the deployment associated with %s : %s", existingDeployment.Name, err)
		}
		deploymentExists = false
	}

	if deploymentExists {
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

		if !reflect.DeepEqual(existingDeployment.Spec.Template.Spec.Volumes, desiredDeployment.Spec.Template.Spec.Volumes) {
			existingDeployment.Spec.Template.Spec.Volumes = desiredDeployment.Spec.Template.Spec.Volumes
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

		if !reflect.DeepEqual(existingDeployment.Spec.Template.Spec.NodeSelector, desiredDeployment.Spec.Template.Spec.NodeSelector) {
			existingDeployment.Spec.Template.Spec.NodeSelector = desiredDeployment.Spec.Template.Spec.NodeSelector
			deploymentChanged = true
		}

		if !reflect.DeepEqual(existingDeployment.Spec.Template.Spec.Tolerations, desiredDeployment.Spec.Template.Spec.Tolerations) {
			existingDeployment.Spec.Template.Spec.Tolerations = desiredDeployment.Spec.Template.Spec.Tolerations
			deploymentChanged = true
		}

		if deploymentChanged {
			return r.Client.Update(context.TODO(), existingDeployment)
		}
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

// reconcileNotificationsConfigMap only creates/deletes the argocd-notifications-cm based on whether notifications is enabled/disabled in the CR
// It does not reconcile/overwrite any fields or information in the configmap itself
func (r *ReconcileArgoCD) reconcileNotificationsConfigMap(cr *argoprojv1a1.ArgoCD) error {

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
func (r *ReconcileArgoCD) reconcileNotificationsSecret(cr *argoprojv1a1.ArgoCD) error {

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

	log.Info(fmt.Sprintf("Creating configmap %s", desiredSecret.Name))
	err := r.Client.Create(context.TODO(), desiredSecret)
	if err != nil {
		return err
	}

	return nil
}

func getNotificationsCommand() []string {

	cmd := make([]string, 0)
	cmd = append(cmd, "argocd-notifications")

	return cmd
}

// getNotificationsResources returns the ResourceRequirements for the Notifications container.
func getNotificationsResources(cr *argoprojv1a1.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Notifications.Resources != nil {
		resources = *cr.Spec.Notifications.Resources
	}

	return resources
}

// getDefaultNotificationsConfig returns a map that contains default triggers and template configurations for argocd-notifications-cm
func getDefaultNotificationsConfig() map[string]string {

	notificationsConfig := make(map[string]string)

	// configure default notifications templates

	notificationsConfig["template.app-created"] = `email:
  subject: Application {{.app.metadata.name}} has been created.
message: Application {{.app.metadata.name}} has been created.
teams:
  title: Application {{.app.metadata.name}} has been created.`

	notificationsConfig["template.app-deleted"] = `email:
  subject: Application {{.app.metadata.name}} has been deleted.
message: Application {{.app.metadata.name}} has been deleted.
teams:
  title: Application {{.app.metadata.name}} has been deleted.`

	notificationsConfig["template.app-deployed"] = `email:
  subject: New version of an application {{.app.metadata.name}} is up and running.
message: |
  {{if eq .serviceType "slack"}}:white_check_mark:{{end}} Application {{.app.metadata.name}} is now running new version of deployments manifests.
slack:
  attachments: |
    [{
      "title": "{{ .app.metadata.name}}",
      "title_link":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
      "color": "#18be52",
      "fields": [
      {
        "title": "Sync Status",
        "value": "{{.app.status.sync.status}}",
        "short": true
      },
      {
        "title": "Repository",
        "value": "{{.app.spec.source.repoURL}}",
        "short": true
      },
      {
        "title": "Revision",
        "value": "{{.app.status.sync.revision}}",
        "short": true
      }
      {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "title": "{{$c.type}}",
        "value": "{{$c.message}}",
        "short": true
      }
      {{end}}
      ]
    }]
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: |
    [{
      "name": "Sync Status",
      "value": "{{.app.status.sync.status}}"
    },
    {
      "name": "Repository",
      "value": "{{.app.spec.source.repoURL}}"
    },
    {
      "name": "Revision",
      "value": "{{.app.status.sync.revision}}"
    }
    {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "name": "{{$c.type}}",
        "value": "{{$c.message}}"
      }
    {{end}}
    ]
  potentialAction: |-
    [{
      "@type":"OpenUri",
      "name":"Operation Application",
      "targets":[{
        "os":"default",
        "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}"
      }]
    },
    {
      "@type":"OpenUri",
      "name":"Open Repository",
      "targets":[{
        "os":"default",
        "uri":"{{.app.spec.source.repoURL | call .repo.RepoURLToHTTPS}}"
      }]
    }]
  themeColor: '#000080'
  title: New version of an application {{.app.metadata.name}} is up and running.`

	notificationsConfig["template.app-health-degraded"] = `email:
  subject: Application {{.app.metadata.name}} has degraded.
message: |
  {{if eq .serviceType "slack"}}:exclamation:{{end}} Application {{.app.metadata.name}} has degraded.
  Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.
slack:
  attachments: |
    [{
      "title": "{{ .app.metadata.name}}",
      "title_link": "{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
      "color": "#f4c030",
      "fields": [
      {
        "title": "Health Status",
        "value": "{{.app.status.health.status}}",
        "short": true
      },
      {
        "title": "Repository",
        "value": "{{.app.spec.source.repoURL}}",
        "short": true
      }
      {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "title": "{{$c.type}}",
        "value": "{{$c.message}}",
        "short": true
      }
      {{end}}
      ]
    }]
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: |
    [{
      "name": "Health Status",
      "value": "{{.app.status.health.status}}"
    },
    {
      "name": "Repository",
      "value": "{{.app.spec.source.repoURL}}"
    }
    {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "name": "{{$c.type}}",
        "value": "{{$c.message}}"
      }
    {{end}}
    ]
  potentialAction: |
    [{
      "@type":"OpenUri",
      "name":"Open Application",
      "targets":[{
        "os":"default",
        "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}"
      }]
    },
    {
      "@type":"OpenUri",
      "name":"Open Repository",
      "targets":[{
        "os":"default",
        "uri":"{{.app.spec.source.repoURL | call .repo.RepoURLToHTTPS}}"
      }]
    }]
  themeColor: '#FF0000'
  title: Application {{.app.metadata.name}} has degraded.`

	notificationsConfig["template.app-sync-failed"] = `email:
  subject: Failed to sync application {{.app.metadata.name}}.
message: |
  {{if eq .serviceType "slack"}}:exclamation:{{end}}  The sync operation of application {{.app.metadata.name}} has failed at {{.app.status.operationState.finishedAt}} with the following error: {{.app.status.operationState.message}}
  Sync operation details are available at: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true .
slack:
  attachments: |
    [{
      "title": "{{ .app.metadata.name}}",
      "title_link":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
      "color": "#E96D76",
      "fields": [
      {
        "title": "Sync Status",
        "value": "{{.app.status.sync.status}}",
        "short": true
      },
      {
        "title": "Repository",
        "value": "{{.app.spec.source.repoURL}}",
        "short": true
      }
      {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "title": "{{$c.type}}",
        "value": "{{$c.message}}",
        "short": true
      }
      {{end}}
      ]
    }]
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: |
    [{
      "name": "Sync Status",
      "value": "{{.app.status.sync.status}}"
    },
    {
      "name": "Failed at",
      "value": "{{.app.status.operationState.finishedAt}}"
    },
    {
      "name": "Repository",
      "value": "{{.app.spec.source.repoURL}}"
    }
    {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "name": "{{$c.type}}",
        "value": "{{$c.message}}"
      }
    {{end}}
    ]
  potentialAction: |-
    [{
      "@type":"OpenUri",
      "name":"Open Operation",
      "targets":[{
        "os":"default",
        "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true"
      }]
    },
    {
      "@type":"OpenUri",
      "name":"Open Repository",
      "targets":[{
        "os":"default",
        "uri":"{{.app.spec.source.repoURL | call .repo.RepoURLToHTTPS}}"
      }]
    }]
  themeColor: '#FF0000'
  title: Failed to sync application {{.app.metadata.name}}.`

	notificationsConfig["template.app-sync-running"] = `email:
  subject: Start syncing application {{.app.metadata.name}}.
message: |
  The sync operation of application {{.app.metadata.name}} has started at {{.app.status.operationState.startedAt}}.
  Sync operation details are available at: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true .
slack:
  attachments: |
    [{
      "title": "{{ .app.metadata.name}}",
      "title_link":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
      "color": "#0DADEA",
      "fields": [
      {
        "title": "Sync Status",
        "value": "{{.app.status.sync.status}}",
        "short": true
      },
      {
        "title": "Repository",
        "value": "{{.app.spec.source.repoURL}}",
        "short": true
      }
      {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "title": "{{$c.type}}",
        "value": "{{$c.message}}",
        "short": true
      }
      {{end}}
      ]
    }]
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: |
    [{
      "name": "Sync Status",
      "value": "{{.app.status.sync.status}}"
    },
    {
      "name": "Started at",
      "value": "{{.app.status.operationState.startedAt}}"
    },
    {
      "name": "Repository",
      "value": "{{.app.spec.source.repoURL}}"
    }
    {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "name": "{{$c.type}}",
        "value": "{{$c.message}}"
      }
    {{end}}
    ]
  potentialAction: |-
    [{
      "@type":"OpenUri",
      "name":"Open Operation",
      "targets":[{
        "os":"default",
        "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true"
      }]
    },
    {
      "@type":"OpenUri",
      "name":"Open Repository",
      "targets":[{
        "os":"default",
        "uri":"{{.app.spec.source.repoURL | call .repo.RepoURLToHTTPS}}"
      }]
    }]
  title: Start syncing application {{.app.metadata.name}}.`

	notificationsConfig["template.app-sync-status-unknown"] = `email:
  subject: Application {{.app.metadata.name}} sync status is 'Unknown'
message: |
  {{if eq .serviceType "slack"}}:exclamation:{{end}} Application {{.app.metadata.name}} sync is 'Unknown'.
  Application details: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}.
  {{if ne .serviceType "slack"}}
  {{range $c := .app.status.conditions}}
      * {{$c.message}}
  {{end}}
  {{end}}
slack:
  attachments: |
    [{
      "title": "{{ .app.metadata.name}}",
      "title_link":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
      "color": "#E96D76",
      "fields": [
      {
        "title": "Sync Status",
        "value": "{{.app.status.sync.status}}",
        "short": true
      },
      {
        "title": "Repository",
        "value": "{{.app.spec.source.repoURL}}",
        "short": true
      }
      {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "title": "{{$c.type}}",
        "value": "{{$c.message}}",
        "short": true
      }
      {{end}}
      ]
    }]
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: |
    [{
      "name": "Sync Status",
      "value": "{{.app.status.sync.status}}"
    },
    {
      "name": "Repository",
      "value": "{{.app.spec.source.repoURL}}"
    }
    {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "name": "{{$c.type}}",
        "value": "{{$c.message}}"
      }
    {{end}}
    ]
  potentialAction: |-
    [{
      "@type":"OpenUri",
      "name":"Open Application",
      "targets":[{
        "os":"default",
        "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}"
      }]
    },
    {
      "@type":"OpenUri",
      "name":"Open Repository",
      "targets":[{
        "os":"default",
        "uri":"{{.app.spec.source.repoURL | call .repo.RepoURLToHTTPS}}"
      }]
    }]
  title: Application {{.app.metadata.name}} sync status is 'Unknown'`

	notificationsConfig["template.app-sync-succeeded"] = `email:
  subject: Application {{.app.metadata.name}} has been successfully synced.
message: |
  {{if eq .serviceType "slack"}}:white_check_mark:{{end}} Application {{.app.metadata.name}} has been successfully synced at {{.app.status.operationState.finishedAt}}.
  Sync operation details are available at: {{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true .
slack:
  attachments: |
    [{
      "title": "{{ .app.metadata.name}}",
      "title_link":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}",
      "color": "#18be52",
      "fields": [
      {
        "title": "Sync Status",
        "value": "{{.app.status.sync.status}}",
        "short": true
      },
      {
        "title": "Repository",
        "value": "{{.app.spec.source.repoURL}}",
        "short": true
      }
      {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "title": "{{$c.type}}",
        "value": "{{$c.message}}",
        "short": true
      }
      {{end}}
      ]
    }]
  deliveryPolicy: Post
  groupingKey: ""
  notifyBroadcast: false
teams:
  facts: |
    [{
      "name": "Sync Status",
      "value": "{{.app.status.sync.status}}"
    },
    {
      "name": "Synced at",
      "value": "{{.app.status.operationState.finishedAt}}"
    },
    {
      "name": "Repository",
      "value": "{{.app.spec.source.repoURL}}"
    }
    {{range $index, $c := .app.status.conditions}}
      {{if not $index}},{{end}}
      {{if $index}},{{end}}
      {
        "name": "{{$c.type}}",
        "value": "{{$c.message}}"
      }
    {{end}}
    ]
  potentialAction: |-
    [{
      "@type":"OpenUri",
      "name":"Operation Details",
      "targets":[{
        "os":"default",
        "uri":"{{.context.argocdUrl}}/applications/{{.app.metadata.name}}?operation=true"
      }]
    },
    {
      "@type":"OpenUri",
      "name":"Open Repository",
      "targets":[{
        "os":"default",
        "uri":"{{.app.spec.source.repoURL | call .repo.RepoURLToHTTPS}}"
      }]
    }]
  themeColor: '#000080'
  title: Application {{.app.metadata.name}} has been successfully synced`

	// configure default notifications triggers

	notificationsConfig["trigger.on-created"] = `- description: Application is created.
  oncePer: app.metadata.name
  send:
  - app-created
  when: "true"`

	notificationsConfig["trigger.on-deleted"] = `- description: Application is deleted.
  oncePer: app.metadata.name
  send:
  - app-deleted
  when: app.metadata.deletionTimestamp != nil`

	notificationsConfig["trigger.on-deployed"] = `- description: Application is synced and healthy. Triggered once per commit.
  oncePer: app.status.operationState.syncResult.revision
  send:
  - app-deployed
  when: app.status.operationState.phase in ['Succeeded'] and app.status.health.status
      == 'Healthy'`

	notificationsConfig["trigger.on-health-degraded"] = `- description: Application has degraded
  send:
  - app-health-degraded
  when: app.status.health.status == 'Degraded'`

	notificationsConfig["trigger.on-sync-failed"] = `- description: Application syncing has failed
  send:
  - app-sync-failed
  when: app.status.operationState.phase in ['Error', 'Failed']`

	notificationsConfig["trigger.on-sync-running"] = `- description: Application is being synced
  send:
  - app-sync-running
  when: app.status.operationState.phase in ['Running']`

	notificationsConfig["trigger.on-sync-status-unknown"] = `- description: Application status is 'Unknown'
  end:
  - app-sync-status-unknown
  when: app.status.sync.status == 'Unknown'`

	notificationsConfig["trigger.on-sync-succeeded"] = `-description: Application syncing has succeeded
  send:
  - app-sync-succeeded
  when: app.status.operationState.phase in ['Succeeded']`

	return notificationsConfig
}
