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

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

func applicationSetDefaultVolumeMounts() []corev1.VolumeMount {
	repoMounts := repoServerDefaultVolumeMounts()
	ignoredMounts := map[string]bool{
		"plugins":                             true,
		"argocd-repo-server-tls":              true,
		common.ArgoCDRedisServerTLSSecretName: true,
	}
	mounts := make([]corev1.VolumeMount, len(repoMounts)-len(ignoredMounts), len(repoMounts)-len(ignoredMounts))
	j := 0
	for _, mount := range repoMounts {
		if !ignoredMounts[mount.Name] {
			mounts[j] = mount
			j += 1
		}
	}
	return mounts
}

func applicationSetDefaultVolumes() []corev1.Volume {
	repoVolumes := repoServerDefaultVolumes()
	ignoredVolumes := map[string]bool{
		"var-files":                           true,
		"plugins":                             true,
		"argocd-repo-server-tls":              true,
		common.ArgoCDRedisServerTLSSecretName: true,
	}
	volumes := make([]corev1.Volume, len(repoVolumes)-len(ignoredVolumes), len(repoVolumes)-len(ignoredVolumes))
	j := 0
	for _, volume := range repoVolumes {
		if !ignoredVolumes[volume.Name] {
			volumes[j] = volume
			j += 1
		}
	}
	return volumes
}

