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
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

func applicationSetDefaultVolumes() []v1.Volume {
	repoVolumes := repoServerDefaultVolumes()
	ignoredVolumes := map[string]bool{
		"var-files":                           true,
		"plugins":                             true,
		"argocd-repo-server-tls":              true,
		common.ArgoCDRedisServerTLSSecretName: true,
	}
	volumes := make([]v1.Volume, len(repoVolumes)-len(ignoredVolumes))
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	sa := v1.ServiceAccount{}

	assert.NoError(t, r.reconcileApplicationSetDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	// Ensure the created Deployment has the expected properties
	checkExpectedDeploymentValues(t, r, deployment, &sa, nil, nil, a)
}

func checkExpectedDeploymentValues(t *testing.T, r *ReconcileArgoCD, deployment *appsv1.Deployment, sa *v1.ServiceAccount, extraVolumes *[]v1.Volume, extraVolumeMounts *[]v1.VolumeMount, a *argoproj.ArgoCD) {
	assert.Equal(t, deployment.Spec.Template.Spec.ServiceAccountName, sa.Name)
	appsetAssertExpectedLabels(t, &deployment.ObjectMeta)

	want := []v1.Container{r.applicationSetContainer(a, false)}

	if diff := cmp.Diff(want, deployment.Spec.Template.Spec.Containers); diff != "" {
		t.Fatalf("failed to reconcile applicationset-controller deployment containers:\n%s", diff)
	}

	volumes := []v1.Volume{
		{
			Name: "ssh-known-hosts",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: common.ArgoCDKnownHostsConfigMapName,
					},
				},
			},
		},
		{
			Name: "tls-certs",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: common.ArgoCDTLSCertsConfigMapName,
					},
				},
			},
		},
		{
			Name: "gpg-keys",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: common.ArgoCDGPGKeysConfigMapName,
					},
				},
			},
		},
		{
			Name: "gpg-keyring",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "tmp",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		},
	}

	if a.Spec.ApplicationSet.SCMRootCAConfigMap != "" {

		exists, err := argoutil.IsObjectFound(r.Client, a.Namespace, common.ArgoCDAppSetGitlabSCMTLSCertsConfigMapName, a)
		assert.Nil(t, err)
		if exists {
			volumes = append(volumes, v1.Volume{
				Name: "appset-gitlab-scm-tls-cert",
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: common.ArgoCDAppSetGitlabSCMTLSCertsConfigMapName,
						},
					},
				},
			})
		}
	}

	if extraVolumes != nil {
		volumes = append(volumes, *extraVolumes...)
	}

	if diff := cmp.Diff(volumes, deployment.Spec.Template.Spec.Volumes); diff != "" {
		t.Fatalf("failed to reconcile applicationset-controller deployment volumes:\n%s", diff)
	}

	volumeMounts := []v1.VolumeMount{
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	sa := v1.ServiceAccount{}

	err := r.reconcileApplicationSetDeployment(a, &sa)
	assert.NoError(t, err)

	want := []v1.EnvVar{
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
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
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
	err = r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment)
	assert.NoError(t, err)

	if diff := cmp.Diff(want, deployment.Spec.Template.Spec.Containers[0].Env); diff != "" {
		t.Fatalf("failed to reconcile applicationset-controller deployment containers:\n%s", diff)
	}

}

