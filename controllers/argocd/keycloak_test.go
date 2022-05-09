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
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argoappv1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
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

	defer removeTemplateAPI()
	tests := []struct {
		name               string
		setEnvVarFunc      func(string)
		envVar             string
		restoreEnvFunc     func(t *testing.T)
		argoCD             *argoprojv1alpha1.ArgoCD
		updateCrFunc       func(cr *argoprojv1alpha1.ArgoCD)
		templateAPIFound   bool
		wantContainerImage string
	}{
		{
			name:           "no .spec.sso, no ArgoCDKeycloakImageEnvName env var set",
			setEnvVarFunc:  nil,
			envVar:         "",
			restoreEnvFunc: restoreEnv,
			argoCD: makeArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
				}
			}),
			updateCrFunc:       nil,
			templateAPIFound:   false,
			wantContainerImage: "quay.io/keycloak/keycloak@sha256:64fb81886fde61dee55091e6033481fa5ccdac62ae30a4fd29b54eb5e97df6a9",
		},
		{
			name:           "no .spec.sso, no ArgoCDKeycloakImageEnvName env var set - for OCP",
			setEnvVarFunc:  nil,
			envVar:         "",
			restoreEnvFunc: restoreEnv,
			argoCD: makeArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
				}
			}),
			updateCrFunc:       nil,
			templateAPIFound:   true,
			wantContainerImage: "registry.redhat.io/rh-sso-7/sso75-openshift-rhel8@sha256:720a7e4c4926c41c1219a90daaea3b971a3d0da5a152a96fed4fb544d80f52e3",
		},
		{
			name: "ArgoCDKeycloakImageEnvName env var set",
			setEnvVarFunc: func(s string) {
				os.Setenv(common.ArgoCDKeycloakImageEnvName, s)
			},
			envVar:         "envImage:latest",
			restoreEnvFunc: restoreEnv,
			argoCD: makeArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
				}
			}),
			updateCrFunc:       nil,
			templateAPIFound:   true,
			wantContainerImage: "envImage:latest",
		},
		{
			name: "both cr.spec.sso.Image and ArgoCDKeycloakImageEnvName are set.",
			setEnvVarFunc: func(s string) {
				os.Setenv(common.ArgoCDKeycloakImageEnvName, s)
			},
			envVar:         "envImage:latest",
			restoreEnvFunc: restoreEnv,
			argoCD: makeArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
				}
			}),
			updateCrFunc: func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
					Image:    "crImage",
					Version:  "crVersion",
				}
			},
			templateAPIFound:   true,
			wantContainerImage: "crImage:crVersion",
		},
		{
			name: "both cr.spec.sso.keycloak.Image and ArgoCDKeycloakImageEnvName are set",
			setEnvVarFunc: func(s string) {
				os.Setenv(common.ArgoCDKeycloakImageEnvName, s)
			},
			envVar:         "envImage:latest",
			restoreEnvFunc: restoreEnv,
			argoCD: makeArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
				}
			}),
			updateCrFunc: func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
					Keycloak: &v1alpha1.ArgoCDKeycloakSpec{
						Image:   "crImage",
						Version: "crVersion",
					},
				}
			},
			templateAPIFound:   true,
			wantContainerImage: "crImage:crVersion",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.restoreEnvFunc(t)
			templateAPIFound = test.templateAPIFound

			if test.setEnvVarFunc != nil {
				test.setEnvVarFunc(test.envVar)
			}
			if test.updateCrFunc != nil {
				test.updateCrFunc(test.argoCD)
			}

			testImage := getKeycloakContainerImage(test.argoCD)
			assert.Equal(t, test.wantContainerImage, testImage)

		})
	}
}

func TestNewKeycloakTemplateInstance(t *testing.T) {
	// For OpenShift Container Platform.
	templateAPIFound = true
	defer removeTemplateAPI()

	a := makeTestArgoCD()
	a.Spec.SSO = &argoappv1.ArgoCDSSOSpec{
		Provider: "keycloak",
	}
	tmplInstance, err := newKeycloakTemplateInstance(a)
	assert.NoError(t, err)

	assert.Equal(t, tmplInstance.Name, "rhsso")
	assert.Equal(t, tmplInstance.Namespace, a.Namespace)
}

func TestNewKeycloakTemplate(t *testing.T) {
	// For OpenShift Container Platform.
	templateAPIFound = true
	defer removeTemplateAPI()

	a := makeTestArgoCD()
	a.Spec.SSO = &argoappv1.ArgoCDSSOSpec{
		Provider: "keycloak",
	}
	tmpl, err := newKeycloakTemplate(a)
	assert.NoError(t, err)

	assert.Equal(t, tmpl.Name, "rhsso")
	assert.Equal(t, tmpl.Namespace, a.Namespace)
}

