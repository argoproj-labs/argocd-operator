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
	"os"
	"sort"
	"testing"

	"github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
	"github.com/google/go-cmp/cmp"
	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestReconcileApplicationSet_CreateDeployments(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	a.Spec.ApplicationSet = &v1alpha1.ArgoCDApplicationSet{}

	r := makeTestReconciler(t, a)

	sa := corev1.ServiceAccount{}

	assert.NilError(t, r.reconcileApplicationSetDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NilError(t, r.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	// Ensure the created Deployment has the expected properties
	checkExpectedDeploymentValues(t, deployment, &sa, a)
}

func checkExpectedDeploymentValues(t *testing.T, deployment *appsv1.Deployment, sa *corev1.ServiceAccount, a *v1alpha1.ArgoCD) {
	assert.Equal(t, deployment.Spec.Template.Spec.ServiceAccountName, sa.ObjectMeta.Name)
	appsetAssertExpectedLabels(t, &deployment.ObjectMeta)

	want := []corev1.Container{{
		Command: []string{"applicationset-controller", "--argocd-repo-server", getRepoServerAddress(a), "--loglevel", "info"},
		Env: []corev1.EnvVar{{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		}},
		Image:           argoutil.CombineImageTag(common.ArgoCDDefaultApplicationSetImage, common.ArgoCDDefaultApplicationSetVersion),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "argocd-applicationset-controller",
		VolumeMounts:    repoServerDefaultVolumeMounts(),
	}}

	if diff := cmp.Diff(want, deployment.Spec.Template.Spec.Containers); diff != "" {
		t.Fatalf("failed to reconcile applicationset-controller deployment containers:\n%s", diff)
	}

	volumes := []corev1.Volume{
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
			Name: "argocd-repo-server-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRepoServerTLSSecretName,
					Optional:   boolPtr(true),
				},
			},
		},
	}

	if diff := cmp.Diff(volumes, deployment.Spec.Template.Spec.Volumes); diff != "" {
		t.Fatalf("failed to reconcile applicationset-controller deployment volumes:\n%s", diff)
	}

	expectedSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.ArgoCDKeyName: deployment.Name,
		},
	}

	if diff := cmp.Diff(expectedSelector, deployment.Spec.Selector); diff != "" {
		t.Fatalf("failed to reconcile applicationset-controller label selector:\n%s", diff)
	}
}

