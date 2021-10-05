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

	appsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoappv1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
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

func getFakeKeycloakResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("1Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("1m"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("128Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("128m"),
		},
	}
}

func TestKeycloakContainerImage(t *testing.T) {
	cr := makeTestArgoCD()
	cr.Spec.SSO = &argoappv1.ArgoCDSSOSpec{
		Provider: "keycloak",
	}

	// When both cr.spec.sso.Image and ArgoCDKeycloakImageEnvName are not set.
	testImage := getKeycloakContainerImage(cr)
	assert.Equal(t, testImage,
		"registry.redhat.io/rh-sso-7/sso74-openshift-rhel8@sha256:39d752173fc97c29373cd44477b48bcb078531def0a897ee81a60e8d1d0212cc")

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
		"registry.redhat.io/rh-sso-7/sso74-openshift-rhel8@sha256:39d752173fc97c29373cd44477b48bcb078531def0a897ee81a60e8d1d0212cc")
	assert.Equal(t, kc.ImagePullPolicy, corev1.PullAlways)
	assert.Equal(t, kc.Name, "${APPLICATION_NAME}")
}

func TestKeycloakResources(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.SSO = &argoappv1.ArgoCDSSOSpec{
		Provider: "keycloak",
	}
	kc := getKeycloakContainer(a)

	// Verify resource requirements are set to default.
	assert.DeepEqual(t, kc.Resources, defaultKeycloakResources())

	// Verify resource requirements are overridden by ArgoCD CR(.spec.SSO.Resources)
	fR := getFakeKeycloakResources()
	a.Spec.SSO.Resources = &fR

	kc = getKeycloakContainer(a)
	assert.DeepEqual(t, kc.Resources, getFakeKeycloakResources())
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
	r.Client.Create(context.TODO(), sslCertsSecret)

	_, err := r.getKCServerCert(a)
	assert.NilError(t, err)

	sslCertsSecret.Data["tls.crt"] = nil
	r.Client.Update(context.TODO(), sslCertsSecret)

	_, err = r.getKCServerCert(a)
	assert.NilError(t, err)
}

func TestKeycloak_NodeLabelSelector(t *testing.T) {
	a := makeTestArgoCDForKeycloak()
	a.Spec.NodePlacement = &argoappv1.ArgoCDNodePlacementSpec{
		NodeSelector: deploymentDefaultNodeSelector(),
		Tolerations:  deploymentDefaultTolerations(),
	}

	dc := getKeycloakDeploymentConfigTemplate(a)
	assert.DeepEqual(t, dc.Spec.Template.Spec.NodeSelector, a.Spec.NodePlacement.NodeSelector)
	assert.DeepEqual(t, dc.Spec.Template.Spec.Tolerations, a.Spec.NodePlacement.Tolerations)
}