func TestReconcileApplicationSet_CreateDeployments(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	sa := corev1.ServiceAccount{}

	assert.NoError(t, r.reconcileApplicationSetDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	// Ensure the created Deployment has the expected properties
	checkExpectedDeploymentValues(t, r, deployment, &sa, nil, nil, a)
}

func checkExpectedDeploymentValues(t *testing.T, r *ReconcileArgoCD, deployment *appsv1.Deployment, sa *corev1.ServiceAccount, extraVolumes *[]corev1.Volume, extraVolumeMounts *[]corev1.VolumeMount, a *argoproj.ArgoCD) {
	assert.Equal(t, deployment.Spec.Template.Spec.ServiceAccountName, sa.ObjectMeta.Name)
	appsetAssertExpectedLabels(t, &deployment.ObjectMeta)

	want := []corev1.Container{r.applicationSetContainer(a, false)}

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
			Name: "tmp",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	if a.Spec.ApplicationSet.SCMRootCAConfigMap != "" && argoutil.IsObjectFound(r.Client, a.Namespace, common.ArgoCDAppSetGitlabSCMTLSCertsConfigMapName, a) {
		volumes = append(volumes, corev1.Volume{
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

	if extraVolumes != nil {
		volumes = append(volumes, *extraVolumes...)
	}

	if diff := cmp.Diff(volumes, deployment.Spec.Template.Spec.Volumes); diff != "" {
		t.Fatalf("failed to reconcile applicationset-controller deployment volumes:\n%s", diff)
	}

	volumeMounts := []corev1.VolumeMount{
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

	if extraVolumeMounts != nil {
		volumeMounts = append(volumeMounts, *extraVolumeMounts...)
	}

	// Verify VolumeMounts
	if diff := cmp.Diff(volumeMounts, deployment.Spec.Template.Spec.Containers[0].VolumeMounts); diff != "" {
		t.Fatalf("failed to reconcile applicationset-controller deployment volume mounts:\n%s", diff)
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

func TestReconcileApplicationSetProxyConfiguration(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	// Proxy Env vars
	setProxyEnvVars(t)

	a := makeTestArgoCD()
	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	sa := corev1.ServiceAccount{}

	r.reconcileApplicationSetDeployment(a, &sa)

	want := []corev1.EnvVar{
		{
			Name:  "HTTPS_PROXY",
			Value: "https://example.com",
		},
		{
			Name:  "HTTP_PROXY",
			Value: "http://example.com",
		},
		{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name:  "NO_PROXY",
			Value: ".cluster.local",
		},
	}

	deployment := &appsv1.Deployment{}

	// reconcile ApplicationSets
	r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment)

	if diff := cmp.Diff(want, deployment.Spec.Template.Spec.Containers[0].Env); diff != "" {
		t.Fatalf("failed to reconcile applicationset-controller deployment containers:\n%s", diff)
	}

}

func TestReconcileApplicationSetVolumes(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	extraVolumes := []corev1.Volume{
		{
			Name: "example-volume",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	extraVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "example-volume",
			MountPath: "/mnt/data",
		},
	}

	a := makeTestArgoCD()
	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
		Volumes:      extraVolumes,
		VolumeMounts: extraVolumeMounts,
	}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	sa := corev1.ServiceAccount{}

	// Reconcile the ApplicationSet deployment
	assert.NoError(t, r.reconcileApplicationSetDeployment(a, &sa))

	// Get the deployment after reconciliation
	deployment := &appsv1.Deployment{}
	err := r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment,
	)
	if err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}

	// Ensure the created Deployment has the expected properties
	checkExpectedDeploymentValues(t, r, deployment, &sa, &extraVolumes, &extraVolumeMounts, a)
}

func TestReconcileApplicationSet_UpdateExistingDeployments(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}

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

	resObjs := []client.Object{a, existingDeployment}
	subresObjs := []client.Object{a, existingDeployment}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	sa := corev1.ServiceAccount{}

	assert.NoError(t, r.reconcileApplicationSetDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	// Ensure the updated Deployment has the expected properties
	checkExpectedDeploymentValues(t, r, deployment, &sa, nil, nil, a)

}

func TestReconcileApplicationSet_Deployments_resourceRequirements(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCDWithResources()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	sa := corev1.ServiceAccount{}

	assert.NoError(t, r.reconcileApplicationSetDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Equal(t, deployment.Spec.Template.Spec.ServiceAccountName, sa.ObjectMeta.Name)
	appsetAssertExpectedLabels(t, &deployment.ObjectMeta)

	containerWant := []corev1.Container{r.applicationSetContainer(a, false)}

	if diff := cmp.Diff(containerWant, deployment.Spec.Template.Spec.Containers); diff != "" {
		t.Fatalf("failed to reconcile argocd-server deployment:\n%s", diff)
	}

	volumesWant := applicationSetDefaultVolumes()

	if diff := cmp.Diff(volumesWant, deployment.Spec.Template.Spec.Volumes); diff != "" {
		t.Fatalf("failed to reconcile argocd-server deployment:\n%s", diff)
	}
}

func TestReconcileApplicationSet_Deployments_SpecOverride(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name                   string
		appSetField            *argoproj.ArgoCDApplicationSet
		argocdField            argoproj.ArgoCDSpec
		envVars                map[string]string
		expectedContainerImage string
	}{
		{
			name:        "fields are set in argocd spec and not on appsetspec",
			appSetField: &argoproj.ArgoCDApplicationSet{},
			argocdField: argoproj.ArgoCDSpec{
				Image:   "test",
				Version: "sha256:b835999eb5cf75d01a2678cd971095926d9c2566c9ffe746d04b83a6a0a2849f",
			},
			expectedContainerImage: "test@sha256:b835999eb5cf75d01a2678cd971095926d9c2566c9ffe746d04b83a6a0a2849f",
		},
		{
			name: "fields are set in both argocdSpec and on appsetSpec",
			appSetField: &argoproj.ArgoCDApplicationSet{
				Image:   "custom-image",
				Version: "sha256:b835999eb5cf75d01a2678cd971095926d9c2566c9ffe746d04b83a6a0a2849f",
			},
			argocdField: argoproj.ArgoCDSpec{
				Image:   "test",
				Version: "sha256:b835999eb5cf75d01a2678cd9710952566c9ffe746d04b83a6a0a2849f926d9c",
			},
			expectedContainerImage: "custom-image@sha256:b835999eb5cf75d01a2678cd971095926d9c2566c9ffe746d04b83a6a0a2849f",
		},
		{
			name:                   "unspecified fields should use default",
			appSetField:            &argoproj.ArgoCDApplicationSet{},
			expectedContainerImage: argoutil.CombineImageTag(common.ArgoCDDefaultArgoImage, common.ArgoCDDefaultArgoVersion),
		},
		{
			name: "ensure that sha hashes are formatted correctly",
			appSetField: &argoproj.ArgoCDApplicationSet{
				Image:   "custom-image",
				Version: "sha256:b835999eb5cf75d01a2678cd971095926d9c2566c9ffe746d04b83a6a0a2849f",
			},
			expectedContainerImage: "custom-image@sha256:b835999eb5cf75d01a2678cd971095926d9c2566c9ffe746d04b83a6a0a2849f",
		},
		{
			name: "custom image should properly substitute",
			appSetField: &argoproj.ArgoCDApplicationSet{
				Image:   "custom-image",
				Version: "custom-version",
			},
			expectedContainerImage: "custom-image:custom-version",
		},
		{
			name:                   "verify env var substitution overrides default",
			appSetField:            &argoproj.ArgoCDApplicationSet{},
			envVars:                map[string]string{common.ArgoCDImageEnvName: "custom-env-image"},
			expectedContainerImage: "custom-env-image",
		},

		{
			name: "env var should not override spec fields",
			appSetField: &argoproj.ArgoCDApplicationSet{
				Image:   "custom-image",
				Version: "custom-version",
			},
			envVars:                map[string]string{common.ArgoCDImageEnvName: "custom-env-image"},
			expectedContainerImage: "custom-image:custom-version",
		},
		{
			name: "ensure scm tls cert mount is present",
			appSetField: &argoproj.ArgoCDApplicationSet{
				SCMRootCAConfigMap: "test-scm-tls-mount",
			},
			envVars:                map[string]string{common.ArgoCDImageEnvName: "custom-env-image"},
			expectedContainerImage: "custom-env-image",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			for testEnvName, testEnvValue := range test.envVars {
				t.Setenv(testEnvName, testEnvValue)
			}

			a := makeTestArgoCD()
			resObjs := []client.Object{a}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch)
			cm := newConfigMapWithName(getCAConfigMapName(a), a)
			r.Client.Create(context.Background(), cm, &client.CreateOptions{})

			if test.argocdField.Image != "" {
				a.Spec.Image = test.argocdField.Image
				a.Spec.Version = test.argocdField.Version
			}

			a.Spec.ApplicationSet = test.appSetField

			sa := corev1.ServiceAccount{}
			assert.NoError(t, r.reconcileApplicationSetDeployment(a, &sa))

			deployment := &appsv1.Deployment{}
			assert.NoError(t, r.Client.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      "argocd-applicationset-controller",
					Namespace: a.Namespace,
				},
				deployment))

			specImage := deployment.Spec.Template.Spec.Containers[0].Image
			assert.Equal(t, test.expectedContainerImage, specImage)
			checkExpectedDeploymentValues(t, r, deployment, &sa, nil, nil, a)
		})
	}

}

