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
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	amerr "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	// appset source namespaces should be subset of apps source namespaces
	appsetsSourceNamespaces := []string{}
	appsNamespaces, err := r.getSourceNamespaces(cr)
	if err == nil {
		for _, ns := range cr.Spec.ApplicationSet.SourceNamespaces {
			if contains(appsNamespaces, ns) {
				appsetsSourceNamespaces = append(appsetsSourceNamespaces, ns)
			} else {
				log.V(1).Info(fmt.Sprintf("Apps in target sourceNamespace %s is not enabled, thus skipping the namespace in deployment command.", ns))
			}
		}
	}

	if len(appsetsSourceNamespaces) > 0 {
		cmd = append(cmd, "--applicationset-namespaces", fmt.Sprint(strings.Join(appsetsSourceNamespaces, ",")))
	}

	if len(cr.Spec.ApplicationSet.SCMProviders) > 0 {
		cmd = append(cmd, "--allowed-scm-providers", fmt.Sprint(strings.Join(cr.Spec.ApplicationSet.SCMProviders, ",")))
	}

	// appset in any ns is enabled and no scmProviders allow list is specified,
	// disables scm & PR generators to prevent potential security issues
	// https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/Appset-Any-Namespace/#scm-providers-secrets-consideration
	if len(appsetsSourceNamespaces) > 0 && !(len(cr.Spec.ApplicationSet.SCMProviders) > 0) {
		cmd = append(cmd, "--enable-scm-providers=false")
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

	// create clusterrole & clusterrolebinding if cluster-scoped ArgoCD
	log.Info("reconciling applicationset clusterroles")
	clusterrole, err := r.reconcileApplicationSetClusterRole(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling applicationset clusterrolebindings")
	if err := r.reconcileApplicationSetClusterRoleBinding(cr, clusterrole, sa); err != nil {
		return err
	}

	// reconcile source namespace roles & rolebindings
	log.Info("reconciling applicationset roles & rolebindings in source namespaces")
	if err := r.reconcileApplicationSetSourceNamespacesResources(cr); err != nil {
		return err
	}

	// remove resources for namespaces not part of SourceNamespaces
	log.Info("performing cleanup for applicationset source namespaces")
	if err := r.removeUnmanagedApplicationSetSourceNamespaceResources(cr); err != nil {
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
			return r.Client.Delete(context.TODO(), existing)
		}
		return nil
	}

	deploy := newDeploymentWithSuffix("applicationset-controller", "controller", cr)

	setAppSetLabels(&deploy.ObjectMeta)

	podSpec := &deploy.Spec.Template.Spec

	// sa would be nil when spec.applicationset.enabled = false
	if sa != nil {
		podSpec.ServiceAccountName = sa.ObjectMeta.Name
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
	argoutil.LogResourceCreation(log, deploy)
	return r.Client.Create(context.TODO(), deploy)

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
		ImagePullPolicy: corev1.PullAlways,
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
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			AllowPrivilegeEscalation: boolPtr(false),
			ReadOnlyRootFilesystem:   boolPtr(true),
			RunAsNonRoot:             boolPtr(true),
			SeccompProfile: &corev1.SeccompProfile{
				Type: "RuntimeDefault",
			},
		},
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
			err := r.Client.Delete(context.TODO(), sa)
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
		err := r.Client.Create(context.TODO(), sa)
		if err != nil {
			return sa, err
		}
	}

	return sa, nil
}

