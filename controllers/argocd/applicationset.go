// Copyright 2021 ArgoCD Operator Developers
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
	"os"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

const (
	ApplicationSetGitlabSCMTlsCertPath = "/app/tls/scm/cert"
)

// getArgoApplicationSetCommand will return the command for the ArgoCD ApplicationSet component.
func getArgoApplicationSetCommand(cr *argoproj.ArgoCD) []string {
	cmd := make([]string, 0)

	cmd = append(cmd, "entrypoint.sh")
	cmd = append(cmd, "argocd-applicationset-controller")

	cmd = append(cmd, "--argocd-repo-server")
	cmd = append(cmd, getRepoServerAddress(cr))

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

	podSpec.ServiceAccountName = sa.ObjectMeta.Name

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
		if !errors.IsNotFound(err) {
			return nil, err
		}
		exists = false
	}

	if exists {
		return sa, nil
	}

	if err := controllerutil.SetControllerReference(cr, sa, r.Scheme); err != nil {
		return nil, err
	}

	err := r.Client.Create(context.TODO(), sa)
	if err != nil {
		return nil, err
	}

	return sa, err
}

func (r *ReconcileArgoCD) reconcileApplicationSetRole(cr *argoproj.ArgoCD) (*v1.Role, error) {

	policyRules := []v1.PolicyRule{

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
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the role for the service account associated with %s : %s", role.Name, err)
		}
		if err = controllerutil.SetControllerReference(cr, role, r.Scheme); err != nil {
			return nil, err
		}
		return role, r.Client.Create(context.TODO(), role)
	}

	role.Rules = policyRules
	if err = controllerutil.SetControllerReference(cr, role, r.Scheme); err != nil {
		return nil, err
	}
	return role, r.Client.Update(context.TODO(), role)
}

func (r *ReconcileArgoCD) reconcileApplicationSetRoleBinding(cr *argoproj.ArgoCD, role *v1.Role, sa *corev1.ServiceAccount) error {

	name := "applicationset-controller"

	// get expected name
	roleBinding := newRoleBindingWithname(name, cr)

	// fetch existing rolebinding by name
	roleBindingExists := true
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: cr.Namespace}, roleBinding); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s : %s", name, err)
		}
		roleBindingExists = false
	}

	setAppSetLabels(&roleBinding.ObjectMeta)

	roleBinding.RoleRef = v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     "Role",
		Name:     role.Name,
	}

	roleBinding.Subjects = []v1.Subject{
		{
			Kind:      v1.ServiceAccountKind,
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
	if cr.Spec.ApplicationSet == nil {

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