func TestReconcileApplicationSet_Deployments_Command(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name           string
		argocdSpec     argoproj.ArgoCDSpec
		expectedCmd    []string
		notExpectedCmd []string
	}{
		{
			name: "Appset in any namespaces without scm provider list",
			argocdSpec: argoproj.ArgoCDSpec{
				ApplicationSet: &argoproj.ArgoCDApplicationSet{
					SourceNamespaces: []string{"foo", "bar"},
				},
				SourceNamespaces: []string{"foo", "bar"},
			},
			expectedCmd: []string{"--applicationset-namespaces", "foo,bar", "--enable-scm-providers=false"},
		},
		{
			name: "with SCM provider list",
			argocdSpec: argoproj.ArgoCDSpec{
				ApplicationSet: &argoproj.ArgoCDApplicationSet{
					SourceNamespaces: []string{"foo"},
					SCMProviders:     []string{"github.com"},
				},
				SourceNamespaces: []string{"foo", "bar"},
			},
			expectedCmd: []string{"--applicationset-namespaces", "foo", "--allowed-scm-providers", "github.com"},
		},
		{
			name: "Appsets namespaces without Apps namespaces",
			argocdSpec: argoproj.ArgoCDSpec{
				ApplicationSet: &argoproj.ArgoCDApplicationSet{
					SourceNamespaces: []string{"foo"},
					SCMProviders:     []string{"github.com"},
				},
				SourceNamespaces: []string{},
			},
			expectedCmd:    []string{"--allowed-scm-providers", "github.com"},
			notExpectedCmd: []string{"--applicationset-namespaces", "foo"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			a := makeTestArgoCD()

			os.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", a.Namespace)
			defer os.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", "")

			ns1 := v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			}
			ns2 := v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bar",
				},
			}
			resObjs := []client.Object{a, &ns1, &ns2}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch)
			cm := newConfigMapWithName(getCAConfigMapName(a), a)
			r.Client.Create(context.Background(), cm, &client.CreateOptions{})

			a.Spec = test.argocdSpec

			sa := corev1.ServiceAccount{}
			assert.NoError(t, r.reconcileApplicationSetDeployment(a, &sa))

			deployment := &appsv1.Deployment{}
			assert.NoError(t, r.Client.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      "argocd-applicationset-controller",
					Namespace: a.Namespace,
				},
				deployment))

			cmds := deployment.Spec.Template.Spec.Containers[0].Command
			for _, c := range test.expectedCmd {
				assert.True(t, contains(cmds, c))
			}
			for _, c := range test.notExpectedCmd {
				assert.False(t, contains(cmds, c))
			}
		})
	}
}

