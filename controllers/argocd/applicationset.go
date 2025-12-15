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
	"reflect"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

const (
	ApplicationSetGitlabSCMTlsCertPath  = "/app/tls/scm/cert"
	ApplicationSetGitlabSCMTlsMountPath = "/app/tls/scm/"
)

// getArgoApplicationSetCommand will return the command for the ArgoCD ApplicationSet component.
func (r *ReconcileArgoCD) getArgoApplicationSetCommand(cr *argoproj.ArgoCD) []string {
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

	cmd = append(cmd, "--logformat")
	cmd = append(cmd, getLogFormat(cr.Spec.ApplicationSet.LogFormat))

	if cr.Spec.ApplicationSet.SCMRootCAConfigMap != "" {
		cmd = append(cmd, "--scm-root-ca-path")
		cmd = append(cmd, ApplicationSetGitlabSCMTlsCertPath)
	}

	if len(cr.Spec.ApplicationSet.SCMProviders) > 0 {
		cmd = append(cmd, "--allowed-scm-providers", fmt.Sprint(strings.Join(cr.Spec.ApplicationSet.SCMProviders, ",")))
	}

	// ApplicationSet command arguments provided by the user
	extraArgs := cr.Spec.ApplicationSet.ExtraCommandArgs
	cmd = appendUniqueArgs(cmd, extraArgs)

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

	existing := newDeploymentWithSuffix("applicationset-controller", "controller", cr)

	deplExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing)
	if err != nil {
		return err
	}

	if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.IsEnabled() {
		if deplExists {
			argoutil.LogResourceDeletion(log, existing, "application set not enabled")
			return r.Delete(context.TODO(), existing)
		}
		return nil
	}

	deploy := newDeploymentWithSuffix("applicationset-controller", "controller", cr)

	setAppSetLabels(&deploy.ObjectMeta)

	podSpec := &deploy.Spec.Template.Spec

	// sa would be nil when spec.applicationset.enabled = false
	if sa != nil {
		podSpec.ServiceAccountName = sa.Name
	}

	serverVolumes := []corev1.Volume{
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
	if cr.Spec.ApplicationSet.Volumes != nil {
		serverVolumes = append(serverVolumes, cr.Spec.ApplicationSet.Volumes...)
	}
	podSpec.Volumes = serverVolumes
	addSCMGitlabVolumeMount := false
	if scmRootCAConfigMapName := getSCMRootCAConfigMapName(cr); scmRootCAConfigMapName != "" {
		cm := newConfigMapWithName(scmRootCAConfigMapName, cr)

		cmExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, cr.Spec.ApplicationSet.SCMRootCAConfigMap, cm)
		if err != nil {
			return err
		}

		if cmExists {
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

	if cr.Spec.ApplicationSet.Annotations != nil {
		for key, value := range cr.Spec.ApplicationSet.Annotations {
			deploy.Spec.Template.Annotations[key] = value
		}
	}

	if cr.Spec.ApplicationSet.Labels != nil {
		for key, value := range cr.Spec.ApplicationSet.Labels {
			deploy.Spec.Template.Labels[key] = value
		}
	}

	podSpec.Containers = []corev1.Container{
		r.applicationSetContainer(cr, addSCMGitlabVolumeMount),
	}
	AddSeccompProfileForOpenShift(r.Client, podSpec)

	if deplExists {
		// Add Kubernetes-specific labels/annotations from the live object in the source to preserve metadata.
		addKubernetesData(deploy.Spec.Template.Labels, existing.Spec.Template.Labels)
		addKubernetesData(deploy.Spec.Template.Annotations, existing.Spec.Template.Annotations)

		// If the Deployment already exists, make sure the values we care about are up-to-date
		deploymentsDifferent := identifyDeploymentDifference(*existing, *deploy)
		if len(deploymentsDifferent) > 0 {
			existing.Spec.Template.Spec.Containers = podSpec.Containers
			existing.Spec.Template.Spec.Volumes = podSpec.Volumes
			existing.Spec.Template.Spec.ServiceAccountName = podSpec.ServiceAccountName
			existing.Labels = deploy.Labels
			existing.Spec.Template.Labels = deploy.Spec.Template.Labels
			existing.Spec.Selector = deploy.Spec.Selector
			existing.Spec.Template.Spec.NodeSelector = deploy.Spec.Template.Spec.NodeSelector
			existing.Spec.Template.Spec.Tolerations = deploy.Spec.Template.Spec.Tolerations
			existing.Spec.Template.Spec.Containers[0].SecurityContext = deploy.Spec.Template.Spec.Containers[0].SecurityContext
			existing.Spec.Template.Annotations = deploy.Spec.Template.Annotations

			argoutil.LogResourceUpdate(log, existing, "due to difference in", deploymentsDifferent)
			return r.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if !cr.Spec.ApplicationSet.IsEnabled() {
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, deploy)
	return r.Create(context.TODO(), deploy)

}

// identifyDeploymentDifference is a simple comparison of the contents of two
// deployments, returning "" if they are the same, otherwise returning the name
// of the field that changed.
func identifyDeploymentDifference(x appsv1.Deployment, y appsv1.Deployment) string {

	xPodSpec := x.Spec.Template.Spec
	yPodSpec := y.Spec.Template.Spec

	if !reflect.DeepEqual(xPodSpec.Containers, yPodSpec.Containers) {
		return ".Spec.Template.Spec.Containers"
	}

	if !reflect.DeepEqual(xPodSpec.Volumes, yPodSpec.Volumes) {
		return ".Spec.Template.Spec.Volumes"
	}

	if xPodSpec.ServiceAccountName != yPodSpec.ServiceAccountName {
		return "ServiceAccountName"
	}

	if !reflect.DeepEqual(x.Labels, y.Labels) {
		return "Labels"
	}

	if !reflect.DeepEqual(x.Spec.Template.Labels, y.Spec.Template.Labels) {
		return ".Spec.Template.Labels"
	}

	if !reflect.DeepEqual(x.Spec.Selector, y.Spec.Selector) {
		return ".Spec.Selector"
	}

	if !reflect.DeepEqual(xPodSpec.NodeSelector, yPodSpec.NodeSelector) {
		return "Spec.Template.Spec.NodeSelector"
	}

	if !reflect.DeepEqual(xPodSpec.Tolerations, yPodSpec.Tolerations) {
		return "Spec.Template.Spec.Tolerations"
	}

	if !reflect.DeepEqual(xPodSpec.Containers[0].SecurityContext, yPodSpec.Containers[0].SecurityContext) {
		return "Spec.Template.Spec..Containers[0].SecurityContext"
	}

	if !reflect.DeepEqual(x.Spec.Template.Annotations, y.Spec.Template.Annotations) {
		return ".Spec.Template.Annotations"
	}

	return ""
}

func (r *ReconcileArgoCD) applicationSetContainer(cr *argoproj.ArgoCD, addSCMGitlabVolumeMount bool) corev1.Container {
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

	// Default VolumeMounts for ApplicationSetController
	serverVolumeMounts := []corev1.VolumeMount{
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
	}

	// Optional extra VolumeMounts for ApplicationSetController
	if cr.Spec.ApplicationSet.VolumeMounts != nil {
		serverVolumeMounts = append(serverVolumeMounts, cr.Spec.ApplicationSet.VolumeMounts...)
	}

	if addSCMGitlabVolumeMount {
		serverVolumeMounts = append(serverVolumeMounts, corev1.VolumeMount{
			Name:      "appset-gitlab-scm-tls-cert",
			MountPath: ApplicationSetGitlabSCMTlsMountPath,
		})
	}

	container := corev1.Container{
		Command:         r.getArgoApplicationSetCommand(cr),
		Env:             appSetEnv,
		Image:           getApplicationSetContainerImage(cr),
		ImagePullPolicy: argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy),
		Name:            "argocd-applicationset-controller",
		Resources:       getApplicationSetResources(cr),
		VolumeMounts:    serverVolumeMounts,
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
		SecurityContext: argoutil.DefaultSecurityContext(),
	}
	return container
}

func (r *ReconcileArgoCD) reconcileApplicationSetServiceAccount(cr *argoproj.ArgoCD) (*corev1.ServiceAccount, error) {

	sa := newServiceAccountWithName("applicationset-controller", cr)
	setAppSetLabels(&sa.ObjectMeta)

	exists := true
	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		if !apierrors.IsNotFound(err) {
			return sa, err
		}
		exists = false
	}

	if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.IsEnabled() {
		if exists {
			argoutil.LogResourceDeletion(log, sa, "application set not enabled")
			err := r.Delete(context.TODO(), sa)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					return sa, err
				}
			}
		}
		return sa, nil
	}

	if !exists {
		if err := controllerutil.SetControllerReference(cr, sa, r.Scheme); err != nil {
			return sa, err
		}

		argoutil.LogResourceCreation(log, sa)
		err := r.Create(context.TODO(), sa)
		if err != nil {
			return sa, err
		}
	}

	return sa, nil
}