// reconcileApplicationSetClusterRoleBinding reconciles required clusterrole for appset controller when ArgoCD is cluster-scoped
func (r *ReconcileArgoCD) reconcileApplicationSetClusterRole(cr *argoproj.ArgoCD) (*v1.ClusterRole, error) {

	allowed := false
	if allowedNamespace(cr.Namespace, os.Getenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")) {
		allowed = true
	}

	// controller disabled, don't create resources
	if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.IsEnabled() {
		allowed = false
	}

	policyRules := []v1.PolicyRule{
		// ApplicationSet
		{
			APIGroups: []string{"argoproj.io"},
			Resources: []string{
				"applications",
				"applicationsets",
			},
			Verbs: []string{
				"list",
				"watch",
			},
		},
		// Secrets
		{
			APIGroups: []string{""},
			Resources: []string{
				"secrets",
			},
			Verbs: []string{
				"list",
				"watch",
			},
		},
	}

	clusterRole := newClusterRole(common.ArgoCDApplicationSetControllerComponent, policyRules, cr)
	if err := applyReconcilerHook(cr, clusterRole, ""); err != nil {
		return nil, err
	}

	existingClusterRole := &v1.ClusterRole{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRole.Name}, existingClusterRole)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the cluster role for the service account associated with %s : %s", clusterRole.Name, err)
		}
		if !allowed {
			// Do Nothing
			return clusterRole, nil
		}
		argoutil.LogResourceCreation(log, clusterRole)
		return clusterRole, r.Client.Create(context.TODO(), clusterRole)
	}

	// ArgoCD not cluster scoped, cleanup any existing resource and exit
	if !allowed {
		argoutil.LogResourceDeletion(log, existingClusterRole, "argocd not cluster scoped")
		err := r.Client.Delete(context.TODO(), existingClusterRole)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return existingClusterRole, err
			}
		}
		return existingClusterRole, nil
	}

	// if the Rules differ, update the Role
	if !reflect.DeepEqual(existingClusterRole.Rules, clusterRole.Rules) {
		existingClusterRole.Rules = clusterRole.Rules
		argoutil.LogResourceUpdate(log, existingClusterRole, "updating rules")
		if err := r.Client.Update(context.TODO(), existingClusterRole); err != nil {
			return nil, err
		}
	}
	return existingClusterRole, nil
}

// reconcileApplicationSetClusterRoleBinding reconciles required clusterrolebinding for appset controller when ArgoCD is cluster-scoped
func (r *ReconcileArgoCD) reconcileApplicationSetClusterRoleBinding(cr *argoproj.ArgoCD, role *v1.ClusterRole, sa *corev1.ServiceAccount) error {

	allowed := false
	if allowedNamespace(cr.Namespace, os.Getenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")) {
		allowed = true
	}

	// controller disabled, don't create resources
	if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.IsEnabled() {
		allowed = false
	}

	clusterRB := newClusterRoleBindingWithname(common.ArgoCDApplicationSetControllerComponent, cr)
	clusterRB.Subjects = []v1.Subject{
		{
			Kind:      v1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: cr.Namespace,
		},
	}
	clusterRB.RoleRef = v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     "ClusterRole",
		Name:     role.Name,
	}
	if err := applyReconcilerHook(cr, clusterRB, ""); err != nil {
		return err
	}

	existingClusterRB := &v1.ClusterRoleBinding{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRB.Name}, existingClusterRB)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to reconcile the cluster rolebinding for the service account associated with %s : %s", clusterRB.Name, err)
		}
		if !allowed {
			// Do Nothing
			return nil
		}
		argoutil.LogResourceCreation(log, clusterRB)
		return r.Client.Create(context.TODO(), clusterRB)
	}

	// ArgoCD not cluster scoped, cleanup any existing resource and exit
	if !allowed {
		argoutil.LogResourceDeletion(log, existingClusterRB, "argocd not cluster scoped")
		err := r.Client.Delete(context.TODO(), existingClusterRB)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
		return nil
	}

	// if subj differ, update the rolebinding
	if !reflect.DeepEqual(existingClusterRB.Subjects, clusterRB.Subjects) {
		existingClusterRB.Subjects = clusterRB.Subjects
		argoutil.LogResourceUpdate(log, existingClusterRB, "updating subjects")
		if err := r.Client.Update(context.TODO(), existingClusterRB); err != nil {
			return err
		}
	} else if !reflect.DeepEqual(existingClusterRB.RoleRef, clusterRB.RoleRef) {
		// RoleRef can't be updated, delete the rolebinding so that it gets recreated
		argoutil.LogResourceDeletion(log, existingClusterRB, "roleref changed, deleting rolebinding so it gets recreated")
		_ = r.Client.Delete(context.TODO(), existingClusterRB)
		return fmt.Errorf("change detected in roleRef for rolebinding %s of Argo CD instance %s in namespace %s", existingClusterRB.Name, cr.Name, existingClusterRB.Namespace)
	}
	return nil
}