func TestReconcileApplicationSet_ServiceAccount(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
		Enabled: boolPtr(true),
	}

	retSa, err := r.reconcileApplicationSetServiceAccount(a)
	assert.NoError(t, err)

	sa := &corev1.ServiceAccount{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		sa))

	assert.Equal(t, sa.Name, retSa.Name)

	appsetAssertExpectedLabels(t, &sa.ObjectMeta)
}

// Test creation/cleanup of applicationset-controller clusterrole & clusterrolebinding
func TestReconcileApplicationSet_ClusterRBACCreationAndCleanup(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resName := "argocd-argocd-argocd-applicationset-controller"

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
		Enabled: boolPtr(true),
	}

	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa-name"}}

	// test: ArgoCD is not cluster-scoped, resources shouldn't be created
	role, err := r.reconcileApplicationSetClusterRole(a)
	assert.NoError(t, err)
	err = r.reconcileApplicationSetClusterRoleBinding(a, role, sa)
	assert.NoError(t, err)

	// clusterrole should not be created
	cr := &rbacv1.ClusterRole{}
	err = r.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: resName}, cr)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))

	// clusterrolebinding should not be created
	crb := &rbacv1.ClusterRoleBinding{}
	err = r.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: resName}, crb)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))

	// test: make ArgoCD cluster-scoped, resources should be created
	os.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", a.Namespace)

	role, err = r.reconcileApplicationSetClusterRole(a)
	assert.NoError(t, err)
	err = r.reconcileApplicationSetClusterRoleBinding(a, role, sa)
	assert.NoError(t, err)

	// clusterrole should be created
	cr = &rbacv1.ClusterRole{}
	err = r.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: resName}, cr)
	assert.NoError(t, err)

	// clusterrolebinding should be created
	crb = &rbacv1.ClusterRoleBinding{}
	err = r.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: resName}, crb)
	assert.NoError(t, err)
	assert.Equal(t, crb.RoleRef.Name, cr.Name)
	assert.Equal(t, crb.Subjects[0].Name, sa.Name)

	// test: make ArgoCD namespaced-scope, existing resources should be deleted
	os.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", "")
	role, err = r.reconcileApplicationSetClusterRole(a)
	assert.NoError(t, err)
	err = r.reconcileApplicationSetClusterRoleBinding(a, role, sa)
	assert.NoError(t, err)

	// clusterrole should not exists
	cr = &rbacv1.ClusterRole{}
	err = r.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: resName}, cr)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))

	// clusterrolebinding should not exists
	crb = &rbacv1.ClusterRoleBinding{}
	err = r.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: resName}, crb)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

