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
	"fmt"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	oappsv1 "github.com/openshift/api/apps/v1"
	template "github.com/openshift/api/template/v1"
	oappsv1client "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	templatev1client "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	// SuccessResonse is returned when a realm is created in keycloak.
	SuccessResponse = "201 Created"
	// ExpectedReplicas is used to identify the keycloak running status.
	ExpectedReplicas int32 = 1
	// ServingCertSecretName is a secret that holds the service certificate.
	ServingCertSecretName = "sso-x509-https-secret"
	// Authentication api path for keycloak.
	authURL = "/auth/realms/master/protocol/openid-connect/token"
	// Realm api path for keycloak.
	realmURL = "/auth/admin/realms"
	// Keycloak client for Argo CD.
	KeycloakClient = "argocd"
	// Keycloak realm for Argo CD.
	KeycloakRealm = "argocd"
	// Secret to authenticate argocd client.
	ArgocdClientSecret = "admin"
	// Secret to authenticate oAuthClient.
	OAuthClientSecret = "admin"
	// Identifier for Keycloak.
	DefaultKeycloakIdentifier = "keycloak"
	// Identifier for TemplateInstance and Template.
	defaultTemplateIdentifier = "rhsso"
	// Default name for Keycloak broker.
	defaultKeycloakBrokerName = "keycloak-broker"
)

var (
	templateAPIFound = false
)

// KeycloakConfig defines the values required to update Keycloak Realm.
type KeycloakConfig struct {
	ArgoName           string
	ArgoNamespace      string
	Username           string
	Password           string
	KeycloakURL        string
	ArgoCDURL          string
	KeycloakServerCert []byte
	VerifyTLS          bool
}

type OidcConfig struct {
	Name           string   `json:"name"`
	Issuer         string   `json:"issuer"`
	ClientID       string   `json:"clientID"`
	ClientSecret   string   `json:"clientSecret"`
	RequestedScope []string `json:"requestedScopes"`
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

func deleteSSOConfiguration(cr *argoprojv1a1.ArgoCD) error {

	// If SSO is installed using OpenShift templates.
	if IsTemplateAPIAvailable() {
		cfg, err := config.GetConfig()
		if err != nil {
			logr.Error(err, fmt.Sprintf("unable to get k8s config for ArgoCD %s in namespace %s",
				cr.Name, cr.Namespace))
			return err
		}

		// Initialize template client.
		templateclient, err := templatev1client.NewForConfig(cfg)
		if err != nil {
			logr.Error(err, fmt.Sprintf("unable to create Template client for ArgoCD %s in namespace %s",
				cr.Name, cr.Namespace))
			return err
		}

		logr.Info(fmt.Sprintf("Delete Template Instance for ArgoCD %s in namespace %s",
			cr.Name, cr.Namespace))
		// We use the foreground propagation policy to ensure that the garbage
		// collector removes all instantiated objects before the TemplateInstance
		// itself disappears.
		foreground := metav1.DeletePropagationForeground
		deleteOptions := metav1.DeleteOptions{PropagationPolicy: &foreground}
		err = templateclient.TemplateInstances(cr.Namespace).Delete(context.TODO(), defaultTemplateIdentifier, deleteOptions)
		if err != nil {
			return err
		}

		// Delete OAuthClient created for keycloak.
		oauth, err := oauthclient.NewForConfig(cfg)
		if err != nil {
			logr.Error(err, fmt.Sprintf("unable to create oAuth client for ArgoCD %s in namespace %s",
				cr.Name, cr.Namespace))
			return err
		}
		logr.Info(fmt.Sprintf("Delete OAuthClient for ArgoCD %s in namespace %s",
			cr.Name, cr.Namespace))

		oa := getOAuthClient(cr.Namespace)
		err = oauth.OAuthClients().Delete(context.TODO(), oa, deleteOptions)
		if err != nil {
			return err
		}
	}

	return nil
}

// HandleKeycloakPodDeletion resets the Realm Creation Status to false when keycloak pod is deleted.
func handleKeycloakPodDeletion(dc *oappsv1.DeploymentConfig) error {
	cfg, err := config.GetConfig()
	if err != nil {
		logr.Error(err, fmt.Sprintf("unable to get k8s config"))
		return err
	}

	// Initialize deployment config client.
	dcClient, err := oappsv1client.NewForConfig(cfg)
	if err != nil {
		logr.Error(err, fmt.Sprintf("unable to create apps client for Deployment config %s in namespace %s",
			dc.Name, dc.Namespace))
		return err
	}

	logr.Info(fmt.Sprintf("Set the Realm Creation status annoation to false"))
	existingDC, err := dcClient.DeploymentConfigs(dc.Namespace).Get(context.TODO(), defaultKeycloakIdentifier, metav1.GetOptions{})
	if err != nil {
		return err
	}

	existingDC.Annotations["argocd.argoproj.io/realm-created"] = "false"
	_, err = dcClient.DeploymentConfigs(dc.Namespace).Update(context.TODO(), existingDC, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// IsTemplateAPIAvailable returns true if the template API is present.
func IsTemplateAPIAvailable() bool {
	return templateAPIFound
}