// reconcileApplicationSetSourceNamespacesResources creates role & rolebinding in target source namespaces for appset controller
// Appset resources are only created if target source ns is subset of apps source namespaces
func (r *ReconcileArgoCD) reconcileApplicationSetSourceNamespacesResources(cr *argoproj.ArgoCD) error {

	var reconciliationErrors []error

	// controller disabled, nothing to do. cleanup handled by removeUnmanagedApplicationSetSourceNamespaceResources()
	if cr.Spec.ApplicationSet == nil {
		return nil
	}

	// create resources for each appset source namespace
	for _, sourceNamespace := range cr.Spec.ApplicationSet.SourceNamespaces {

		// source ns should be part of app-in-any-ns
		appsNamespaces, err := r.getSourceNamespaces(cr)
		if err != nil {
			reconciliationErrors = append(reconciliationErrors, err)
			continue
		}
		if !contains(appsNamespaces, sourceNamespace) {
			log.Error(fmt.Errorf("skipping reconciliation of resources for sourceNamespace %s as Apps in target sourceNamespace is not enabled", sourceNamespace), "Warning")
			continue
		}

		// skip source ns if doesn't exist
		namespace := &corev1.Namespace{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: sourceNamespace}, namespace); err != nil {
			errMsg := fmt.Errorf("failed to retrieve namespace %s", sourceNamespace)
			reconciliationErrors = append(reconciliationErrors, errors.Join(errMsg, err))
			continue
		}

		// No namespace can be managed by multiple argo-cd instances (cluster scoped or namespace scoped)
		// i.e, only one of either managed-by or applicationset-managed-by-cluster-argocd labels can be applied to a given namespace.
		// Since appset-in-any-ns is in beta, we prioritize managed-by label in case of a conflict.
		if value, ok := namespace.Labels[common.ArgoCDManagedByLabel]; ok && value != "" {
			log.Info(fmt.Sprintf("Skipping reconciling resources for namespace %s as it is already managed-by namespace %s.", namespace.Name, value))
			// remove any source namespace resources
			if val, ok1 := namespace.Labels[common.ArgoCDApplicationSetManagedByClusterArgoCDLabel]; ok1 && val != cr.Namespace {
				delete(r.ManagedApplicationSetSourceNamespaces, namespace.Name)
				if err := r.cleanupUnmanagedApplicationSetSourceNamespaceResources(cr, namespace.Name); err != nil {
					log.Error(err, fmt.Sprintf("error cleaning up resources for namespace %s", namespace.Name))
				}
			}
			continue
		}

		log.Info(fmt.Sprintf("Reconciling applicationset resources for %s", namespace.Name))
		// add applicationset-managed-by-cluster-argocd label on namespace
		if _, ok := namespace.Labels[common.ArgoCDApplicationSetManagedByClusterArgoCDLabel]; !ok {
			// Get the latest value of namespace before updating it
			if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: namespace.Name}, namespace); err != nil {
				return err
			}
			// Update namespace with applicationset-managed-by-cluster-argocd label
			if namespace.Labels == nil {
				namespace.Labels = make(map[string]string)
			}
			namespace.Labels[common.ArgoCDApplicationSetManagedByClusterArgoCDLabel] = cr.Namespace
			explanation := fmt.Sprintf("adding label '%s=%s'", common.ArgoCDApplicationSetManagedByClusterArgoCDLabel, cr.Namespace)
			argoutil.LogResourceUpdate(log, namespace, explanation)
			if err := r.Client.Update(context.TODO(), namespace); err != nil {
				log.Error(err, fmt.Sprintf("failed to add label from namespace [%s]", namespace.Name))
			}
		}

		// role & rolebinding for applicationset controller in source namespace
		role := v1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getResourceNameForApplicationSetSourceNamespaces(cr),
				Namespace: sourceNamespace,
				Labels:    argoutil.LabelsForCluster(cr),
			},
			Rules: policyRuleForApplicationSetController(),
		}
		err = r.reconcileSourceNamespaceRole(role, cr)
		if err != nil {
			reconciliationErrors = append(reconciliationErrors, err)
		}

		roleBinding := v1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        getResourceNameForApplicationSetSourceNamespaces(cr),
				Labels:      argoutil.LabelsForCluster(cr),
				Annotations: argoutil.AnnotationsForCluster(cr),
				Namespace:   sourceNamespace,
			},
			RoleRef: v1.RoleRef{
				APIGroup: v1.GroupName,
				Kind:     "Role",
				Name:     getResourceNameForApplicationSetSourceNamespaces(cr),
			},
			Subjects: []v1.Subject{
				{
					Kind:      v1.ServiceAccountKind,
					Name:      getServiceAccountName(cr.Name, "applicationset-controller"),
					Namespace: cr.Namespace,
				},
			},
		}
		err = r.reconcileSourceNamespaceRoleBinding(roleBinding, cr)
		if err != nil {
			reconciliationErrors = append(reconciliationErrors, err)
		}

		// appset permissions for argocd server in source namespaces are handled by apps-in-any-ns code

		if _, ok := r.ManagedApplicationSetSourceNamespaces[sourceNamespace]; !ok {
			if r.ManagedApplicationSetSourceNamespaces == nil {
				r.ManagedApplicationSetSourceNamespaces = make(map[string]string)
			}
			r.ManagedApplicationSetSourceNamespaces[sourceNamespace] = ""
		}
	}

	return amerr.NewAggregate(reconciliationErrors)
}