// Test creation/cleanup of applicationset-controller role & rolebinding in source namespaces
// Appset resources are only created if target source ns is subset of apps source namespaces
func TestReconcileApplicationSet_SourceNamespacesRBACCreation(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name         string
		argoCDSpec   argoproj.ArgoCDSpec
		expectErr    bool
		existInNs    []string
		notExistInNs []string
	}{
		{
			name: "No appset & app source namespaces", // no resources should be created
			argoCDSpec: argoproj.ArgoCDSpec{
				ApplicationSet:   nil,
				SourceNamespaces: []string(nil),
			},
			expectErr: false,
		},
		{
			name: "appset source ns not subset of app source ns", // resources shouldn't be created in allowed namespaces
			argoCDSpec: argoproj.ArgoCDSpec{
				ApplicationSet: &argoproj.ArgoCDApplicationSet{
					SourceNamespaces: []string{"foo", "bar"},
				},
				SourceNamespaces: []string(nil),
			},
			expectErr:    false,
			existInNs:    []string{},
			notExistInNs: []string{"foo", "bar"},
		},
		{
			name: "appset source ns subset of app source ns ", // resources should be created is all appset ns
			argoCDSpec: argoproj.ArgoCDSpec{
				ApplicationSet: &argoproj.ArgoCDApplicationSet{
					SourceNamespaces: []string{"foo", "bar"},
				},
				SourceNamespaces: []string{"foo", "bar"},
			},
			expectErr:    false,
			existInNs:    []string{"foo", "bar"},
			notExistInNs: []string{},
		},
		{
			name: "appset source ns partial subset of app source ns ", // resources should be created only in ns part of app source ns
			argoCDSpec: argoproj.ArgoCDSpec{
				ApplicationSet: &argoproj.ArgoCDApplicationSet{
					SourceNamespaces: []string{"foo", "bar"},
				},
				SourceNamespaces: []string{"foo"},
			},
			expectErr:    false,
			existInNs:    []string{"foo"},
			notExistInNs: []string{"bar"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			a := makeTestArgoCD()
			allowClusterConfigNamespaces(t, a.Namespace)
			defer os.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", "")

			resObjs := []client.Object{a}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch)
			a.Spec = test.argoCDSpec

			for _, ns := range append(test.existInNs, test.notExistInNs...) {
				createNamespace(r, ns, "")
			}

			err := r.reconcileApplicationSetSourceNamespacesResources(a)
			if test.expectErr {
				assert.Error(t, err)
			}

			// resources for applicationset-controller should be created in target ns
			for _, ns := range test.existInNs {
				resName := getResourceNameForApplicationSetSourceNamespaces(a)

				role := &rbacv1.Role{}
				err = r.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: resName, Namespace: ns}, role)
				assert.NoError(t, err)

				roleBinding := &rbacv1.RoleBinding{}
				err = r.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: resName, Namespace: ns}, roleBinding)
				assert.NoError(t, err)
			}

			// appset tracker label should be added on the target namespace
			for _, ns := range test.existInNs {
				namespace := &v1.Namespace{}
				err = r.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: ns}, namespace)
				assert.NoError(t, err)
				val, found := namespace.Labels[common.ArgoCDApplicationSetManagedByClusterArgoCDLabel]
				assert.True(t, found)
				assert.Equal(t, a.Namespace, val)
			}

			// resources for applicationset-controller shouldn't be created in target ns
			for _, ns := range test.notExistInNs {
				resName := getResourceNameForApplicationSetSourceNamespaces(a)

				role := &rbacv1.Role{}
				err = r.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: resName, Namespace: ns}, role)
				assert.Error(t, err)
				assert.True(t, apierrors.IsNotFound(err))

				roleBinding := &rbacv1.RoleBinding{}
				err = r.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: resName, Namespace: ns}, roleBinding)
				assert.Error(t, err)
				assert.True(t, apierrors.IsNotFound(err))
			}

			// appset tracker label shouldn't be added on the target namespace
			for _, ns := range test.notExistInNs {
				namespace := &v1.Namespace{}
				err = r.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: ns}, namespace)
				assert.NoError(t, err)
				_, found := namespace.Labels[common.ArgoCDApplicationSetManagedByClusterArgoCDLabel]
				assert.False(t, found)
			}

		})
	}
}

