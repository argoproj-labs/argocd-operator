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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testSAName = "test-service-account"
)

func TestReconcilePrincipalDeployment_DeploymentDoesNotExist_PrincipalDisabled(t *testing.T) {
	// Test case: Deployment doesn't exist and principal is disabled
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestArgoCD(withPrincipalEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, testSAName, cr, sch)
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

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, testSAName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment was created
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.NoError(t, err)

	// Verify Deployment has expected metadata
	expectedName := generateAgentResourceName(cr.Name, testCompName)
	assert.Equal(t, expectedName, deployment.Name)
	assert.Equal(t, cr.Namespace, deployment.Namespace)
	assert.Equal(t, buildLabelsForAgentPrincipal(cr.Name), deployment.Labels)

	// Verify Deployment has expected spec
	expectedSpec := buildPrincipalSpec(testCompName, testSAName, cr)
	assert.Equal(t, expectedSpec.Selector, deployment.Spec.Selector)
	assert.Equal(t, expectedSpec.Template.Labels, deployment.Spec.Template.Labels)
	assert.Equal(t, expectedSpec.Template.Spec.ServiceAccountName, deployment.Spec.Template.Spec.ServiceAccountName)
	assert.Len(t, deployment.Spec.Template.Spec.Containers, 1)

	// Verify container spec
	container := deployment.Spec.Template.Spec.Containers[0]
	assert.Equal(t, buildPrincipalImage(), container.Image)
	assert.Equal(t, generateAgentResourceName(cr.Name, testCompName), container.Name)
	assert.Equal(t, buildPrincipalContainerEnv(), container.Env)
	assert.Equal(t, buildArgs(), container.Args)
	assert.Equal(t, buildSecurityContext(), container.SecurityContext)
	assert.Equal(t, buildPorts(), container.Ports)

	// Verify owner reference is set
	assert.Len(t, deployment.OwnerReferences, 1)
	assert.Equal(t, cr.Name, deployment.OwnerReferences[0].Name)
	assert.Equal(t, "ArgoCD", deployment.OwnerReferences[0].Kind)
}

func TestReconcilePrincipalDeployment_DeploymentExists_PrincipalDisabled(t *testing.T) {
	// Test case: Deployment exists and principal is disabled
	// Expected behavior: Should delete the Deployment

	cr := makeTestArgoCD(withPrincipalEnabled(false))

	// Create existing Deployment
	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name),
		},
		Spec: buildPrincipalSpec(testCompName, testSAName, cr),
	}

	resObjs := []client.Object{cr, existingDeployment}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, testSAName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment was deleted
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalDeployment_DeploymentExists_PrincipalEnabled_SameSpec(t *testing.T) {
	// Test case: Deployment exists, principal is enabled, and spec is the same
	// Expected behavior: Should do nothing (no update)

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	expectedSpec := buildPrincipalSpec(testCompName, testSAName, cr)
	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name),
		},
		Spec: expectedSpec,
	}

	resObjs := []client.Object{cr, existingDeployment}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, testSAName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment still exists with same spec
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.NoError(t, err)
	assert.Equal(t, expectedSpec.Selector, deployment.Spec.Selector)
	assert.Equal(t, expectedSpec.Template.Spec.ServiceAccountName, deployment.Spec.Template.Spec.ServiceAccountName)
}