func (r *ReconcileArgoCD) reconcileApplicationSetRole(cr *argoproj.ArgoCD) (*v1.Role, error) {

	policyRules := policyRuleForApplicationSetController()

	role := newRole("applicationset-controller", policyRules, cr)
	setAppSetLabels(&role.ObjectMeta)

	exists := true
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: cr.Namespace}, role)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return role, err
		}
		exists = false
	}

	if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.IsEnabled() {
		if exists {
			argoutil.LogResourceDeletion(log, role, "application set not enabled")
			if err := r.Client.Delete(context.TODO(), role); err != nil {
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
		return role, r.Client.Update(context.TODO(), role)
	} else {
		argoutil.LogResourceCreation(log, role)
		return role, r.Client.Create(context.TODO(), role)
	}

}

func (r *ReconcileArgoCD) reconcileApplicationSetRoleBinding(cr *argoproj.ArgoCD, role *v1.Role, sa *corev1.ServiceAccount) error {

	name := "applicationset-controller"

	// get expected name
	roleBinding := newRoleBindingWithname(name, cr)

	// fetch existing rolebinding by name
	roleBindingExists := true
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: cr.Namespace}, roleBinding); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s : %s", name, err)
		}
		roleBindingExists = false
	}

	if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.IsEnabled() {
		if roleBindingExists {
			argoutil.LogResourceDeletion(log, roleBinding, "application set not enabled")
			return r.Client.Delete(context.TODO(), roleBinding)
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
		return r.Client.Update(context.TODO(), roleBinding)
	}

	argoutil.LogResourceCreation(log, roleBinding)
	return r.Client.Create(context.TODO(), roleBinding)
}