func TestReconcileApplicationSet_Role(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
		Enabled: boolPtr(true),
	}

	roleRet, err := r.reconcileApplicationSetRole(a)
	assert.NoError(t, err)

	role := &rbacv1.Role{}
	assert.NoError(t, r.Client.Get(
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
		"leases",
	}

	foundResources := []string{}

	for _, rule := range role.Rules {
		for _, resource := range rule.Resources {
			foundResources = append(foundResources, resource)
		}
	}

	sort.Strings(expectedResources)
	sort.Strings(foundResources)

	assert.Equal(t, expectedResources, foundResources)
}

func TestReconcileApplicationSet_RoleBinding(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
		Enabled: boolPtr(true),
	}

	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "role-name"}}
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa-name"}}

	err := r.reconcileApplicationSetRoleBinding(a, role, sa)
	assert.NoError(t, err)

	roleBinding := &rbacv1.RoleBinding{}
	assert.NoError(t, r.Client.Get(
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

func setProxyEnvVars(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "https://example.com")
	t.Setenv("HTTP_PROXY", "http://example.com")
	t.Setenv("NO_PROXY", ".cluster.local")
}

func TestReconcileApplicationSet_Service(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	s := newServiceWithSuffix(common.ApplicationSetServiceNameSuffix, common.ApplicationSetServiceNameSuffix, a)

	assert.NoError(t, r.reconcileApplicationSetService(a))
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s))
}

func TestArgoCDApplicationSetCommand(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	baseCommand := []string{
		"entrypoint.sh",
		"argocd-applicationset-controller",
		"--argocd-repo-server",
		"argocd-repo-server.argocd.svc.cluster.local:8081",
		"--loglevel",
		"info",
	}

	// When a single command argument is passed
	a.Spec.ApplicationSet.ExtraCommandArgs = []string{
		"--foo",
		"bar",
	}

	wantCmd := []string{
		"entrypoint.sh",
		"argocd-applicationset-controller",
		"--loglevel",
		"info",
		"--argocd-repo-server",
		"foo.scv.cluster.local:6379",
	}

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.reconcileApplicationSetController(a))

	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	cmd := append(baseCommand, "--foo", "bar")
	assert.Equal(t, cmd, deployment.Spec.Template.Spec.Containers[0].Command)

	// When multiple command arguments are passed
	a.Spec.ApplicationSet.ExtraCommandArgs = []string{
		"--foo",
		"bar",
		"--ping",
		"pong",
		"test",
	}

	assert.NoError(t, r.reconcileApplicationSetController(a))
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	cmd = append(cmd, "--ping", "pong", "test")
	assert.Equal(t, cmd, deployment.Spec.Template.Spec.Containers[0].Command)

	// When one of the ExtraCommandArgs already exists in cmd with same or different value
	a.Spec.ApplicationSet.ExtraCommandArgs = []string{
		"--argocd-repo-server",
		"foo.scv.cluster.local:6379",
	}

	assert.NoError(t, r.reconcileApplicationSetController(a))
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Equal(t, wantCmd, deployment.Spec.Template.Spec.Containers[0].Command)

	// Remove all the command arguments that were added.
	a.Spec.ApplicationSet.ExtraCommandArgs = []string{}

	assert.NoError(t, r.reconcileApplicationSetController(a))
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Equal(t, baseCommand, deployment.Spec.Template.Spec.Containers[0].Command)

	// When ExtraCommandArgs contains a non-duplicate argument along with a duplicate
	a.Spec.ApplicationSet.ExtraCommandArgs = []string{
		"--foo",
		"bar",
		"--ping",
		"pong",
		"test",
		"--newarg", // Non-duplicate argument
		"newvalue",
		"--newarg", // Duplicate argument passing at once
		"newvalue",
		"--arg1", // flag with 2 different vales
		"value1",
		"--arg1",
		"value2",
	}

	assert.NoError(t, r.reconcileApplicationSetController(a))
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	// Non-duplicate argument "--newarg" should be added, duplicate "--newarg" which is added twice is ignored
	cmd = append(cmd, "--newarg", "newvalue", "--arg1", "value1", "--arg1", "value2")
	assert.Equal(t, cmd, deployment.Spec.Template.Spec.Containers[0].Command)
}