func TestReconcileApplicationSetVolumes(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	extraVolumes := []v1.Volume{
		{
			Name: "example-volume",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		},
	}

	extraVolumeMounts := []v1.VolumeMount{
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	sa := v1.ServiceAccount{}

	// Reconcile the ApplicationSet deployment
	assert.NoError(t, r.reconcileApplicationSetDeployment(a, &sa))

	// Get the deployment after reconciliation
	deployment := &appsv1.Deployment{}
	err := r.Get(
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
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	sa := v1.ServiceAccount{}

	assert.NoError(t, r.reconcileApplicationSetDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Get(
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	sa := v1.ServiceAccount{}

	assert.NoError(t, r.reconcileApplicationSetDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Equal(t, deployment.Spec.Template.Spec.ServiceAccountName, sa.Name)
	appsetAssertExpectedLabels(t, &deployment.ObjectMeta)

	containerWant := []v1.Container{r.applicationSetContainer(a, false)}

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
			envVars:                map[string]string{common.ArgoCDImageEnvName: "docker.io/library/ubuntu:latest"},
			expectedContainerImage: "docker.io/library/ubuntu:latest",
		},

		{
			name: "env var should not override spec fields",
			appSetField: &argoproj.ArgoCDApplicationSet{
				Image:   "custom-image",
				Version: "custom-version",
			},
			envVars:                map[string]string{common.ArgoCDImageEnvName: "docker.io/library/ubuntu:latest"},
			expectedContainerImage: "custom-image:custom-version",
		},
		{
			name: "ensure scm tls cert mount is present",
			appSetField: &argoproj.ArgoCDApplicationSet{
				SCMRootCAConfigMap: "test-scm-tls-mount",
			},
			envVars:                map[string]string{common.ArgoCDImageEnvName: "docker.io/library/ubuntu:latest"},
			expectedContainerImage: "docker.io/library/ubuntu:latest",
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
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
			cm := newConfigMapWithName(getCAConfigMapName(a), a)
			err := r.Create(context.Background(), cm, &client.CreateOptions{})
			assert.NoError(t, err)

			if test.argocdField.Image != "" {
				a.Spec.Image = test.argocdField.Image
				a.Spec.Version = test.argocdField.Version
			}

			a.Spec.ApplicationSet = test.appSetField

			sa := v1.ServiceAccount{}
			assert.NoError(t, r.reconcileApplicationSetDeployment(a, &sa))

			deployment := &appsv1.Deployment{}
			assert.NoError(t, r.Get(
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

func TestReconcileApplicationSet_ServiceAccount(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
		Enabled: boolPtr(true),
	}

	retSa, err := r.reconcileApplicationSetServiceAccount(a)
	assert.NoError(t, err)

	sa := &v1.ServiceAccount{}
	assert.NoError(t, r.Get(
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
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
		Enabled: boolPtr(true),
	}

	roleRet, err := r.reconcileApplicationSetRole(a)
	assert.NoError(t, err)

	role := &rbacv1.Role{}
	assert.NoError(t, r.Get(
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
		foundResources = append(foundResources, rule.Resources...)
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
		Enabled: boolPtr(true),
	}

	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "role-name"}}
	sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa-name"}}

	err := r.reconcileApplicationSetRoleBinding(a, role, sa)
	assert.NoError(t, err)

	roleBinding := &rbacv1.RoleBinding{}
	assert.NoError(t, r.Get(
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
	assert.Equal(t, meta.Labels["app.kubernetes.io/part-of"], "argocd")
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	s := newServiceWithSuffix(common.ApplicationSetServiceNameSuffix, common.ApplicationSetServiceNameSuffix, a)

	assert.NoError(t, r.reconcileApplicationSetService(a))
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s))
}

func TestReconcileApplicationSet_ServiceWithLongName(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	// Create ArgoCD with a very long name that will trigger truncation
	longName := "argocd-long-name-for-route-testiiiiiiiiiiiiiiiiiiiiiiiing"
	a := makeTestArgoCD()
	a.Name = longName
	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	// Test ApplicationSet Service reconciliation
	err := r.reconcileApplicationSetService(a)
	assert.NoError(t, err)

	// Get the created service
	serviceList := &v1.ServiceList{}
	err = r.List(context.TODO(), serviceList, client.InNamespace(a.Namespace))
	assert.NoError(t, err)

	var applicationSetService *v1.Service
	for i := range serviceList.Items {
		if serviceList.Items[i].Labels[common.ArgoCDKeyComponent] == common.ApplicationSetServiceNameSuffix {
			applicationSetService = &serviceList.Items[i]
			break
		}
	}
	assert.NotNil(t, applicationSetService, "ApplicationSet service should exist")

	// Verify that the service name is truncated and within limits
	assert.LessOrEqual(t, len(applicationSetService.Name), 63)
	assert.Contains(t, applicationSetService.Name, "applicationset-controller")

	// Verify that the service selector uses the component name (our fix)
	expectedComponentName := nameWithSuffix(common.ApplicationSetServiceNameSuffix, a)
	assert.Equal(t, expectedComponentName, applicationSetService.Spec.Selector[common.ArgoCDKeyName])

	// Verify that the suffix "applicationset-controller" is not truncated in the selector
	assert.Contains(t, applicationSetService.Spec.Selector[common.ArgoCDKeyName], "applicationset-controller")

	// Verify that the selector name is truncated and within limits
	selectorName := applicationSetService.Spec.Selector[common.ArgoCDKeyName]
	assert.LessOrEqual(t, len(selectorName), 63)

	// Verify that the component label is set correctly
	assert.Equal(t, common.ApplicationSetServiceNameSuffix, applicationSetService.Labels[common.ArgoCDKeyComponent])
}

func TestArgoCDApplicationSetCommand(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	baseCommand := []string{
		"entrypoint.sh",
		"argocd-applicationset-controller",
		"--argocd-repo-server",
		"argocd-repo-server.argocd.svc.cluster.local:8081",
		"--loglevel",
		"info",
		"--logformat",
		"text",
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
		"--logformat",
		"text",
		"--argocd-repo-server",
		"foo.scv.cluster.local:6379",
	}

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.reconcileApplicationSetController(a))

	assert.NoError(t, r.Get(
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
	assert.NoError(t, r.Get(
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
	assert.NoError(t, r.Get(
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
	assert.NoError(t, r.Get(
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
	assert.NoError(t, r.Get(
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	defaultEnv := []v1.EnvVar{
		{
			Name: "NAMESPACE",
			ValueFrom: &v1.EnvVarSource{
				FieldRef: &v1.ObjectFieldSelector{
					APIVersion: "",
					FieldPath:  "metadata.namespace",
				},
			},
		},
	}

	// Pass an environment variable using Argo CD CR.
	customEnv := []v1.EnvVar{
		{
			Name:  "foo",
			Value: "bar",
		},
	}
	a.Spec.ApplicationSet.Env = customEnv

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.reconcileApplicationSetController(a))

	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	expectedEnv := append(defaultEnv, customEnv...)
	assert.Equal(t, expectedEnv, deployment.Spec.Template.Spec.Containers[0].Env)

	// Remove all the env vars that were added.
	a.Spec.ApplicationSet.Env = []v1.EnvVar{}

	assert.NoError(t, r.reconcileApplicationSetController(a))
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Equal(t, defaultEnv, deployment.Spec.Template.Spec.Containers[0].Env)
}

func TestGetApplicationSetContainerImage(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	// when env var is set and spec fields are not set, env var should be returned
	cr := argoproj.ArgoCD{}
	cr.Spec = argoproj.ArgoCDSpec{}
	cr.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}
	os.Setenv(common.ArgoCDImageEnvName, "testingimage@sha:123456")
	out := getApplicationSetContainerImage(&cr)
	assert.Equal(t, "testingimage@sha:123456", out)

	// when env var is set and also spec image and version fields are set, spec fields should be returned
	cr.Spec.Image = "customimage"
	cr.Spec.Version = "sha256:7e0aa2f42232f6b2f0a9d5f98b2e3a9a6b8c9b7f3a4c1d2e5f6a7b8c9d0e1f2a"
	os.Setenv(common.ArgoCDImageEnvName, "quay.io/project/registry@sha256:7e0aa2f42232f6b2f0a9d5f98b2e3a9a6b8c9b7f3a4c1d2e5f6a7b8c9d0e1f2a")
	out = getApplicationSetContainerImage(&cr)
	assert.Equal(t, "customimage@sha256:7e0aa2f42232f6b2f0a9d5f98b2e3a9a6b8c9b7f3a4c1d2e5f6a7b8c9d0e1f2a", out)

	// when spec.image and spec.applicationset.image is passed and also env is passed, container level image should take priority
	cr.Spec.Image = "customimage"
	cr.Spec.Version = "sha256:7e0aa2f42232f6b2f0a9d5f98b2e3a9a6b8c9b7f3a4c1d2e5f6a7b8c9d0e1f2a"
	cr.Spec.ApplicationSet.Image = "containerImage"
	cr.Spec.ApplicationSet.Version = "sha256:7e0aa2f42232f6b2f0a9d5f98b2e3a9a6b8c9b7f3a4c1d2e5f6a7b8c9d0e1f2b"
	os.Setenv(common.ArgoCDImageEnvName, "quay.io/project/registry@sha256:7e0aa2f42232f6b2f0a9d5f98b2e3a9a6b8c9b7f3a4c1d2e5f6a7b8c9d0e1f2c")
	out = getApplicationSetContainerImage(&cr)
	assert.Equal(t, "containerImage@sha256:7e0aa2f42232f6b2f0a9d5f98b2e3a9a6b8c9b7f3a4c1d2e5f6a7b8c9d0e1f2b", out)

	// when env var is set and also spec version field is set but image field is not set, should return env var image with spec version
	cr.Spec.Image = ""
	cr.Spec.Version = "sha256:7e0aa2f42232f6b2f0a9d5f98b2e3a9a6b8c9b7f3a4c1d2e5f6a7b8c9d0e1f2a"
	cr.Spec.ApplicationSet.Image = ""
	cr.Spec.ApplicationSet.Version = ""
	os.Setenv(common.ArgoCDImageEnvName, "quay.io/project/registry@sha256:7e0aa2f42232f6b2f0a9d5f98b2e3a9a6b8c9b7f3a4c1d2e5f6a7b8c9d0e1f2b")
	out = getApplicationSetContainerImage(&cr)
	assert.Equal(t, "quay.io/project/registry@sha256:7e0aa2f42232f6b2f0a9d5f98b2e3a9a6b8c9b7f3a4c1d2e5f6a7b8c9d0e1f2a", out)

	// when env var in wrong format is set and also spec version field is set but image field is not set
	cr.Spec.Image = ""
	cr.Spec.Version = "sha256:7e0aa2f42232f6b2f0a9d5f98b2e3a9a6b8c9b7f3a4c1d2e5f6a7b8c9d0e1f2a"
	os.Setenv(common.ArgoCDImageEnvName, "quay.io/project/registry:latest")
	out = getApplicationSetContainerImage(&cr)
	assert.Equal(t, "quay.io/project/registry@sha256:7e0aa2f42232f6b2f0a9d5f98b2e3a9a6b8c9b7f3a4c1d2e5f6a7b8c9d0e1f2a", out)

	cr.Spec.Image = ""
	cr.Spec.Version = ""
	os.Setenv(common.ArgoCDImageEnvName, "quay.io/project/registry:latest@sha256:7e0aa2f42232f6b2f0a9d5f98b2e3a9a6b8c9b7f3a4c1d2e5f6a7b8c9d0e1f2a")
	out = getApplicationSetContainerImage(&cr)
	assert.Equal(t, "quay.io/project/registry:latest@sha256:7e0aa2f42232f6b2f0a9d5f98b2e3a9a6b8c9b7f3a4c1d2e5f6a7b8c9d0e1f2a", out)

	cr.Spec.Image = ""
	cr.Spec.Version = ""
	os.Setenv(common.ArgoCDImageEnvName, "docker.io/library/ubuntu")
	out = getApplicationSetContainerImage(&cr)
	assert.Equal(t, "docker.io/library/ubuntu", out)

	cr.Spec.Image = ""
	cr.Spec.Version = "v0.0.1"
	os.Setenv(common.ArgoCDImageEnvName, "quay.io/project/registry:latest@sha256:7e0aa2f42232f6b2f0a9d5f98b2e3a9a6b8c9b7f3a4c1d2e5f6a7b8c9d0e1f2a")
	out = getApplicationSetContainerImage(&cr)
	assert.Equal(t, "quay.io/project/registry:v0.0.1", out)

	cr.Spec.Image = ""
	cr.Spec.Version = "v0.0.1"
	os.Setenv(common.ArgoCDImageEnvName, "docker.io/library/ubuntu")
	out = getApplicationSetContainerImage(&cr)
	assert.Equal(t, "docker.io/library/ubuntu:v0.0.1", out)

	cr.Spec.Image = ""
	cr.Spec.Version = "v0.0.1"
	os.Setenv(common.ArgoCDImageEnvName, "ubuntu")
	out = getApplicationSetContainerImage(&cr)
	assert.Equal(t, "ubuntu:v0.0.1", out)

	// when env var is not set and spec image and version fields are not set, default image should be returned
	os.Setenv(common.ArgoCDImageEnvName, "")
	cr.Spec.Image = ""
	cr.Spec.Version = ""
	out = getApplicationSetContainerImage(&cr)
	assert.Equal(t, "quay.io/argoproj/argocd@"+common.ArgoCDDefaultArgoVersion, out)

	// when env var is not set and spec image and version fields are set, spec fields should be returned
	cr.Spec.Image = "customimage"
	cr.Spec.Version = "sha256:1234567890abcdef"
	os.Setenv(common.ArgoCDImageEnvName, "")
	out = getApplicationSetContainerImage(&cr)
	assert.Equal(t, "customimage@sha256:1234567890abcdef", out)

	// when env var is not set and spec version field is set but image field is not set, should return default image with spec version tag
	cr.Spec.Image = ""
	cr.Spec.Version = "customversion"
	os.Setenv(common.ArgoCDImageEnvName, "")
	out = getApplicationSetContainerImage(&cr)
	assert.Equal(t, "quay.io/argoproj/argocd:customversion", out)

	// when env var is not set and spec image field is set but version field is not set, should return spec image with default tag
	cr.Spec.Image = "customimage"
	cr.Spec.Version = ""
	os.Setenv(common.ArgoCDImageEnvName, "")
	out = getApplicationSetContainerImage(&cr)
	assert.Equal(t, "customimage@"+common.ArgoCDDefaultArgoVersion, out)
}