func TestReconcilePrincipalDeployment_DeploymentExists_PrincipalEnabled_DifferentSpec(t *testing.T) {
	// Test case: Deployment exists, principal is enabled, but spec is different
	// Expected behavior: Should update the Deployment with expected spec

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	// Create existing Deployment with different spec
	differentSpec := appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "different-app", // Different selector
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"app": "different-app",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "different-container",
						Image: "different-image:tag",
					},
				},
				ServiceAccountName: "different-sa",
			},
		},
	}

	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name),
		},
		Spec: differentSpec,
	}

	resObjs := []client.Object{cr, existingDeployment}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, testSAName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment was updated with expected spec
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.NoError(t, err)

	expectedSpec := buildPrincipalSpec(testCompName, testSAName, cr)
	assert.Equal(t, expectedSpec.Selector, deployment.Spec.Selector)
	assert.Equal(t, expectedSpec.Template.Spec.ServiceAccountName, deployment.Spec.Template.Spec.ServiceAccountName)
	assert.NotEqual(t, differentSpec.Selector, deployment.Spec.Selector)
	assert.NotEqual(t, differentSpec.Template.Spec.ServiceAccountName, deployment.Spec.Template.Spec.ServiceAccountName)

	// Verify container was updated
	assert.Len(t, deployment.Spec.Template.Spec.Containers, 1)
	container := deployment.Spec.Template.Spec.Containers[0]
	assert.Equal(t, buildPrincipalImage(), container.Image)
	assert.Equal(t, generateAgentResourceName(cr.Name, testCompName), container.Name)
	assert.NotEqual(t, "different-image:tag", container.Image)
	assert.NotEqual(t, "different-container", container.Name)
}

func TestReconcilePrincipalDeployment_DeploymentExists_PrincipalNotSet(t *testing.T) {
	// Test case: Deployment exists but principal is not configured (nil)
	// Expected behavior: Should delete the Deployment (default behavior when not enabled)

	cr := makeTestArgoCD() // No principal configuration

	// Create existing Deployment
	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name),
		},
		Spec: buildPrincipalSpec(testCompName, testSAName, cr),
	}

	resObjs := []client.Object{cr, existingDeployment}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, testSAName, cr, sch)
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
	// Test case: Deployment doesn't exist and agent is not configured (nil)
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestArgoCD() // No agent configuration

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, testSAName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment was not created
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalDeployment_ImageUpdate(t *testing.T) {
	// Test case: Deployment exists with different image, should update

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	// Set environment variable for image
	originalImage := os.Getenv("ARGOCD_AGENT_PRINCIPAL_IMAGE")
	defer func() {
		if originalImage != "" {
			os.Setenv("ARGOCD_AGENT_PRINCIPAL_IMAGE", originalImage)
		} else {
			os.Unsetenv("ARGOCD_AGENT_PRINCIPAL_IMAGE")
		}
	}()
	os.Setenv("ARGOCD_AGENT_PRINCIPAL_IMAGE", "new-image:v2")

	// Create existing Deployment with old image
	existingSpec := buildPrincipalSpec(testCompName, testSAName, cr)
	existingSpec.Template.Spec.Containers[0].Image = "old-image:v1"

	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name),
		},
		Spec: existingSpec,
	}

	resObjs := []client.Object{cr, existingDeployment}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, testSAName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment was updated with new image
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.NoError(t, err)

	assert.Len(t, deployment.Spec.Template.Spec.Containers, 1)
	container := deployment.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "new-image:v2", container.Image)
	assert.NotEqual(t, "old-image:v1", container.Image)
}

func TestReconcilePrincipalDeployment_ServiceAccountNameUpdate(t *testing.T) {
	// Test case: Deployment exists with different service account name, should update

	cr := makeTestArgoCD(withPrincipalEnabled(true))
	newSAName := "new-service-account"

	// Create existing Deployment with old service account
	existingSpec := buildPrincipalSpec(testCompName, testSAName, cr)

	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name),
		},
		Spec: existingSpec,
	}

	resObjs := []client.Object{cr, existingDeployment}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalDeployment(cl, testCompName, newSAName, cr, sch)
	assert.NoError(t, err)

	// Verify Deployment was updated with new service account name
	deployment := &appsv1.Deployment{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, deployment)
	assert.NoError(t, err)

	assert.Equal(t, newSAName, deployment.Spec.Template.Spec.ServiceAccountName)
	assert.NotEqual(t, testSAName, deployment.Spec.Template.Spec.ServiceAccountName)
}