func TestArgoCDApplicationSetEnv(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	defaultEnv := []corev1.EnvVar{
		{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "",
					FieldPath:  "metadata.namespace",
				},
			},
		},
	}

	// Pass an environment variable using Argo CD CR.
	customEnv := []corev1.EnvVar{
		{
			Name:  "foo",
			Value: "bar",
		},
	}
	a.Spec.ApplicationSet.Env = customEnv

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.reconcileApplicationSetController(a))

	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	expectedEnv := append(defaultEnv, customEnv...)
	assert.Equal(t, expectedEnv, deployment.Spec.Template.Spec.Containers[0].Env)

	// Remove all the env vars that were added.
	a.Spec.ApplicationSet.Env = []corev1.EnvVar{}

	assert.NoError(t, r.reconcileApplicationSetController(a))
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Equal(t, defaultEnv, deployment.Spec.Template.Spec.Containers[0].Env)
}

func TestArgoCDApplicationSet_getApplicationSetSourceNamespaces(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name        string
		appSetField *argoproj.ArgoCDApplicationSet
		expected    []string
	}{
		{
			name:        "Appsets not enabled",
			appSetField: nil,
			expected:    []string(nil),
		},
		{
			name: "No appset source namespaces",
			appSetField: &argoproj.ArgoCDApplicationSet{
				Enabled: boolPtr(true),
			},
			expected: []string(nil),
		},
		{
			name: "Appset source namespaces",
			appSetField: &argoproj.ArgoCDApplicationSet{
				SourceNamespaces: []string{"foo", "bar"},
			},
			expected: []string{"foo", "bar"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			a := makeTestArgoCD()
			resObjs := []client.Object{a}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch)
			cm := newConfigMapWithName(getCAConfigMapName(a), a)
			r.Client.Create(context.Background(), cm, &client.CreateOptions{})

			a.Spec.ApplicationSet = test.appSetField

			actual := r.getApplicationSetSourceNamespaces(a)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestArgoCDApplicationSet_setManagedApplicationSetSourceNamespaces(t *testing.T) {
	a := makeTestArgoCD()
	ns1 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace-1",
			Labels: map[string]string{
				common.ArgoCDApplicationSetManagedByClusterArgoCDLabel: testNamespace,
			},
		},
	}
	ns2 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace-2",
		},
	}

	resObjs := []client.Object{a, &ns1, &ns2}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	err := r.setManagedApplicationSetSourceNamespaces(a)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(r.ManagedApplicationSetSourceNamespaces))
	assert.Contains(t, r.ManagedApplicationSetSourceNamespaces, "test-namespace-1")
}