func (r *ReconcileArgoCD) reconcileApplicationSetRole(cr *argoproj.ArgoCD) (*v1.Role, error) {

	policyRules := policyRuleForApplicationSetController()

	role := newRole("applicationset-controller", policyRules, cr)
	setAppSetLabels(&role.ObjectMeta)

	exists := true
	err := r.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: cr.Namespace}, role)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return role, err
		}
		exists = false
	}

	if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.IsEnabled() {
		if exists {
			argoutil.LogResourceDeletion(log, role, "application set not enabled")
			if err := r.Delete(context.TODO(), role); err != nil {
				if !apierrors.IsNotFound(err) {
					return role, err
				}
			}
		}
		return role, nil
	}

	role.Rules = policyRules
	if err = controllerutil.SetControllerReference(cr, role, r.Scheme); err != nil {
		return role, err
	}
	if exists {
		argoutil.LogResourceUpdate(log, role)
		return role, r.Update(context.TODO(), role)
	} else {
		argoutil.LogResourceCreation(log, role)
		return role, r.Create(context.TODO(), role)
	}

}

func (r *ReconcileArgoCD) reconcileApplicationSetRoleBinding(cr *argoproj.ArgoCD, role *v1.Role, sa *corev1.ServiceAccount) error {

	name := "applicationset-controller"

	// get expected name
	roleBinding := newRoleBindingWithname(name, cr)

	// fetch existing rolebinding by name
	roleBindingExists := true
	if err := r.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: cr.Namespace}, roleBinding); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s : %s", name, err)
		}
		roleBindingExists = false
	}

	if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.IsEnabled() {
		if roleBindingExists {
			argoutil.LogResourceDeletion(log, roleBinding, "application set not enabled")
			return r.Delete(context.TODO(), roleBinding)
		}
		return nil
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
		argoutil.LogResourceUpdate(log, roleBinding)
		return r.Update(context.TODO(), roleBinding)
	}

	argoutil.LogResourceCreation(log, roleBinding)
	return r.Create(context.TODO(), roleBinding)
}