func TestNewKeycloakTemplate_testDeploymentConfig(t *testing.T) {
	// For OpenShift Container Platform.
	templateAPIFound = true
	defer removeTemplateAPI()

	a := makeTestArgoCD()
	a.Spec.SSO = &argoappv1.ArgoCDSSOSpec{
		Provider: "keycloak",
	}
	dc := getKeycloakDeploymentConfigTemplate(a)

	assert.Equal(t, dc.Spec.Replicas, fakeReplicas)

	strategy := appsv1.DeploymentStrategy{
		Type: "Recreate",
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resourcev1.MustParse("256Mi"),
				corev1.ResourceCPU:    resourcev1.MustParse("250m"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resourcev1.MustParse("512Mi"),
				corev1.ResourceCPU:    resourcev1.MustParse("500m"),
			},
		},
	}
	assert.Equal(t, dc.Spec.Strategy, strategy)

	assert.Equal(t, dc.Spec.Template.ObjectMeta.Name, "${APPLICATION_NAME}")
	assert.Equal(t, dc.Spec.Template.Spec.Volumes, fakeVolumes)
}

func TestNewKeycloakTemplate_testKeycloakContainer(t *testing.T) {
	// For OpenShift Container Platform.
	templateAPIFound = true
	defer removeTemplateAPI()

	a := makeTestArgoCD()
	a.Spec.SSO = &argoappv1.ArgoCDSSOSpec{
		Provider: "keycloak",
	}
	kc := getKeycloakContainer(a)
	assert.Equal(t, kc.Image,
		"registry.redhat.io/rh-sso-7/sso75-openshift-rhel8@sha256:720a7e4c4926c41c1219a90daaea3b971a3d0da5a152a96fed4fb544d80f52e3")
	assert.Equal(t, kc.ImagePullPolicy, corev1.PullAlways)
	assert.Equal(t, kc.Name, "${APPLICATION_NAME}")
}

func TestKeycloakResources(t *testing.T) {
	defer removeTemplateAPI()
	fR := getFakeKeycloakResources()

	tests := []struct {
		name          string
		argoCD        *argoprojv1alpha1.ArgoCD
		updateCrFunc  func(cr *argoprojv1alpha1.ArgoCD)
		wantResources corev1.ResourceRequirements
	}{
		{
			name: "default",
			argoCD: makeTestArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
				}
			}),
			updateCrFunc:  nil,
			wantResources: defaultKeycloakResources(),
		},
		{
			name: "override with .spec.sso",
			argoCD: makeTestArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
				}
			}),
			updateCrFunc: func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Resources: &fR,
				}
			},
			wantResources: getFakeKeycloakResources(),
		},
		{
			name: "override with .spec.sso.keycloak",
			argoCD: makeTestArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
				}
			}),
			updateCrFunc: func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Keycloak: &v1alpha1.ArgoCDKeycloakSpec{
						Resources: &fR,
					},
				}
			},
			wantResources: getFakeKeycloakResources(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.updateCrFunc != nil {
				test.updateCrFunc(test.argoCD)
			}

			testResources := getKeycloakContainer(test.argoCD).Resources
			assert.Equal(t, test.wantResources, testResources)

		})
	}
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
	assert.Equal(t, svc.Spec.Selector, map[string]string{
		"deploymentConfig": "${APPLICATION_NAME}"})
}

func TestNewKeycloakTemplate_testRoute(t *testing.T) {
	route := getKeycloakRouteTemplate(fakeNs)
	assert.Equal(t, route.Name, "${APPLICATION_NAME}")
	assert.Equal(t, route.Namespace, fakeNs)
	assert.Equal(t, route.Spec.To,
		routev1.RouteTargetReference{Name: "${APPLICATION_NAME}"})
	assert.Equal(t, route.Spec.TLS,
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
	assert.NoError(t, err)
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
	assert.NoError(t, err)

	sslCertsSecret.Data["tls.crt"] = nil
	assert.NoError(t, r.Client.Update(context.TODO(), sslCertsSecret))

	_, err = r.getKCServerCert(a)
	assert.NoError(t, err)
}

func TestKeycloak_NodeLabelSelector(t *testing.T) {
	a := makeTestArgoCDForKeycloak()
	a.Spec.NodePlacement = &argoappv1.ArgoCDNodePlacementSpec{
		NodeSelector: deploymentDefaultNodeSelector(),
		Tolerations:  deploymentDefaultTolerations(),
	}

	dc := getKeycloakDeploymentConfigTemplate(a)
	assert.Equal(t, dc.Spec.Template.Spec.NodeSelector, a.Spec.NodePlacement.NodeSelector)
	assert.Equal(t, dc.Spec.Template.Spec.Tolerations, a.Spec.NodePlacement.Tolerations)
}

func removeTemplateAPI() {
	templateAPIFound = false
}