func TestArgoCDApplicationSet_removeUnmanagedApplicationSetSourceNamespaceResources(t *testing.T) {
	ns1 := "foo"
	ns2 := "bar"
	a := makeTestArgoCD()

	allowClusterConfigNamespaces(t, a.Namespace)
	defer os.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", "")

	a.Spec = argoproj.ArgoCDSpec{
		SourceNamespaces: []string{ns1, ns2},
		ApplicationSet: &argoproj.ArgoCDApplicationSet{
			SourceNamespaces: []string{ns1, ns2},
		},
	}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	createNamespace(r, ns1, "")
	createNamespace(r, ns2, "")

	// create resources
	err := r.reconcileApplicationSetSourceNamespacesResources(a)
	assert.NoError(t, err)

	// remove appset ns
	a.Spec = argoproj.ArgoCDSpec{
		SourceNamespaces: []string{ns2},
		ApplicationSet: &argoproj.ArgoCDApplicationSet{
			SourceNamespaces: []string{ns1, ns2},
		},
	}

	// clean up unmanaged namespaces resources
	err = r.removeUnmanagedApplicationSetSourceNamespaceResources(a)
	assert.NoError(t, err)

	// resources shouldn't exist in ns1
	resName := getResourceNameForApplicationSetSourceNamespaces(a)

	role := &rbacv1.Role{}
	err = r.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: resName, Namespace: ns1}, role)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))

	roleBinding := &rbacv1.RoleBinding{}
	err = r.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: resName, Namespace: ns1}, roleBinding)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))

	// appset tracking label should be removed
	namespace := &v1.Namespace{}
	err = r.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: ns1}, namespace)
	assert.NoError(t, err)
	_, found := namespace.Labels[common.ArgoCDApplicationSetManagedByClusterArgoCDLabel]
	assert.False(t, found)

	// resources in ns2 shouldn't be touched

	role = &rbacv1.Role{}
	err = r.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: resName, Namespace: ns2}, role)
	assert.NoError(t, err)

	roleBinding = &rbacv1.RoleBinding{}
	err = r.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: resName, Namespace: ns2}, roleBinding)
	assert.NoError(t, err)

	namespace = &v1.Namespace{}
	err = r.Client.Get(context.TODO(), cntrlClient.ObjectKey{Name: ns2}, namespace)
	assert.NoError(t, err)
	val, found := namespace.Labels[common.ArgoCDApplicationSetManagedByClusterArgoCDLabel]
	assert.True(t, found)
	assert.Equal(t, a.Namespace, val)
}

func TestReconcileApplicationSetSourceNamespacesResources_NonClusterConfigNamespace(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name                      string
		clusterConfigNamespaces   string
		expectInManagedNamespaces bool
		expectedManagedNamespaces []string
	}{
		{
			name:                      "ARGOCD_CLUSTER_CONFIG_NAMESPACES contains namespace",
			clusterConfigNamespaces:   "argocd",
			expectInManagedNamespaces: true,
			expectedManagedNamespaces: []string{"foo", "bar"},
		},
		{
			name:                      "ARGOCD_CLUSTER_CONFIG_NAMESPACES does not contain namespace",
			clusterConfigNamespaces:   "",
			expectInManagedNamespaces: false,
			expectedManagedNamespaces: []string{},
		},
		{
			name:                      "ARGOCD_CLUSTER_CONFIG_NAMESPACES contains different namespace",
			clusterConfigNamespaces:   "different-namespace",
			expectInManagedNamespaces: false,
			expectedManagedNamespaces: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := makeTestArgoCD()
			a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
				SourceNamespaces: []string{"foo", "bar"},
			}
			a.Spec.SourceNamespaces = []string{"foo", "bar"}

			// Set ARGOCD_CLUSTER_CONFIG_NAMESPACES based on test case
			os.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", test.clusterConfigNamespaces)
			defer os.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", "")

			resObjs := []client.Object{a}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch)

			// Initialize ManagedApplicationSetSourceNamespaces as empty map
			r.ManagedApplicationSetSourceNamespaces = make(map[string]string)

			// Create source namespaces so the function can process them
			for _, ns := range []string{"foo", "bar"} {
				err := createNamespace(r, ns, "")
				assert.NoError(t, err)
			}

			// Verify IsNamespaceClusterConfigNamespace returns expected value
			if test.expectInManagedNamespaces {
				assert.True(t, argoutil.IsNamespaceClusterConfigNamespace(a.Namespace))
			} else {
				assert.False(t, argoutil.IsNamespaceClusterConfigNamespace(a.Namespace))
			}

			err := r.reconcileApplicationSetSourceNamespacesResources(a)
			assert.NoError(t, err)

			// Verify ManagedApplicationSetSourceNamespaces contains expected namespaces
			if test.expectInManagedNamespaces {
				assert.Equal(t, len(test.expectedManagedNamespaces), len(r.ManagedApplicationSetSourceNamespaces))
				for _, ns := range test.expectedManagedNamespaces {
					assert.Contains(t, r.ManagedApplicationSetSourceNamespaces, ns)
				}
			} else {
				assert.Empty(t, r.ManagedApplicationSetSourceNamespaces)
			}
		})
	}
}
