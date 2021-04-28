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
	"testing"

	appsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
)

var (
	fakeNs             = "foo"
	fakeReplicas int32 = 1
	fakeVolumes        = []corev1.Volume{
		{
			Name: "sso-x509-https-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "sso-x509-https-secret",
				},
			},
		},
		{
			Name: "sso-x509-jgroups-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "sso-x509-https-secret",
				},
			},
		},
		{
			Name: "service-ca",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "${APPLICATION_NAME}-service-ca",
					},
				},
			},
		},
	}
)

func TestKeycloakContainerImage(t *testing.T) {
	testImage := getKeycloakContainerImage("sso-74-image", "7.4")
	assert.Equal(t, testImage, "sso-74-image:7.4")

	testImage = getKeycloakContainerImage("sso-74-image", "SHA256:cbb1222787986dfs999")
	assert.Equal(t, testImage, "sso-74-image@SHA256:cbb1222787986dfs999")
}

func TestNewKeycloakTemplateInstance(t *testing.T) {
	tmplInstance, err := newKeycloakTemplateInstance(fakeNs)
	assert.NilError(t, err)

	assert.Equal(t, tmplInstance.Name, "rhsso")
	assert.Equal(t, tmplInstance.Namespace, fakeNs)
}

func TestNewKeycloakTemplate(t *testing.T) {
	tmpl, err := newKeycloakTemplate(fakeNs)
	assert.NilError(t, err)

	assert.Equal(t, tmpl.Name, "rhsso")
	assert.Equal(t, tmpl.Namespace, fakeNs)
}

func TestNewKeycloakTemplate_testDeploymentConfig(t *testing.T) {
	dc := getKeycloakDeploymentConfigTemplate(fakeNs)

	assert.Equal(t, dc.Spec.Replicas, fakeReplicas)
	assert.DeepEqual(t, dc.Spec.Strategy, appsv1.DeploymentStrategy{Type: "Recreate"})
	assert.Equal(t, dc.Spec.Template.ObjectMeta.Name, "${APPLICATION_NAME}")
	assert.DeepEqual(t, dc.Spec.Template.Spec.Volumes, fakeVolumes)
}

func TestNewKeycloakTemplate_testKeycloakContainer(t *testing.T) {
	kc := getKeycloakContainer()
	assert.Equal(t, kc.Image, "")
	assert.Equal(t, kc.ImagePullPolicy, corev1.PullAlways)
	assert.Equal(t, kc.Name, "${APPLICATION_NAME}")
}

func TestNewKeycloakTemplate_testConfigmap(t *testing.T) {
	cm := getKeycloakConfigMapTemplate(fakeNs)
	assert.Equal(t, cm.Name, "${APPLICATION_NAME}-service-ca")
	assert.Equal(t, cm.Namespace, fakeNs)
}

func TestNewKeycloakTemplate_testService(t *testing.T) {
	svc := getKeycloakServiceTemplate(fakeNs)
	assert.Equal(t, svc.Name, "${APPLICATION_NAME}")
	assert.Equal(t, svc.Namespace, fakeNs)
	assert.DeepEqual(t, svc.Spec.Selector, map[string]string{
		"deploymentConfig": "${APPLICATION_NAME}"})
}

func TestNewKeycloakTemplate_testPingService(t *testing.T) {
	svc := getKeycloakPingServiceTemplate(fakeNs)
	assert.Equal(t, svc.Name, "${APPLICATION_NAME}-ping")
	assert.Equal(t, svc.Namespace, fakeNs)
	assert.DeepEqual(t, svc.Spec.Selector, map[string]string{
		"deploymentConfig": "${APPLICATION_NAME}"})
}

func TestNewKeycloakTemplate_testRoute(t *testing.T) {
	route := getKeycloakRouteTemplate(fakeNs)
	assert.Equal(t, route.Name, "${APPLICATION_NAME}")
	assert.Equal(t, route.Namespace, fakeNs)
	assert.DeepEqual(t, route.Spec.To,
		routev1.RouteTargetReference{Name: "${APPLICATION_NAME}"})
	assert.DeepEqual(t, route.Spec.TLS,
		&routev1.TLSConfig{Termination: "reencrypt"})
}