func TestReconcileApplicationSet_UpdateExistingDeployments(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()

	a.Spec.ApplicationSet = &v1alpha1.ArgoCDApplicationSet{}

	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.Name + "-applicationset-controller",
			Namespace: a.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "fake-container",
						},
					},
				},
			},
		},
	}

	runtimeObjs := []runtime.Object{a, existingDeployment}
	r := makeTestReconciler(t, runtimeObjs...)

	sa := corev1.ServiceAccount{}

	assert.NilError(t, r.reconcileApplicationSetDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NilError(t, r.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	// Ensure the updated Deployment has the expected properties
	checkExpectedDeploymentValues(t, deployment, &sa, a)

}

func TestReconcileApplicationSet_Deployments_resourceRequirements(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCDWithResources()

	r := makeTestReconciler(t, a)

	sa := corev1.ServiceAccount{}

	assert.NilError(t, r.reconcileApplicationSetDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NilError(t, r.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Equal(t, deployment.Spec.Template.Spec.ServiceAccountName, sa.ObjectMeta.Name)
	appsetAssertExpectedLabels(t, &deployment.ObjectMeta)

	containerWant := []corev1.Container{{
		Command: []string{"applicationset-controller", "--argocd-repo-server", getRepoServerAddress(a), "--loglevel", "info"},
		Env: []corev1.EnvVar{{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		}},
		Image:           argoutil.CombineImageTag(common.ArgoCDDefaultApplicationSetImage, common.ArgoCDDefaultApplicationSetVersion),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "argocd-applicationset-controller",
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resourcev1.MustParse("1024Mi"),
				corev1.ResourceCPU:    resourcev1.MustParse("1000m"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resourcev1.MustParse("2048Mi"),
				corev1.ResourceCPU:    resourcev1.MustParse("2000m"),
			},
		},
		VolumeMounts: repoServerDefaultVolumeMounts(),
	}}

	if diff := cmp.Diff(containerWant, deployment.Spec.Template.Spec.Containers); diff != "" {
		t.Fatalf("failed to reconcile argocd-server deployment:\n%s", diff)
	}

	volumesWant := repoServerDefaultVolumes()

	if diff := cmp.Diff(volumesWant, deployment.Spec.Template.Spec.Volumes); diff != "" {
		t.Fatalf("failed to reconcile argocd-server deployment:\n%s", diff)
	}
}

func TestReconcileApplicationSet_Deployments_SpecOverride(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))

	tests := []struct {
		name                   string
		appSetField            *v1alpha1.ArgoCDApplicationSet
		envVars                map[string]string
		expectedContainerImage string
	}{
		{
			name:                   "unspecified fields should use default",
			appSetField:            &v1alpha1.ArgoCDApplicationSet{},
			expectedContainerImage: argoutil.CombineImageTag(common.ArgoCDDefaultApplicationSetImage, common.ArgoCDDefaultApplicationSetVersion),
		},
		{
			name: "ensure that sha hashes are formatted correctly",
			appSetField: &v1alpha1.ArgoCDApplicationSet{
				Image:   "custom-image",
				Version: "sha256:b835999eb5cf75d01a2678cd971095926d9c2566c9ffe746d04b83a6a0a2849f",
			},
			expectedContainerImage: "custom-image@sha256:b835999eb5cf75d01a2678cd971095926d9c2566c9ffe746d04b83a6a0a2849f",
		},
		{
			name: "custom image should properly substitute",
			appSetField: &v1alpha1.ArgoCDApplicationSet{
				Image:   "custom-image",
				Version: "custom-version",
			},
			expectedContainerImage: "custom-image:custom-version",
		},
		{
			name:                   "verify env var substitution overrides default",
			appSetField:            &v1alpha1.ArgoCDApplicationSet{},
			envVars:                map[string]string{common.ArgoCDApplicationSetEnvName: "custom-env-image"},
			expectedContainerImage: "custom-env-image",
		},

		{
			name: "env var should not override spec fields",
			appSetField: &v1alpha1.ArgoCDApplicationSet{
				Image:   "custom-image",
				Version: "custom-version",
			},
			envVars:                map[string]string{common.ArgoCDApplicationSetEnvName: "custom-env-image"},
			expectedContainerImage: "custom-image:custom-version",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			for testEnvName, testEnvValue := range test.envVars {
				os.Setenv(testEnvName, testEnvValue)
			}

			a := makeTestArgoCD()
			r := makeTestReconciler(t, a)

			a.Spec.ApplicationSet = test.appSetField

			sa := corev1.ServiceAccount{}
			assert.NilError(t, r.reconcileApplicationSetDeployment(a, &sa))

			deployment := &appsv1.Deployment{}
			assert.NilError(t, r.client.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      "argocd-applicationset-controller",
					Namespace: a.Namespace,
				},
				deployment))

			specImage := deployment.Spec.Template.Spec.Containers[0].Image
			assert.Equal(t, specImage, test.expectedContainerImage)

		})
	}

}

func TestReconcileApplicationSet_ServiceAccount(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	retSa, err := r.reconcileApplicationSetServiceAccount(a)
	assert.NilError(t, err)

	sa := &corev1.ServiceAccount{}
	assert.NilError(t, r.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		sa))

	assert.Equal(t, sa.Name, retSa.Name)

	appsetAssertExpectedLabels(t, &sa.ObjectMeta)
}

func TestReconcileApplicationSet_Role(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	roleRet, err := r.reconcileApplicationSetRole(a)
	assert.NilError(t, err)

	role := &rbacv1.Role{}
	assert.NilError(t, r.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		role))

	assert.Equal(t, roleRet.Name, role.Name)
	appsetAssertExpectedLabels(t, &role.ObjectMeta)

	expectedResources := []string{
		"deployments",
		"secrets",
		"configmaps",
		"events",
		"applicationsets/status",
		"applications",
		"applicationsets",
		"appprojects",
		"applicationsets/finalizers",
	}

	foundResources := []string{}

	for _, rule := range role.Rules {
		for _, resource := range rule.Resources {
			foundResources = append(foundResources, resource)
		}
	}

	sort.Strings(expectedResources)
	sort.Strings(foundResources)

	assert.DeepEqual(t, expectedResources, foundResources)
}

func TestReconcileApplicationSet_RoleBinding(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "role-name"}}
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa-name"}}

	err := r.reconcileApplicationSetRoleBinding(a, role, sa)
	assert.NilError(t, err)

	roleBinding := &rbacv1.RoleBinding{}
	assert.NilError(t, r.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		roleBinding))

	appsetAssertExpectedLabels(t, &roleBinding.ObjectMeta)

	assert.Equal(t, roleBinding.RoleRef.Name, role.Name)
	assert.Equal(t, roleBinding.Subjects[0].Name, sa.Name)

}

func appsetAssertExpectedLabels(t *testing.T, meta *metav1.ObjectMeta) {
	assert.Equal(t, meta.Labels["app.kubernetes.io/name"], "argocd-applicationset-controller")
	assert.Equal(t, meta.Labels["app.kubernetes.io/part-of"], "argocd-applicationset")
	assert.Equal(t, meta.Labels["app.kubernetes.io/component"], "controller")
}