func getApplicationSetContainerImage(cr *argoproj.ArgoCD) string {

	defaultImg, defaultTag := false, false
	img := cr.Spec.ApplicationSet.Image
	if img == "" {
		img = cr.Spec.Image
		if img == "" {
			img = common.ArgoCDDefaultArgoImage
			defaultImg = true
		}
	}

	tag := cr.Spec.ApplicationSet.Version
	if tag == "" {
		tag = cr.Spec.Version
		if tag == "" {
			tag = common.ArgoCDDefaultArgoVersion
			defaultTag = true
		}
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
	return r.Client.Create(context.TODO(), svc)
}

// Returns the name of the role/rolebinding for the source namespaces for applicationset-controller in the format of "argocdName-argocdNamespace-applicationset"
func getResourceNameForApplicationSetSourceNamespaces(cr *argoproj.ArgoCD) string {
	return fmt.Sprintf("%s-%s-applicationset", cr.Name, cr.Namespace)
}

// removeUnmanagedApplicationSetSourceNamespaceResources cleansup resources from ApplicationSetSourceNamespaces if namespace is not managed by argocd instance.
// ManagedApplicationSetSourceNamespaces var keeps track of namespaces with appset resources.
func (r *ReconcileArgoCD) removeUnmanagedApplicationSetSourceNamespaceResources(cr *argoproj.ArgoCD) error {

	for ns := range r.ManagedApplicationSetSourceNamespaces {
		managedNamespace := false
		if cr.Spec.ApplicationSet != nil && cr.GetDeletionTimestamp() == nil {
			appsNamespaces, err := r.getSourceNamespaces(cr)
			if err != nil {
				return err
			}
			for _, namespace := range cr.Spec.ApplicationSet.SourceNamespaces {
				// appset ns should be part of apps ns
				if namespace == ns && contains(appsNamespaces, namespace) {
					managedNamespace = true
					break
				}
			}
		}

		if !managedNamespace {
			if err := r.cleanupUnmanagedApplicationSetSourceNamespaceResources(cr, ns); err != nil {
				log.Error(err, fmt.Sprintf("error cleaning up applicationset resources for namespace %s", ns))
				continue
			}
			delete(r.ManagedApplicationSetSourceNamespaces, ns)
		}
	}
	return nil
}

// cleanupUnmanagedApplicationSetSourceNamespaceResources removes the application set resources from target namespace
func (r *ReconcileArgoCD) cleanupUnmanagedApplicationSetSourceNamespaceResources(cr *argoproj.ArgoCD, ns string) error {
	namespace := corev1.Namespace{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: ns}, &namespace); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	// Delete applicationset role & rolebinding
	existingRole := v1.Role{}
	roleName := getResourceNameForApplicationSetSourceNamespaces(cr)
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: roleName, Namespace: namespace.Name}, &existingRole); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to fetch the role for the service account associated with %s : %s", common.ArgoCDApplicationSetControllerComponent, err)
		}
	}
	if existingRole.Name != "" {
		argoutil.LogResourceDeletion(log, &existingRole, "cleaning up unmanaged application set resources")
		err := r.Client.Delete(context.TODO(), &existingRole)
		if err != nil {
			return err
		}
	}

	existingRoleBinding := &v1.RoleBinding{}
	roleBindingName := getResourceNameForApplicationSetSourceNamespaces(cr)
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBindingName, Namespace: namespace.Name}, existingRoleBinding); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s : %s", common.ArgoCDApplicationSetControllerComponent, err)
		}
	}
	if existingRoleBinding.Name != "" {
		argoutil.LogResourceDeletion(log, existingRoleBinding, "cleaning up unmanaged application set resources")
		if err := r.Client.Delete(context.TODO(), existingRoleBinding); err != nil {
			return err
		}
	}

	// app-in-any-ns code will handle removal of appsets permissions for argocd-server in target namespace

	// Remove applicationset-managed-by-cluster-argocd label from the namespace
	argoutil.LogResourceUpdate(log, &namespace, "removing label", common.ArgoCDApplicationSetManagedByClusterArgoCDLabel)
	delete(namespace.Labels, common.ArgoCDApplicationSetManagedByClusterArgoCDLabel)
	if err := r.Client.Update(context.TODO(), &namespace); err != nil {
		return fmt.Errorf("failed to remove applicationset label from namespace %s : %s", namespace.Name, err)
	}

	return nil
}

// setManagedApplicationSetSourceNamespaces populates ManagedApplicationSetSourceNamespaces var with namespaces
// with "argocd.argoproj.io/applicationset-managed-by-cluster-argocd" label.
func (r *ReconcileArgoCD) setManagedApplicationSetSourceNamespaces(cr *argoproj.ArgoCD) error {
	if r.ManagedApplicationSetSourceNamespaces == nil {
		r.ManagedApplicationSetSourceNamespaces = make(map[string]string)
	}
	namespaces := &corev1.NamespaceList{}
	listOption := client.MatchingLabels{
		common.ArgoCDApplicationSetManagedByClusterArgoCDLabel: cr.Namespace,
	}

	// get the list of namespaces managed with "argocd.argoproj.io/applicationset-managed-by-cluster-argocd" label
	if err := r.Client.List(context.TODO(), namespaces, listOption); err != nil {
		return err
	}

	for _, namespace := range namespaces.Items {
		r.ManagedApplicationSetSourceNamespaces[namespace.Name] = ""
	}

	return nil
}

