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
	"testing"

	argoappv1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	appsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	fakeNs             = "foo"
	fakeReplicas int32 = 1
	fakeVolumes        = []corev1.Volume{
		{
			Name: "sso-x509-https-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: servingCertSecretName,
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
	cr := makeTestArgoCD()
	cr.Spec.SSO = &argoappv1.ArgoCDSSOSpec{
		Provider: "keycloak",
	}

	// When both cr.spec.sso.Image and ArgoCDKeycloakImageEnvName are not set.
	testImage := getKeycloakContainerImage(cr)
	assert.Equal(t, testImage,
		"quay.io/keycloak/keycloak@sha256:828e92baa29aee2fdf30cca0e0aeefdf77ca458d6818ebbd08bf26f1c5c6a7cf")

	// When ENV variable is set.
	err := os.Setenv(common.ArgoCDKeycloakImageEnvName, "envImage:latest")
	defer os.Unsetenv(common.ArgoCDKeycloakImageEnvName)
	assert.NilError(t, err)

	testImage = getKeycloakContainerImage(cr)
	assert.Equal(t, testImage, "envImage:latest")

	// when both cr.spec.sso.Image and ArgoCDKeycloakImageEnvName are set.
	cr.Spec.SSO.Image = "crImage"
	cr.Spec.SSO.Version = "crVersion"

	testImage = getKeycloakContainerImage(cr)
	assert.Equal(t, testImage, "crImage:crVersion")
}

func TestNewKeycloakTemplateInstance(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.SSO = &argoappv1.ArgoCDSSOSpec{
		Provider: "keycloak",
	}
	tmplInstance, err := newKeycloakTemplateInstance(a)
	assert.NilError(t, err)

	assert.Equal(t, tmplInstance.Name, "rhsso")
	assert.Equal(t, tmplInstance.Namespace, a.Namespace)
}

func TestNewKeycloakTemplate(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.SSO = &argoappv1.ArgoCDSSOSpec{
		Provider: "keycloak",
	}
	tmpl, err := newKeycloakTemplate(a)
	assert.NilError(t, err)

	assert.Equal(t, tmpl.Name, "rhsso")
	assert.Equal(t, tmpl.Namespace, a.Namespace)
}

func TestNewKeycloakTemplate_testDeploymentConfig(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.SSO = &argoappv1.ArgoCDSSOSpec{
		Provider: "keycloak",
	}
	dc := getKeycloakDeploymentConfigTemplate(a)

	assert.Equal(t, dc.Spec.Replicas, fakeReplicas)
	assert.DeepEqual(t, dc.Spec.Strategy, appsv1.DeploymentStrategy{Type: "Recreate"})
	assert.Equal(t, dc.Spec.Template.ObjectMeta.Name, "${APPLICATION_NAME}")
	assert.DeepEqual(t, dc.Spec.Template.Spec.Volumes, fakeVolumes)
}

func TestNewKeycloakTemplate_testKeycloakContainer(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.SSO = &argoappv1.ArgoCDSSOSpec{
		Provider: "keycloak",
	}
	kc := getKeycloakContainer(a)
	assert.Equal(t, kc.Image,
		"quay.io/keycloak/keycloak@sha256:828e92baa29aee2fdf30cca0e0aeefdf77ca458d6818ebbd08bf26f1c5c6a7cf")
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

func TestNewKeycloakTemplate_testRoute(t *testing.T) {
	route := getKeycloakRouteTemplate(fakeNs)
	assert.Equal(t, route.Name, "${APPLICATION_NAME}")
	assert.Equal(t, route.Namespace, fakeNs)
	assert.DeepEqual(t, route.Spec.To,
		routev1.RouteTargetReference{Name: "${APPLICATION_NAME}"})
	assert.DeepEqual(t, route.Spec.TLS,
		&routev1.TLSConfig{Termination: "reencrypt"})
}

func TestKeycloak_testRealmConfigCreation(t *testing.T) {
	cfg := &keycloakConfig{
		ArgoName:      "foo-argocd",
		ArgoNamespace: "foo",
		Username:      "test-user",
		Password:      "test",
		KeycloakURL:   "https://foo.keycloak.com",
		ArgoCDURL:     "https://bar.argocd.com",
	}

	_, err := createRealmConfig(cfg)
	assert.NilError(t, err)
}

func TestKeycloak_testServerCert(t *testing.T) {

	a := makeTestArgoCDForKeycloak()
	r := makeFakeReconciler(t, a)

	sslCertsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      servingCertSecretName,
			Namespace: a.Namespace,
		},
		Data: map[string][]byte{
			"tls.crt": []byte("asdasfsff"),
		},
	}
	r.client.Create(context.TODO(), sslCertsSecret)

	_, err := r.getKCServerCert(a)
	assert.NilError(t, err)

	sslCertsSecret.Data["tls.crt"] = nil
	r.client.Update(context.TODO(), sslCertsSecret)

	_, err = r.getKCServerCert(a)
	assert.NilError(t, err)
}
