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
	e "errors"
	"fmt"
	"reflect"
	"time"

	oappsv1 "github.com/openshift/api/apps/v1"
	template "github.com/openshift/api/template/v1"
	oappsv1client "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	templatev1client "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// SuccessResonse is returned when a realm is created in keycloak.
	successResponse = "201 Created"
	// ExpectedReplicas is used to identify the keycloak running status.
	expectedReplicas int32 = 1
	// ServingCertSecretName is a secret that holds the service certificate.
	servingCertSecretName = "sso-x509-https-secret"
	// Authentication api path for keycloak.
	authURL = "/auth/realms/master/protocol/openid-connect/token"
	// Realm api path for keycloak.
	realmURL = "/auth/admin/realms"
	// Keycloak client for Argo CD.
	keycloakClient = "argocd"
	// Keycloak realm for Argo CD.
	keycloakRealm = "argocd"
	// Secret to authenticate argocd client.
	argocdClientSecret = "admin"
	// Secret to authenticate oAuthClient.
	oAuthClientSecret = "admin"
	// Identifier for Keycloak.
	defaultKeycloakIdentifier = "keycloak"
	// Identifier for TemplateInstance and Template.
	defaultTemplateIdentifier = "rhsso"
	// Default name for Keycloak broker.
	defaultKeycloakBrokerName = "keycloak-broker"
	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
	template "github.com/openshift/api/template/v1"
)

var (
	templateAPIFound = false
)

// IsTemplateAPIAvailable returns true if the template API is present.
func IsTemplateAPIAvailable() bool {
	return templateAPIFound
}

// verifyTemplateAPI will verify that the template API is present.
func verifyTemplateAPI() error {
	found, err := argoutil.VerifyAPI(template.SchemeGroupVersion.Group, template.SchemeGroupVersion.Version)
	if err != nil {
		return err
	}
	templateAPIFound = found
	return nil
}

func (r *ReconcileArgoCD) reconcileSSO(cr *argoprojv1a1.ArgoCD) error {
	if cr.Spec.SSO.Provider == argoprojv1a1.SSOProviderTypeKeycloak {
		if cr.Spec.Dex.OpenShiftOAuth || cr.Spec.Dex.Config != "" {
			err := e.New("multiple SSO configuration")
			log.Error(err, fmt.Sprintf("Installation of multiple SSO providers is not permitted. Please choose a single provider for Argo CD %s in namespace %s.",
				cr.Name, cr.Namespace))
			return err
		}

		// TemplateAPI is available, Install keycloak using openshift templates.
		if IsTemplateAPIAvailable() {
			err := r.reconcileKeycloakForOpenShift(cr)
			if err != nil {
				return err
			}
<<<<<<< HEAD:controllers/argocd/sso.go
			err = r.Client.Get(context.TODO(), types.NamespacedName{Name: templateInstanceRef.Name,
				Namespace: templateInstanceRef.Namespace}, &template.TemplateInstance{})
			if err != nil {
				if errors.IsNotFound(err) {
					log.Info(fmt.Sprintf("Template API found, Installing keycloak using openshift templates for ArgoCD %s in namespace %s",
						cr.Name, cr.Namespace))

					if err := controllerutil.SetControllerReference(cr, templateInstanceRef, r.Scheme); err != nil {
						return err
					}

					err = r.Client.Create(context.TODO(), templateInstanceRef)
					if err != nil {
						return err
					}
				} else {
					return err
				}
			}

			existingDC := &oappsv1.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      defaultKeycloakIdentifier,
					Namespace: cr.Namespace,
				},
			}

			if argoutil.IsObjectFound(r.Client, existingDC.Namespace, existingDC.Name, existingDC) {
				changed := false

				// Check if the resource requirements are updated by the user.
				existingResources := existingDC.Spec.Template.Spec.Containers[0].Resources
				desiredResources := getKeycloakResources(cr)
				if !reflect.DeepEqual(existingResources, desiredResources) {
					existingDC.Spec.Template.Spec.Containers[0].Resources = desiredResources
					changed = true
				}

				// Check if the Image is updated by the user.
				existingImage := existingDC.Spec.Template.Spec.Containers[0].Image
				desiredImage := getKeycloakContainerImage(cr)
				if existingImage != desiredImage {
					existingDC.Spec.Template.Spec.Containers[0].Image = desiredImage
					existingDC.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
					changed = true
				}

				// Check if Node Placement is updated by the user.
				actualDC := getKeycloakDeploymentConfigTemplate(cr)
				if !reflect.DeepEqual(existingDC.Spec.Template.Spec.NodeSelector, actualDC.Spec.Template.Spec.NodeSelector) {
					existingDC.Spec.Template.Spec.NodeSelector = actualDC.Spec.Template.Spec.NodeSelector
					changed = true
				}

				if !reflect.DeepEqual(existingDC.Spec.Template.Spec.Tolerations, actualDC.Spec.Template.Spec.Tolerations) {
					existingDC.Spec.Template.Spec.Tolerations = actualDC.Spec.Template.Spec.Tolerations
					changed = true
				}

				// If Keycloak deployment exists and a realm is already created for ArgoCD, Do not create a new one.
				if existingDC.Status.AvailableReplicas == expectedReplicas &&
					existingDC.Annotations["argocd.argoproj.io/realm-created"] == "false" {

					cfg, err := r.prepareKeycloakConfig(cr)
					if err != nil {
						return err
					}

					// keycloakRouteURL is used to update the OIDC configuraton for ArgoCD.
					keycloakRouteURL := cfg.KeycloakURL

					// Create a keycloak realm and publish.
					response, err := createRealm(cfg)
					if err != nil {
						log.Error(err, fmt.Sprintf("Failed posting keycloak realm configuration for ArgoCD %s in namespace %s",
							cr.Name, cr.Namespace))
						return err
					}

					if response == successResponse {
						log.Info(fmt.Sprintf("Successfully created keycloak realm for ArgoCD %s in namespace %s",
							cr.Name, cr.Namespace))

						// Update Realm creation. This will avoid posting of realm configuration on further reconciliations.
						existingDC.Annotations["argocd.argoproj.io/realm-created"] = "true"
						changed = true

						err = r.updateArgoCDConfiguration(cr, keycloakRouteURL)
						if err != nil {
							log.Error(err, fmt.Sprintf("Failed to update OIDC Configuration for ArgoCD %s in namespace %s",
								cr.Name, cr.Namespace))
							return err
						}
					}
				}

				if changed {
					err = r.Client.Update(context.TODO(), existingDC)
					if err != nil {
						return err
					}
				}

=======
		} else {
			err := r.reconcileKeycloak(cr)
			if err != nil {
				return err
>>>>>>> feat: Automatically install and configure Keycloak SSO for Kubernetes platform (#312):pkg/controller/argocd/sso.go
			}
		}
	}
	return nil
}

func deleteSSOConfiguration(cr *argoprojv1a1.ArgoCD) error {

	// If SSO is installed using OpenShift templates.
	if IsTemplateAPIAvailable() {
		err := deleteKeycloakConfigForOpenShift(cr)
		if err != nil {
			return err
		}
	} else {
		err := deleteKeycloakConfigForK8s(cr)
		if err != nil {
			return err
		}
	}

	return nil
}