// reconcileSourceNamespaceRole creates/updates role
func (r *ReconcileArgoCD) reconcileSourceNamespaceRole(role v1.Role, cr *argoproj.ArgoCD) error {

	if err := applyReconcilerHook(cr, role, ""); err != nil {
		return err
	}

	existingRole := v1.Role{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, &existingRole)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			errMsg := fmt.Errorf("failed to retrieve role %s in namespace %s", role.Name, role.Namespace)
			return errors.Join(errMsg, err)
		}

		argoutil.LogResourceCreation(log, &role)
		if err := r.Client.Create(context.TODO(), &role); err != nil {
			errMsg := fmt.Errorf("failed to create role %s in namespace %s", role.Name, role.Namespace)
			return errors.Join(errMsg, err)
		}

		log.Info(fmt.Sprintf("role %s created successfully for Argo CD instance %s in namespace %s", role.Name, cr.Name, role.Namespace))
		return nil
	}

	// if the Rules differ, update the Role, ignore if role is just created.
	if !reflect.DeepEqual(existingRole.Rules, role.Rules) {
		existingRole.Rules = role.Rules
		argoutil.LogResourceUpdate(log, &existingRole, "updating rules")
		if err := r.Client.Update(context.TODO(), &existingRole); err != nil {
			errMsg := fmt.Errorf("failed to update role %s in namespace %s", role.Name, role.Namespace)
			return errors.Join(errMsg, err)
		}
	}

	return nil
}

// reconcileSourceNamespaceRole creates/updates rolebinding
func (r *ReconcileArgoCD) reconcileSourceNamespaceRoleBinding(roleBinding v1.RoleBinding, cr *argoproj.ArgoCD) error {

	if err := applyReconcilerHook(cr, roleBinding, ""); err != nil {
		return err
	}

	existingRoleBinding := v1.RoleBinding{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: roleBinding.Namespace}, &existingRoleBinding)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			errMsg := fmt.Errorf("failed to retrieve rolebinding %s in namespace %s", roleBinding.Name, roleBinding.Namespace)
			return errors.Join(errMsg, err)
		}

		argoutil.LogResourceCreation(log, &roleBinding)
		if err := r.Client.Create(context.TODO(), &roleBinding); err != nil {
			errMsg := fmt.Errorf("failed to create rolebinding %s in namespace %s", roleBinding.Name, roleBinding.Namespace)
			return errors.Join(errMsg, err)
		}
		return nil
	}

	// if the RoleRef changes, delete the existing role binding and create a new one
	if !reflect.DeepEqual(roleBinding.RoleRef, existingRoleBinding.RoleRef) {
		argoutil.LogResourceDeletion(log, &existingRoleBinding, "roleref changed, deleting rolebinding so it gets recreated")
		if err = r.Client.Delete(context.TODO(), &existingRoleBinding); err != nil {
			return err
		}
	} else {
		// if the Subjects differ, update the role bindings
		if !reflect.DeepEqual(roleBinding.Subjects, existingRoleBinding.Subjects) {
			existingRoleBinding.Subjects = roleBinding.Subjects
			argoutil.LogResourceUpdate(log, &existingRoleBinding, "updating subjects")
			if err = r.Client.Update(context.TODO(), &existingRoleBinding); err != nil {
				return err
			}
		}
	}

	return nil
}

// getApplicationSetSourceNamespaces return list of namespaces from .spec.ApplicationSet.SourceNamespaces
func (r *ReconcileArgoCD) getApplicationSetSourceNamespaces(cr *argoproj.ArgoCD) []string {
	if cr.Spec.ApplicationSet != nil {
		return cr.Spec.ApplicationSet.SourceNamespaces
	}
	return []string(nil)
}
