// Copyright 2025 ArgoCD Operator Developers
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

package argocdagent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
)

// Helper function to create a test deployment
func makeTestDeployment(cr *argoproj.ArgoCD) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
		Spec: buildPrincipalSpec(testCompName, generateAgentResourceName(cr.Name, testCompName), cr),
	}
}

// Helper function to create a test deployment with custom image
func makeTestDeploymentWithCustomImage(cr *argoproj.ArgoCD, customImage string) *appsv1.Deployment {
	deployment := makeTestDeployment(cr)
	deployment.Spec.Template.Spec.Containers[0].Image = customImage
	return deployment
}

// Helper function to create ArgoCD with custom principal image
func withPrincipalImage(image string) argoCDOpt {
	return func(a *argoproj.ArgoCD) {
		if a.Spec.ArgoCDAgent == nil {
			a.Spec.ArgoCDAgent = &argoproj.ArgoCDAgentSpec{}
		}
		if a.Spec.ArgoCDAgent.Principal == nil {
			a.Spec.ArgoCDAgent.Principal = &argoproj.PrincipalSpec{}
		}
		a.Spec.ArgoCDAgent.Principal.Image = image
	}
}

// TestReconcilePrincipalDeployment tests

func TestReconcilePrincipalDeployment_DeploymentDoesNotExist_PrincipalDisabled(t *testing.T) {
	// Test case: Deployment doesn't exist and principal is disabled
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestArgoCD(withPrincipalEnabled(false))
	saName := generateAgentResourceName(cr.Name, testCompName)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, saName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment was not created
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalDeployment_DeploymentDoesNotExist_PrincipalEnabled(t *testing.T) {
	// Test case: Deployment doesn't exist and principal is enabled
	// Expected behavior: Should create the Deployment with expected spec

	cr := makeTestArgoCD(withPrincipalEnabled(true))
	saName := generateAgentResourceName(cr.Name, testCompName)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, saName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment was created
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.NoError(t, err)

	// Verify Deployment has expected metadata
	assert.Equal(t, generateAgentResourceName(cr.Name, testCompName), deployment.Name)
	assert.Equal(t, cr.Namespace, deployment.Namespace)
	assert.Equal(t, buildLabelsForAgentPrincipal(cr.Name, testCompName), deployment.Labels)

	// Verify Deployment has expected spec
	expectedSpec := buildPrincipalSpec(testCompName, saName, cr)
	assert.Equal(t, expectedSpec.Selector, deployment.Spec.Selector)
	assert.Equal(t, expectedSpec.Template.Labels, deployment.Spec.Template.Labels)
	assert.Equal(t, expectedSpec.Template.Spec.ServiceAccountName, deployment.Spec.Template.Spec.ServiceAccountName)

	// Verify container configuration
	assert.Len(t, deployment.Spec.Template.Spec.Containers, 1)
	container := deployment.Spec.Template.Spec.Containers[0]
	assert.Equal(t, generateAgentResourceName(cr.Name, testCompName), container.Name)
	assert.Equal(t, buildPrincipalImage(cr), container.Image)
	assert.Equal(t, buildArgs(testCompName), container.Args)
	assert.Equal(t, buildPrincipalContainerEnv(cr), container.Env)
	assert.Equal(t, buildSecurityContext(), container.SecurityContext)
	assert.Equal(t, buildPorts(testCompName), container.Ports)

	// Verify owner reference is set
	assert.Len(t, deployment.OwnerReferences, 1)
	assert.Equal(t, cr.Name, deployment.OwnerReferences[0].Name)
	assert.Equal(t, "ArgoCD", deployment.OwnerReferences[0].Kind)
}

func TestReconcilePrincipalDeployment_DeploymentExists_PrincipalDisabled(t *testing.T) {
	// Test case: Deployment exists and principal is disabled
	// Expected behavior: Should delete the Deployment

	cr := makeTestArgoCD(withPrincipalEnabled(false))
	saName := generateAgentResourceName(cr.Name, testCompName)

	// Create existing Deployment
	existingDeployment := makeTestDeployment(cr)

	resObjs := []client.Object{cr, existingDeployment}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, saName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment was deleted
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalDeployment_DeploymentExists_PrincipalEnabled_NoChanges(t *testing.T) {
	// Test case: Deployment exists, principal is enabled, and no changes are needed
	// Expected behavior: Should not update the Deployment

	cr := makeTestArgoCD(withPrincipalEnabled(true))
	saName := generateAgentResourceName(cr.Name, testCompName)

	// Create existing Deployment with correct spec
	existingDeployment := makeTestDeployment(cr)

	resObjs := []client.Object{cr, existingDeployment}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, saName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment still exists with same spec
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.NoError(t, err)
	assert.Equal(t, buildPrincipalImage(cr), deployment.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, saName, deployment.Spec.Template.Spec.ServiceAccountName)
}

func TestReconcilePrincipalDeployment_DeploymentExists_PrincipalEnabled_ImageChanged(t *testing.T) {
	// Test case: Deployment exists, principal is enabled, but image has changed
	// Expected behavior: Should update the Deployment with new image

	cr := makeTestArgoCD(withPrincipalEnabled(true), withPrincipalImage("quay.io/argoproj/argocd-agent:v2"))
	saName := generateAgentResourceName(cr.Name, testCompName)

	// Create existing Deployment with old image
	existingDeployment := makeTestDeploymentWithCustomImage(cr, "quay.io/argoproj/argocd-agent:v1")

	resObjs := []client.Object{cr, existingDeployment}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, saName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment was updated with new image
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.NoError(t, err)
	assert.Equal(t, "quay.io/argoproj/argocd-agent:v2", deployment.Spec.Template.Spec.Containers[0].Image)
}

func TestReconcilePrincipalDeployment_DeploymentExists_PrincipalEnabled_ServiceAccountChanged(t *testing.T) {
	// Test case: Deployment exists, principal is enabled, but service account has changed
	// Expected behavior: Should update the Deployment with new service account

	cr := makeTestArgoCD(withPrincipalEnabled(true))
	oldSAName := "old-service-account"
	newSAName := "new-service-account"

	// Create existing Deployment with old service account
	existingDeployment := makeTestDeployment(cr)
	existingDeployment.Spec.Template.Spec.ServiceAccountName = oldSAName

	resObjs := []client.Object{cr, existingDeployment}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, newSAName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment was updated with new service account
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.NoError(t, err)
	assert.Equal(t, newSAName, deployment.Spec.Template.Spec.ServiceAccountName)
}

func TestReconcilePrincipalDeployment_DeploymentExists_PrincipalNotSet(t *testing.T) {
	// Test case: Deployment exists but principal is not set (nil)
	// Expected behavior: Should delete the Deployment

	cr := makeTestArgoCD() // No principal configuration
	saName := generateAgentResourceName(cr.Name, testCompName)

	// Create existing Deployment
	existingDeployment := makeTestDeployment(cr)

	resObjs := []client.Object{cr, existingDeployment}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, saName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment was deleted
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalDeployment_DeploymentDoesNotExist_AgentNotSet(t *testing.T) {
	// Test case: Deployment doesn't exist and ArgoCDAgent is not set (nil)
	// Expected behavior: Should do nothing since principal is effectively disabled

	cr := makeTestArgoCD() // No agent configuration
	saName := generateAgentResourceName(cr.Name, testCompName)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, saName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment was not created
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalDeployment_DeploymentExists_AgentNotSet(t *testing.T) {
	// Test case: Deployment exists but ArgoCDAgent is not set (nil)
	// Expected behavior: Should delete the Deployment

	cr := makeTestArgoCD() // No agent configuration
	saName := generateAgentResourceName(cr.Name, testCompName)

	// Create existing Deployment
	existingDeployment := makeTestDeployment(cr)

	resObjs := []client.Object{cr, existingDeployment}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, saName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment was deleted
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalDeployment_VerifyDeploymentSpec(t *testing.T) {
	// Test case: Verify the deployment spec has correct configuration
	// Expected behavior: Should create deployment with correct security context, ports, etc.

	cr := makeTestArgoCD(withPrincipalEnabled(true))
	saName := generateAgentResourceName(cr.Name, testCompName)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, saName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment was created
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.NoError(t, err)

	// Verify security context
	container := deployment.Spec.Template.Spec.Containers[0]
	assert.Equal(t, ptr.To(false), container.SecurityContext.AllowPrivilegeEscalation)
	assert.Equal(t, ptr.To(true), container.SecurityContext.ReadOnlyRootFilesystem)
	assert.Equal(t, ptr.To(true), container.SecurityContext.RunAsNonRoot)
	assert.Equal(t, []corev1.Capability{"ALL"}, container.SecurityContext.Capabilities.Drop)
	assert.Equal(t, corev1.SeccompProfileType("RuntimeDefault"), container.SecurityContext.SeccompProfile.Type)

	// Verify ports configuration
	assert.Len(t, container.Ports, 5)
	principalPort := container.Ports[0]
	assert.Equal(t, testCompName, principalPort.Name)
	assert.Equal(t, int32(8443), principalPort.ContainerPort)
	assert.Equal(t, corev1.ProtocolTCP, principalPort.Protocol)
	metricsPort := container.Ports[1]
	assert.Equal(t, "metrics", metricsPort.Name)
	assert.Equal(t, int32(8000), metricsPort.ContainerPort)
	assert.Equal(t, corev1.ProtocolTCP, metricsPort.Protocol)
	redisPort := container.Ports[2]
	assert.Equal(t, "redis", redisPort.Name)
	assert.Equal(t, int32(6379), redisPort.ContainerPort)
	assert.Equal(t, corev1.ProtocolTCP, redisPort.Protocol)
	resourceProxyPort := container.Ports[3]
	assert.Equal(t, "resource-proxy", resourceProxyPort.Name)
	assert.Equal(t, int32(9090), resourceProxyPort.ContainerPort)
	assert.Equal(t, corev1.ProtocolTCP, resourceProxyPort.Protocol)
	healthzPort := container.Ports[4]
	assert.Equal(t, "healthz", healthzPort.Name)
	assert.Equal(t, int32(8003), healthzPort.ContainerPort)
	assert.Equal(t, corev1.ProtocolTCP, healthzPort.Protocol)

	// Verify args
	assert.Equal(t, []string{testCompName}, container.Args)

	// Verify environment variables are set
	assert.True(t, len(container.Env) > 0)
	// Verify some expected environment variables are present
	envNames := make(map[string]bool)
	for _, env := range container.Env {
		// TODO: Convert to volume mount once possible: https://issues.redhat.com/browse/GITOPS-9070
		if env.Name == "REDIS_PASSWORD" {
			continue
		}

		envNames[env.Name] = true
		// All environment variables should have direct values, not references
		assert.Nil(t, env.ValueFrom, "Environment variable %s should have direct value, not reference", env.Name)
	}
	// Check for some environment variables
	assert.True(t, envNames["ARGOCD_PRINCIPAL_NAMESPACE"], "ARGOCD_PRINCIPAL_NAMESPACE should be set")
	// TODO: Convert to volume mount once possible: https://issues.redhat.com/browse/GITOPS-9070
	assert.False(t, envNames["REDIS_PASSWORD"], "REDIS_PASSWORD should not be set")

	// Verify volume mounts
	assert.Len(t, container.VolumeMounts, 3)
	jwtVolumeMount := container.VolumeMounts[0]
	assert.Equal(t, "jwt-secret", jwtVolumeMount.Name)
	assert.Equal(t, "/app/config/jwt", jwtVolumeMount.MountPath)
	userpassVolumeMount := container.VolumeMounts[1]
	assert.Equal(t, "userpass-passwd", userpassVolumeMount.Name)
	assert.Equal(t, "/app/config/userpass", userpassVolumeMount.MountPath)
	redisAuthVolumeMount := container.VolumeMounts[2]
	assert.Equal(t, "redis-initial-pass", redisAuthVolumeMount.Name)
	assert.Equal(t, "/app/config/redis-auth/", redisAuthVolumeMount.MountPath)

	// Verify pod volumes
	assert.Len(t, deployment.Spec.Template.Spec.Volumes, 3)
	jwtVolume := deployment.Spec.Template.Spec.Volumes[0]
	assert.Equal(t, "jwt-secret", jwtVolume.Name)
	assert.NotNil(t, jwtVolume.Secret)
	assert.Equal(t, "argocd-agent-jwt", jwtVolume.Secret.SecretName)
	assert.Equal(t, ptr.To(true), jwtVolume.Secret.Optional)
	assert.Len(t, jwtVolume.Secret.Items, 1)
	assert.Equal(t, "jwt.key", jwtVolume.VolumeSource.Secret.Items[0].Key)
	assert.Equal(t, "jwt.key", jwtVolume.VolumeSource.Secret.Items[0].Path)

	userpassVolume := deployment.Spec.Template.Spec.Volumes[1]
	assert.Equal(t, "userpass-passwd", userpassVolume.Name)
	assert.NotNil(t, userpassVolume.Secret)
	assert.Equal(t, "argocd-agent-principal-userpass", userpassVolume.Secret.SecretName)
	assert.Equal(t, ptr.To(true), userpassVolume.Secret.Optional)
	assert.Len(t, userpassVolume.Secret.Items, 1)
	assert.Equal(t, "passwd", userpassVolume.VolumeSource.Secret.Items[0].Key)
	assert.Equal(t, "passwd", userpassVolume.VolumeSource.Secret.Items[0].Path)

	redisAuthVolume := deployment.Spec.Template.Spec.Volumes[2]
	assert.Equal(t, "redis-initial-pass", redisAuthVolume.Name)
	assert.NotNil(t, redisAuthVolume.Secret)
	assert.Equal(t, "argocd-redis-initial-password", redisAuthVolume.Secret.SecretName)
	assert.NotEqual(t, ptr.To(true), redisAuthVolume.Secret.Optional)
	assert.Len(t, redisAuthVolume.Secret.Items, 0)
}

func TestReconcilePrincipalDeployment_CustomImage(t *testing.T) {
	// Test case: Verify custom image is used when specified
	// Expected behavior: Should create deployment with custom image

	customImage := "custom-registry/argocd-agent:custom-tag"
	cr := makeTestArgoCD(withPrincipalEnabled(true), withPrincipalImage(customImage))
	saName := generateAgentResourceName(cr.Name, testCompName)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, saName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment was created with custom image
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.NoError(t, err)
	assert.Equal(t, customImage, deployment.Spec.Template.Spec.Containers[0].Image)
}

func TestReconcilePrincipalDeployment_DefaultImage(t *testing.T) {
	// Test case: Verify default image is used when no custom image is specified
	// Expected behavior: Should create deployment with default image

	cr := makeTestArgoCD(withPrincipalEnabled(true))
	saName := generateAgentResourceName(cr.Name, testCompName)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, saName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment was created with default image
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.NoError(t, err)
	assert.Equal(t, common.ArgoCDAgentPrincipalDefaultImageName, deployment.Spec.Template.Spec.Containers[0].Image)
}

func TestReconcilePrincipalDeployment_VolumeMountsAndVolumes(t *testing.T) {
	// Test case: Verify volume mounts and volumes are correctly configured
	// Expected behavior: Should create deployment with JWT and userpass volume mounts and volumes

	cr := makeTestArgoCD(withPrincipalEnabled(true))
	saName := generateAgentResourceName(cr.Name, testCompName)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, saName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment was created
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.NoError(t, err)

	// Verify volume mounts
	container := deployment.Spec.Template.Spec.Containers[0]

	// Verify specific volume mount details
	assert.Len(t, container.VolumeMounts, 3)
	jwtMount := container.VolumeMounts[0]
	assert.Equal(t, "jwt-secret", jwtMount.Name)
	assert.Equal(t, "/app/config/jwt", jwtMount.MountPath)

	userpassMount := container.VolumeMounts[1]
	assert.Equal(t, "userpass-passwd", userpassMount.Name)
	assert.Equal(t, "/app/config/userpass", userpassMount.MountPath)

	redisAuthMount := container.VolumeMounts[2]
	assert.Equal(t, "redis-initial-pass", redisAuthMount.Name)

	// Verify specific volume details
	assert.Len(t, deployment.Spec.Template.Spec.Volumes, 3)
	jwtVolume := deployment.Spec.Template.Spec.Volumes[0]
	assert.Equal(t, "jwt-secret", jwtVolume.Name)
	assert.Equal(t, "argocd-agent-jwt", jwtVolume.Secret.SecretName)
	assert.Equal(t, ptr.To(true), jwtVolume.Secret.Optional)

	userpassVolume := deployment.Spec.Template.Spec.Volumes[1]
	assert.Equal(t, "userpass-passwd", userpassVolume.Name)
	assert.Equal(t, "argocd-agent-principal-userpass", userpassVolume.Secret.SecretName)
	assert.Equal(t, ptr.To(true), userpassVolume.Secret.Optional)

	redisAuthVolume := deployment.Spec.Template.Spec.Volumes[2]
	assert.Equal(t, "redis-initial-pass", redisAuthVolume.Name)
}

func TestBuildPrincipalImage(t *testing.T) {
	tests := []struct {
		name          string
		cr            *argoproj.ArgoCD
		envImage      string
		expectedImage string
		description   string
	}{
		{
			name: "CR specification takes precedence",
			cr: makeTestArgoCD(
				withPrincipalEnabled(true),
				withPrincipalImage("custom-registry/argocd-agent:custom-tag"),
			),
			envImage:      "env-registry/argocd-agent:env-tag",
			expectedImage: "custom-registry/argocd-agent:custom-tag",
			description:   "When CR specifies an image, it should take precedence over environment variable and default",
		},
		{
			name: "Environment variable used when CR image not specified",
			cr: makeTestArgoCD(
				withPrincipalEnabled(true),
			),
			envImage:      "env-registry/argocd-agent:env-tag",
			expectedImage: "env-registry/argocd-agent:env-tag",
			description:   "When CR doesn't specify an image but environment variable is set, use environment variable",
		},
		{
			name: "Default image used when neither CR nor environment specified",
			cr: makeTestArgoCD(
				withPrincipalEnabled(true),
			),
			envImage:      "",
			expectedImage: common.ArgoCDAgentPrincipalDefaultImageName,
			description:   "When neither CR nor environment variable specifies an image, use default",
		},
		{
			name: "Empty CR image should not override environment variable",
			cr: makeTestArgoCD(
				withPrincipalEnabled(true),
				withPrincipalImage(""),
			),
			envImage:      "env-registry/argocd-agent:env-tag",
			expectedImage: "env-registry/argocd-agent:env-tag",
			description:   "When CR specifies empty image, environment variable should be used",
		},
		{
			name: "Default image used when CR image is empty and no environment variable",
			cr: makeTestArgoCD(
				withPrincipalEnabled(true),
				withPrincipalImage(""),
			),
			envImage:      "",
			expectedImage: common.ArgoCDAgentPrincipalDefaultImageName,
			description:   "When CR specifies empty image and no environment variable, use default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable if specified
			if tt.envImage != "" {
				t.Setenv("ARGOCD_PRINCIPAL_IMAGE", tt.envImage)
			} else {
				// Clear environment variable
				t.Setenv("ARGOCD_PRINCIPAL_IMAGE", "")
			}

			result := buildPrincipalImage(tt.cr)
			assert.Equal(t, tt.expectedImage, result, tt.description)
		})
	}
}