// getApplicationSetContainerImage computes the image and tag based on the logic from env and CR spec and combines the image and tag obtained
func getApplicationSetContainerImage(cr *argoproj.ArgoCD) string {
	img, tag := GetImageAndTag(common.ArgoCDImageEnvName, cr.Spec.ApplicationSet.Image, cr.Spec.ApplicationSet.Version, cr.Spec.Image, cr.Spec.Version)
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
	obj.Labels["app.kubernetes.io/part-of"] = "argocd"
	obj.Labels["app.kubernetes.io/component"] = "controller"
}

// reconcileApplicationSetService will ensure that the Service is present for the ApplicationSet webhook and metrics component.
func (r *ReconcileArgoCD) reconcileApplicationSetService(cr *argoproj.ArgoCD) error {
	log.Info("reconciling applicationset service")

	svc := newServiceWithSuffix(common.ApplicationSetServiceNameSuffix, common.ApplicationSetServiceNameSuffix, cr)
	serviceExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc)
	if err != nil {
		return err
	}

	if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.IsEnabled() {

		if serviceExists {
			err := argoutil.FetchObject(r.Client, cr.Namespace, svc.Name, svc)
			if err != nil {
				return err
			}
			argoutil.LogResourceDeletion(log, svc, "application set not enabled")
			if err := r.Delete(context.TODO(), svc); err != nil {
				return err
			}
		}
		return nil

	} else {
		if serviceExists {
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
	argoutil.LogResourceCreation(log, svc)
	return r.Create(context.TODO(), svc)
}
