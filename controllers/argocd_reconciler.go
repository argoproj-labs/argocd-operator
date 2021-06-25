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

package controllers

import (
	b64 "encoding/base64"

	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	argopass "github.com/argoproj/argo-cd/util/password"
	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"

	oappsv1 "github.com/openshift/api/apps/v1"
	template "github.com/openshift/api/template/v1"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	autoscaling "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	crierrors "k8s.io/cri-api/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ArgoCDReconciler) deleteClusterResources(cr *argoprojv1a1.ArgoCD) error {
	selector, err := argocd.ArgocdInstanceSelector(cr.Name)
	if err != nil {
		return err
	}

	clusterRoleList := &v1.ClusterRoleList{}
	if err := argocd.FilterObjectsBySelector(r.Client, clusterRoleList, selector); err != nil {
		return fmt.Errorf("failed to filter ClusterRoles for %s: %w", cr.Name, err)
	}

	if err := argocd.DeleteClusterRoles(r.Client, clusterRoleList); err != nil {
		return err
	}

	clusterBindingsList := &v1.ClusterRoleBindingList{}
	if err := argocd.FilterObjectsBySelector(r.Client, clusterBindingsList, selector); err != nil {
		return fmt.Errorf("failed to filter ClusterRoleBindings for %s: %w", cr.Name, err)
	}

	if err := argocd.DeleteClusterRoleBindings(r.Client, clusterBindingsList); err != nil {
		return err
	}

	return nil
}

func (r *ArgoCDReconciler) removeDeletionFinalizer(cr *argoprojv1a1.ArgoCD) error {
	cr.Finalizers = argocd.RemoveString(cr.GetFinalizers(), common.ArgoCDDeletionFinalizer)
	if err := r.Client.Update(context.TODO(), cr); err != nil {
		return fmt.Errorf("failed to remove deletion finalizer from %s: %w", cr.Name, err)
	}
	return nil
}

func (r *ArgoCDReconciler) addDeletionFinalizer(argocd *argoprojv1a1.ArgoCD) error {
	argocd.Finalizers = append(argocd.Finalizers, common.ArgoCDDeletionFinalizer)
	if err := r.Client.Update(context.TODO(), argocd); err != nil {
		return fmt.Errorf("failed to add deletion finalizer for %s: %w", argocd.Name, err)
	}
	return nil
}

// reconcileResources will reconcile common ArgoCD resources.
func (r *ArgoCDReconciler) reconcileResources(cr *argoprojv1a1.ArgoCD) error {
	logr.Info("reconciling status")
	if err := r.reconcileStatus(cr); err != nil {
		return err
	}

	logr.Info("reconciling roles")
	if _, err := r.reconcileRoles(cr); err != nil {
		return err
	}

	logr.Info("reconciling rolebindings")
	if err := r.reconcileRoleBindings(cr); err != nil {
		return err
	}

	logr.Info("reconciling service accounts")
	if err := r.reconcileServiceAccounts(cr); err != nil {
		return err
	}

	logr.Info("reconciling certificate authority")
	if err := r.reconcileCertificateAuthority(cr); err != nil {
		return err
	}

	logr.Info("reconciling secrets")
	if err := r.reconcileSecrets(cr); err != nil {
		return err
	}

	logr.Info("reconciling config maps")
	if err := r.reconcileConfigMaps(cr); err != nil {
		return err
	}

	logr.Info("reconciling services")
	if err := r.reconcileServices(cr); err != nil {
		return err
	}

	logr.Info("reconciling deployments")
	if err := r.reconcileDeployments(cr); err != nil {
		return err
	}

	logr.Info("reconciling statefulsets")
	if err := r.reconcileStatefulSets(cr); err != nil {
		return err
	}

	logr.Info("reconciling autoscalers")
	if err := r.reconcileAutoscalers(cr); err != nil {
		return err
	}

	logr.Info("reconciling ingresses")
	if err := r.reconcileIngresses(cr); err != nil {
		return err
	}

	if argocd.IsRouteAPIAvailable() {
		logr.Info("reconciling routes")
		if err := r.reconcileRoutes(cr); err != nil {
			return err
		}
	}

	if argocd.IsPrometheusAPIAvailable() {
		logr.Info("reconciling prometheus")
		if err := r.reconcilePrometheus(cr); err != nil {
			return err
		}

		if err := r.reconcileMetricsServiceMonitor(cr); err != nil {
			return err
		}

		if err := r.reconcileRepoServerServiceMonitor(cr); err != nil {
			return err
		}

		if err := r.reconcileServerMetricsServiceMonitor(cr); err != nil {
			return err
		}
	}

	if cr.Spec.ApplicationSet != nil {
		logr.Info("reconciling ApplicationSet controller")
		if err := r.reconcileApplicationSetController(cr); err != nil {
			return err
		}
	}

	if err := r.reconcileRepoServerTLSSecret(cr); err != nil {
		return err
	}

	if cr.Spec.SSO != nil {
		logr.Info("reconciling SSO")
		if err := r.reconcileSSO(cr); err != nil {
			return err
		}
	}

	return nil
}

func (r *ArgoCDReconciler) reconcileSSO(cr *argoprojv1a1.ArgoCD) error {
	if cr.Spec.SSO.Provider == argoprojv1a1.SSOProviderTypeKeycloak {
		// TemplateAPI is available, Install keycloack using openshift templates.
		if argocd.IsTemplateAPIAvailable() {
			templateInstanceRef, err := argocd.NewKeycloakTemplateInstance(cr)
			if err != nil {
				return err
			}
			err = r.Client.Get(context.TODO(), types.NamespacedName{Name: templateInstanceRef.Name,
				Namespace: templateInstanceRef.Namespace}, &template.TemplateInstance{})
			if err != nil {
				if crierrors.IsNotFound(err) {
					logr.Info(fmt.Sprintf("Template API found, Installing keycloak using openshift templates for ArgoCD %s in namespace %s",
						cr.Name, cr.Namespace))

					if err := controllerutil.SetControllerReference(cr, templateInstanceRef, r.scheme); err != nil {
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
					Name:      argocd.DefaultKeycloakIdentifier,
					Namespace: cr.Namespace,
				},
			}
			err = r.Client.Get(context.TODO(), types.NamespacedName{Name: existingDC.Name, Namespace: existingDC.Namespace}, existingDC)
			if err != nil {
				logr.Error(err, fmt.Sprintf("Keycloak Deployment not found or being created for ArgoCD %s in namespace %s",
					cr.Name, cr.Namespace))
			}

			// If Keycloak deployment exists and a realm is already created for ArgoCD, Do not create a new one.
			if existingDC.Status.AvailableReplicas == argocd.ExpectedReplicas &&
				existingDC.Annotations["argocd.argoproj.io/realm-created"] == "false" {

				cfg, err := r.prepareKeycloakConfig(cr)
				if err != nil {
					return err
				}

				// keycloakRouteURL is used to update the OIDC configuraton for ArgoCD.
				keycloakRouteURL := cfg.KeycloakURL

				// Create a keycloak realm and publish.
				response, err := argocd.CreateRealm(cfg)
				if err != nil {
					logr.Error(err, fmt.Sprintf("Failed posting keycloak realm configuration for ArgoCD %s in namespace %s",
						cr.Name, cr.Namespace))
					return err
				}

				if response == argocd.SuccessResponse {
					logr.Info(fmt.Sprintf("Successfully created keycloak realm for ArgoCD %s in namespace %s",
						cr.Name, cr.Namespace))

					// Update Realm creation. This will avoid posting of realm configuration on further reconciliations.
					existingDC.Annotations["argocd.argoproj.io/realm-created"] = "true"
					r.Client.Update(context.TODO(), existingDC)

					err = r.updateArgoCDConfiguration(cr, keycloakRouteURL)
					if err != nil {
						logr.Error(err, fmt.Sprintf("Failed to update OIDC Configuration for ArgoCD %s in namespace %s",
							cr.Name, cr.Namespace))
						return err
					}
				}
			}
		} else {
			return nil
		}
	}
	return nil
}

// Updates OIDC configuration for ArgoCD.
func (r *ArgoCDReconciler) updateArgoCDConfiguration(cr *argoprojv1a1.ArgoCD, kRouteURL string) error {

	// Update the ArgoCD client secret for OIDC in argocd-secret.
	argoCDSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDSecretName,
			Namespace: cr.Namespace,
		},
	}

	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: argoCDSecret.Name, Namespace: argoCDSecret.Namespace}, argoCDSecret)
	if err != nil {
		logr.Error(err, fmt.Sprintf("ArgoCD secret not found for ArgoCD %s in namespace %s",
			cr.Name, cr.Namespace))
		return err
	}

	argoCDSecret.Data["oidc.keycloak.clientSecret"] = []byte(argocd.ArgocdClientSecret)
	err = r.Client.Update(context.TODO(), argoCDSecret)
	if err != nil {
		logr.Error(err, fmt.Sprintf("Error updating ArgoCD Secret for ArgoCD %s in namespace %s",
			cr.Name, cr.Namespace))
		return err
	}

	// Create openshift OAuthClient
	oAuthClient := &oauthv1.OAuthClient{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OAuthClient",
			APIVersion: "oauth.openshift.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      argocd.GetOAuthClient(cr.Namespace),
			Namespace: cr.Namespace,
		},
		Secret: argocd.OAuthClientSecret,
		RedirectURIs: []string{fmt.Sprintf("%s/auth/realms/%s/broker/openshift-v4/endpoint",
			kRouteURL, argocd.KeycloakClient)},
		GrantMethod: "prompt",
	}

	err = controllerutil.SetOwnerReference(cr, oAuthClient, r.Scheme)
	if err != nil {
		return err
	}

	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: oAuthClient.Name}, oAuthClient)
	if err != nil {
		if crierrors.IsNotFound(err) {
			err = r.Client.Create(context.TODO(), oAuthClient)
			if err != nil {
				return err
			}
		}
	}

	// Update ArgoCD instance for OIDC Config with Keycloakrealm URL
	o, _ := yaml.Marshal(argocd.OidcConfig{
		Name: "Keycloak",
		Issuer: fmt.Sprintf("%s/auth/realms/%s",
			kRouteURL, argocd.KeycloakRealm),
		ClientID:       argocd.KeycloakClient,
		ClientSecret:   "$oidc.keycloak.clientSecret",
		RequestedScope: []string{"openid", "profile", "email", "groups"},
	})

	argoCDCM := argocd.NewConfigMapWithName(common.ArgoCDConfigMapName, cr)
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: argoCDCM.Name, Namespace: argoCDCM.Namespace}, argoCDCM)
	if err != nil {
		logr.Error(err, fmt.Sprintf("ArgoCD configmap not found for ArgoCD %s in namespace %s",
			cr.Name, cr.Namespace))

		return err
	}

	argoCDCM.Data[common.ArgoCDKeyOIDCConfig] = string(o)
	err = r.Client.Update(context.TODO(), argoCDCM)
	if err != nil {
		logr.Error(err, fmt.Sprintf("Error updating OIDC Configuration for ArgoCD %s in namespace %s",
			cr.Name, cr.Namespace))
		return err
	}

	// Update RBAC for ArgoCD Instance.
	argoRBACCM := argocd.NewConfigMapWithName(common.ArgoCDRBACConfigMapName, cr)
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: argoRBACCM.Name, Namespace: argoRBACCM.Namespace}, argoRBACCM)
	if err != nil {
		logr.Error(err, fmt.Sprintf("ArgoCD RBAC configmap not found for ArgoCD %s in namespace %s",
			cr.Name, cr.Namespace))

		return err
	}

	argoRBACCM.Data["scopes"] = "[groups,email]"
	err = r.Client.Update(context.TODO(), argoRBACCM)
	if err != nil {
		logr.Error(err, fmt.Sprintf("Error updating ArgoCD RBAC configmap %s in namespace %s",
			cr.Name, cr.Namespace))
		return err
	}

	return nil
}

// prepares a keycloak config which is used in creating keycloak realm configuration.
func (r *ArgoCDReconciler) prepareKeycloakConfig(cr *argoprojv1a1.ArgoCD) (*argocd.KeycloakConfig, error) {

	var tlsVerification bool
	// Get keycloak hostname from route.
	// keycloak hostname is required to post realm configuration to keycloak when keycloak cannot be accessed using service name
	// due to network policies or operator running outside the cluster or development purpose.
	existingKeycloakRoute := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      argocd.DefaultKeycloakIdentifier,
			Namespace: cr.Namespace,
		},
	}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: existingKeycloakRoute.Name,
		Namespace: existingKeycloakRoute.Namespace}, existingKeycloakRoute)
	if err != nil {
		return nil, err
	}
	kRouteURL := fmt.Sprintf("https://%s", existingKeycloakRoute.Spec.Host)

	// Get ArgoCD hostname from route. ArgoCD hostname is used in the keycloak client configuration.
	existingArgoCDRoute := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, "server"),
			Namespace: cr.Namespace,
		},
	}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: existingArgoCDRoute.Name,
		Namespace: existingArgoCDRoute.Namespace}, existingArgoCDRoute)
	if err != nil {
		return nil, err
	}
	aRouteURL := fmt.Sprintf("https://%s", existingArgoCDRoute.Spec.Host)

	// Get keycloak Secret for credentials. credentials are required to authenticate with keycloak.
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", argocd.DefaultKeycloakIdentifier, "secret"),
			Namespace: cr.Namespace,
		},
	}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: existingSecret.Name,
		Namespace: existingSecret.Namespace}, existingSecret)
	if err != nil {
		return nil, err
	}

	userEnc := b64.URLEncoding.EncodeToString(existingSecret.Data["SSO_USERNAME"])
	passEnc := b64.URLEncoding.EncodeToString(existingSecret.Data["SSO_PASSWORD"])

	username, _ := b64.URLEncoding.DecodeString(userEnc)
	password, _ := b64.URLEncoding.DecodeString(passEnc)

	// Get Keycloak Service Cert
	serverCert, err := r.getKCServerCert(cr)
	if err != nil {
		return nil, err
	}

	// By default TLS Verification should be enabled.
	if cr.Spec.SSO.VerifyTLS == nil || *cr.Spec.SSO.VerifyTLS == true {
		tlsVerification = true
	}

	cfg := &argocd.KeycloakConfig{
		ArgoName:           cr.Name,
		ArgoNamespace:      cr.Namespace,
		Username:           string(username),
		Password:           string(password),
		KeycloakURL:        kRouteURL,
		ArgoCDURL:          aRouteURL,
		KeycloakServerCert: serverCert,
		VerifyTLS:          tlsVerification,
	}

	return cfg, nil
}

// Gets Keycloak Server cert. This cert is used to authenticate the api calls to the Keycloak service.
func (r *ArgoCDReconciler) getKCServerCert(cr *argoprojv1a1.ArgoCD) ([]byte, error) {

	sslCertsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      argocd.ServingCertSecretName,
			Namespace: cr.Namespace,
		},
	}

	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: sslCertsSecret.Name, Namespace: sslCertsSecret.Namespace}, sslCertsSecret)

	switch {
	case err == nil:
		return sslCertsSecret.Data["tls.crt"], nil
	case crierrors.IsNotFound(err):
		return nil, nil
	default:
		return nil, err
	}
}

// reconcileRepoServerTLSSecret checks whether the argocd-repo-server-tls secret
// has changed since our last reconciliation loop. It does so by comparing the
// checksum of tls.crt and tls.key in the status of the ArgoCD CR against the
// values calculated from the live state in the cluster.
func (r *ArgoCDReconciler) reconcileRepoServerTLSSecret(cr *argoprojv1a1.ArgoCD) error {
	var tlsSecretObj corev1.Secret
	var sha256sum string

	logr.Info("reconciling repo-server TLS secret")

	tlsSecretName := types.NamespacedName{Namespace: cr.Namespace, Name: common.ArgoCDRepoServerTLSSecretName}
	err := r.Client.Get(context.TODO(), tlsSecretName, &tlsSecretObj)
	if err != nil {
		if !crierrors.IsNotFound(err) {
			return err
		}
	} else if tlsSecretObj.Type != corev1.SecretTypeTLS {
		// We only process secrets of type kubernetes.io/tls
		return nil
	} else {
		// We do the checksum over a concatenated byte stream of cert + key
		crt, crtOk := tlsSecretObj.Data[corev1.TLSCertKey]
		key, keyOk := tlsSecretObj.Data[corev1.TLSPrivateKeyKey]
		if crtOk && keyOk {
			var sumBytes []byte
			sumBytes = append(sumBytes, crt...)
			sumBytes = append(sumBytes, key...)
			sha256sum = fmt.Sprintf("%x", sha256.Sum256(sumBytes))
		}
	}

	// The content of the TLS secret has changed since we last looked if the
	// calculated checksum doesn't match the one stored in the status.
	if cr.Status.RepoTLSChecksum != sha256sum {
		// We store the value early to prevent a possible restart loop, for the
		// cost of a possibly missed restart when we cannot update the status
		// field of the resource.
		cr.Status.RepoTLSChecksum = sha256sum
		err = r.Client.Status().Update(context.TODO(), cr)
		if err != nil {
			return err
		}

		// Trigger rollout of API server
		apiDepl := argocd.NewDeploymentWithSuffix("server", "server", cr)
		err = r.triggerRollout(apiDepl, "repo.tls.cert.changed")
		if err != nil {
			return err
		}

		// Trigger rollout of repository server
		repoDepl := argocd.NewDeploymentWithSuffix("repo-server", "repo-server", cr)
		err = r.triggerRollout(repoDepl, "repo.tls.cert.changed")
		if err != nil {
			return err
		}

		// Trigger rollout of application controller
		controllerSts := argocd.NewStatefulSetWithSuffix("application-controller", "application-controller", cr)
		err = r.triggerRollout(controllerSts, "repo.tls.cert.changed")
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ArgoCDReconciler) reconcileApplicationSetController(cr *argoprojv1a1.ArgoCD) error {
	logr.Info("reconciling applicationset serviceaccounts")
	sa, err := r.reconcileApplicationSetServiceAccount(cr)
	if err != nil {
		return err
	}

	logr.Info("reconciling applicationset roles")
	role, err := r.reconcileApplicationSetRole(cr)
	if err != nil {
		return err
	}

	logr.Info("reconciling applicationset role bindings")
	if err := r.reconcileApplicationSetRoleBinding(cr, role, sa); err != nil {
		return err
	}

	logr.Info("reconciling applicationset deployments")
	if err := r.reconcileApplicationSetDeployment(cr, sa); err != nil {
		return err
	}

	return nil
}

// reconcileApplicationControllerDeployment will ensure the Deployment resource is present for the ArgoCD Application Controller component.
func (r *ArgoCDReconciler) reconcileApplicationSetDeployment(cr *argoprojv1a1.ArgoCD, sa *corev1.ServiceAccount) error {
	deploy := argocd.NewDeploymentWithSuffix("applicationset-controller", "controller", cr)

	argocd.SetAppSetLabels(&deploy.ObjectMeta)

	podSpec := &deploy.Spec.Template.Spec

	podSpec.ServiceAccountName = sa.ObjectMeta.Name

	podSpec.Volumes = []corev1.Volume{
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
			Name: "argocd-repo-server-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRepoServerTLSSecretName,
					Optional:   argocd.BoolPtr(true),
				},
			},
		},
	}

	podSpec.Containers = []corev1.Container{{
		Command: []string{"applicationset-controller", "--argocd-repo-server", argocd.GetRepoServerAddress(cr)},
		Env: []corev1.EnvVar{{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		}},
		Image:           argocd.GetApplicationSetContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "argocd-applicationset-controller",
		Resources:       argocd.GetApplicationSetResources(cr),
		VolumeMounts: []corev1.VolumeMount{
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
				Name:      "argocd-repo-server-tls",
				MountPath: "/app/config/reposerver/tls",
			},
		},
	}}

	if existing := argocd.NewDeploymentWithSuffix("applicationset-controller", "controller", cr); argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {

		// If the Deployment already exists, make sure the containers are up-to-date
		actualContainers := existing.Spec.Template.Spec.Containers[0]
		if !reflect.DeepEqual(actualContainers, podSpec.Containers) {
			existing.Spec.Template.Spec.Containers = podSpec.Containers
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
}

func (r *ArgoCDReconciler) reconcileApplicationSetRole(cr *argoprojv1a1.ArgoCD) (*v1.Role, error) {
	policyRules := []v1.PolicyRule{

		// ApplicationSet
		{
			APIGroups: []string{"argoproj.io"},
			Resources: []string{
				"applications",
				"applicationsets",
				"appprojects",
				"applicationsets/finalizers",
			},
			Verbs: []string{
				"create",
				"delete",
				"get",
				"list",
				"patch",
				"update",
				"watch",
			},
		},
		// ApplicationSet Status
		{
			APIGroups: []string{"argoproj.io"},
			Resources: []string{
				"applicationsets/status",
			},
			Verbs: []string{
				"get",
				"patch",
				"update",
			},
		},

		// Events
		{
			APIGroups: []string{""},
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"create",
				"delete",
				"get",
				"list",
				"patch",
				"update",
				"watch",
			},
		},

		// Read Secrets/ConfigMaps
		{
			APIGroups: []string{""},
			Resources: []string{
				"secrets",
				"configmaps",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},

		// Read Deployments
		{
			APIGroups: []string{"apps", "extensions"},
			Resources: []string{
				"deployments",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
	}

	role := argocd.NewRole("applicationset-controller", policyRules, cr)
	argocd.SetAppSetLabels(&role.ObjectMeta)

	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: cr.Namespace}, role)
	if err != nil {
		if !crierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the role for the service account associated with %s : %s", role.Name, err)
		}
		controllerutil.SetControllerReference(cr, role, r.Scheme)
		return role, r.Client.Create(context.TODO(), role)
	}

	role.Rules = policyRules
	controllerutil.SetControllerReference(cr, role, r.Scheme)
	return role, r.Client.Update(context.TODO(), role)
}

func (r *ArgoCDReconciler) reconcileApplicationSetRoleBinding(cr *argoprojv1a1.ArgoCD, role *v1.Role, sa *corev1.ServiceAccount) error {
	name := "applicationset-controller"

	// get expected name
	roleBinding := argocd.NewRoleBindingWithname(name, cr)

	// fetch existing rolebinding by name
	roleBindingExists := true
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: cr.Namespace}, roleBinding); err != nil {
		if !crierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s : %s", name, err)
		}
		roleBindingExists = false
	}

	argocd.SetAppSetLabels(&roleBinding.ObjectMeta)

	roleBinding.RoleRef = v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     "Role",
		Name:     role.Name,
	}

	roleBinding.Subjects = []v1.Subject{
		{
			Kind:      v1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
	}

	if err := controllerutil.SetControllerReference(cr, roleBinding, r.Scheme); err != nil {
		return err
	}

	if roleBindingExists {
		return r.Client.Update(context.TODO(), roleBinding)
	}

	return r.Client.Create(context.TODO(), roleBinding)
}

func (r *ArgoCDReconciler) reconcileApplicationSetServiceAccount(cr *argoprojv1a1.ArgoCD) (*corev1.ServiceAccount, error) {
	sa := argocd.NewServiceAccountWithName("applicationset-controller", cr)
	argocd.SetAppSetLabels(&sa.ObjectMeta)

	exists := true
	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		if !crierrors.IsNotFound(err) {
			return nil, err
		}
		exists = false
	}

	if exists {
		return sa, nil
	}

	if err := controllerutil.SetControllerReference(cr, sa, r.Scheme); err != nil {
		return nil, err
	}

	err := r.Client.Create(context.TODO(), sa)
	if err != nil {
		return nil, err
	}

	return sa, err
}

// reconcileRepoServerServiceMonitor will ensure that the ServiceMonitor is present for the Repo Server metrics Service.
func (r *ArgoCDReconciler) reconcileRepoServerServiceMonitor(cr *argoprojv1a1.ArgoCD) error {
	sm := argocd.NewServiceMonitorWithSuffix("repo-server-metrics", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, sm.Name, sm) {
		if !cr.Spec.Prometheus.Enabled {
			// ServiceMonitor exists but enabled flag has been set to false, delete the ServiceMonitor
			return r.Client.Delete(context.TODO(), sm)
		}
		return nil // ServiceMonitor found, do nothing
	}

	if !cr.Spec.Prometheus.Enabled {
		return nil // Prometheus not enabled, do nothing.
	}

	sm.Spec.Selector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.ArgoCDKeyName: argocd.NameWithSuffix("repo-server", cr),
		},
	}
	sm.Spec.Endpoints = []monitoringv1.Endpoint{
		{
			Port: common.ArgoCDKeyMetrics,
		},
	}

	if err := controllerutil.SetControllerReference(cr, sm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), sm)
}

// reconcileMetricsServiceMonitor will ensure that the ServiceMonitor is present for the ArgoCD metrics Service.
func (r *ArgoCDReconciler) reconcileMetricsServiceMonitor(cr *argoprojv1a1.ArgoCD) error {
	sm := argocd.NewServiceMonitorWithSuffix(common.ArgoCDKeyMetrics, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, sm.Name, sm) {
		if !cr.Spec.Prometheus.Enabled {
			// ServiceMonitor exists but enabled flag has been set to false, delete the ServiceMonitor
			return r.Client.Delete(context.TODO(), sm)
		}
		return nil // ServiceMonitor found, do nothing
	}

	if !cr.Spec.Prometheus.Enabled {
		return nil // Prometheus not enabled, do nothing.
	}

	sm.Spec.Selector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.ArgoCDKeyName: argocd.NameWithSuffix(common.ArgoCDKeyMetrics, cr),
		},
	}
	sm.Spec.Endpoints = []monitoringv1.Endpoint{
		{
			Port: common.ArgoCDKeyMetrics,
		},
	}

	if err := controllerutil.SetControllerReference(cr, sm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), sm)
}

// reconcilePrometheus will ensure that Prometheus is present for ArgoCD metrics.
func (r *ArgoCDReconciler) reconcilePrometheus(cr *argoprojv1a1.ArgoCD) error {
	prometheus := argocd.NewPrometheus(cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, prometheus.Name, prometheus) {
		if !cr.Spec.Prometheus.Enabled {
			// Prometheus exists but enabled flag has been set to false, delete the Prometheus
			return r.Client.Delete(context.TODO(), prometheus)
		}
		if argocd.HasPrometheusSpecChanged(prometheus, cr) {
			prometheus.Spec.Replicas = cr.Spec.Prometheus.Size
			return r.Client.Update(context.TODO(), prometheus)
		}
		return nil // Prometheus found, do nothing
	}

	if !cr.Spec.Prometheus.Enabled {
		return nil // Prometheus not enabled, do nothing.
	}

	prometheus.Spec.Replicas = argocd.GetPrometheusReplicas(cr)
	prometheus.Spec.ServiceAccountName = "prometheus-k8s"
	prometheus.Spec.ServiceMonitorSelector = &metav1.LabelSelector{}

	if err := controllerutil.SetControllerReference(cr, prometheus, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), prometheus)
}

// reconcileRoutes will ensure that all ArgoCD Routes are present.
func (r *ArgoCDReconciler) reconcileRoutes(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileGrafanaRoute(cr); err != nil {
		return err
	}

	if err := r.reconcilePrometheusRoute(cr); err != nil {
		return err
	}

	if err := r.reconcileServerRoute(cr); err != nil {
		return err
	}
	return nil
}

// reconcileServerRoute will ensure that the ArgoCD Server Route is present.
func (r *ArgoCDReconciler) reconcileServerRoute(cr *argoprojv1a1.ArgoCD) error {

	route := argocd.NewRouteWithSuffix("server", cr)
	found := argoutil.IsObjectFound(r.Client, cr.Namespace, route.Name, route)
	if found {
		if !cr.Spec.Server.Route.Enabled {
			// Route exists but enabled flag has been set to false, delete the Route
			return r.Client.Delete(context.TODO(), route)
		}
	}

	if !cr.Spec.Server.Route.Enabled {
		return nil // Route not enabled, move along...
	}

	// Allow override of the Annotations for the Route.
	if len(cr.Spec.Server.Route.Annotations) > 0 {
		route.Annotations = cr.Spec.Server.Route.Annotations
	}

	// Allow override of the Host for the Route.
	if len(cr.Spec.Server.Host) > 0 {
		route.Spec.Host = cr.Spec.Server.Host // TODO: What additional role needed for this?
	}

	if cr.Spec.Server.Insecure {
		// Disable TLS and rely on the cluster certificate.
		route.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("http"),
		}
		route.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationEdge,
		}
	} else {
		// Server is using TLS configure passthrough.
		route.Spec.Port = &routev1.RoutePort{
			TargetPort: intstr.FromString("https"),
		}
		route.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationPassthrough,
		}
	}

	// Allow override of TLS options for the Route
	if cr.Spec.Server.Route.TLS != nil {
		route.Spec.TLS = cr.Spec.Server.Route.TLS
	}

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = argocd.NameWithSuffix("server", cr)

	// Allow override of the WildcardPolicy for the Route
	if cr.Spec.Server.Route.WildcardPolicy != nil && len(*cr.Spec.Server.Route.WildcardPolicy) > 0 {
		route.Spec.WildcardPolicy = *cr.Spec.Server.Route.WildcardPolicy
	}

	if err := controllerutil.SetControllerReference(cr, route, r.Scheme); err != nil {
		return err
	}
	if !found {
		return r.Client.Create(context.TODO(), route)
	}
	return r.Client.Update(context.TODO(), route)
}

// reconcilePrometheusRoute will ensure that the ArgoCD Prometheus Route is present.
func (r *ArgoCDReconciler) reconcilePrometheusRoute(cr *argoprojv1a1.ArgoCD) error {
	route := argocd.NewRouteWithSuffix("prometheus", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, route.Name, route) {
		if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Route.Enabled {
			// Route exists but enabled flag has been set to false, delete the Route
			return r.Client.Delete(context.TODO(), route)
		}
		return nil // Route found, do nothing
	}

	if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Route.Enabled {
		return nil // Prometheus itself or Route not enabled, do nothing.
	}

	// Allow override of the Annotations for the Route.
	if len(cr.Spec.Prometheus.Route.Annotations) > 0 {
		route.Annotations = cr.Spec.Prometheus.Route.Annotations
	}

	// Allow override of the Host for the Route.
	if len(cr.Spec.Prometheus.Host) > 0 {
		route.Spec.Host = cr.Spec.Prometheus.Host // TODO: What additional role needed for this?
	}

	route.Spec.Port = &routev1.RoutePort{
		TargetPort: intstr.FromString("web"),
	}

	// Allow override of TLS options for the Route
	if cr.Spec.Prometheus.Route.TLS != nil {
		route.Spec.TLS = cr.Spec.Prometheus.Route.TLS
	}

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = "prometheus-operated"

	// Allow override of the WildcardPolicy for the Route
	if cr.Spec.Prometheus.Route.WildcardPolicy != nil && len(*cr.Spec.Prometheus.Route.WildcardPolicy) > 0 {
		route.Spec.WildcardPolicy = *cr.Spec.Prometheus.Route.WildcardPolicy
	}

	if err := controllerutil.SetControllerReference(cr, route, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), route)
}

// reconcileGrafanaRoute will ensure that the ArgoCD Grafana Route is present.
func (r *ArgoCDReconciler) reconcileGrafanaRoute(cr *argoprojv1a1.ArgoCD) error {
	route := argocd.NewRouteWithSuffix("grafana", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, route.Name, route) {
		if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Route.Enabled {
			// Route exists but enabled flag has been set to false, delete the Route
			return r.Client.Delete(context.TODO(), route)
		}
		return nil // Route found, do nothing
	}

	if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Route.Enabled {
		return nil // Grafana itself or Route not enabled, do nothing.
	}

	// Allow override of the Annotations for the Route.
	if len(cr.Spec.Grafana.Route.Annotations) > 0 {
		route.Annotations = cr.Spec.Grafana.Route.Annotations
	}

	// Allow override of the Host for the Route.
	if len(cr.Spec.Grafana.Host) > 0 {
		route.Spec.Host = cr.Spec.Grafana.Host // TODO: What additional role needed for this?
	}

	// Allow override of the Path for the Route
	if len(cr.Spec.Grafana.Route.Path) > 0 {
		route.Spec.Path = cr.Spec.Grafana.Route.Path
	}

	route.Spec.Port = &routev1.RoutePort{
		TargetPort: intstr.FromString("http"),
	}

	// Allow override of TLS options for the Route
	if cr.Spec.Grafana.Route.TLS != nil {
		route.Spec.TLS = cr.Spec.Grafana.Route.TLS
	}

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = argocd.NameWithSuffix("grafana", cr)

	// Allow override of the WildcardPolicy for the Route
	if cr.Spec.Grafana.Route.WildcardPolicy != nil && len(*cr.Spec.Grafana.Route.WildcardPolicy) > 0 {
		route.Spec.WildcardPolicy = *cr.Spec.Grafana.Route.WildcardPolicy
	}

	if err := controllerutil.SetControllerReference(cr, route, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), route)
}

// reconcileIngresses will ensure that all ArgoCD Ingress resources are present.
func (r *ArgoCDReconciler) reconcileIngresses(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileArgoServerIngress(cr); err != nil {
		return err
	}

	if err := r.reconcileArgoServerGRPCIngress(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaIngress(cr); err != nil {
		return err
	}

	if err := r.reconcilePrometheusIngress(cr); err != nil {
		return err
	}
	return nil
}

// reconcilePrometheusIngress will ensure that the Prometheus Ingress is present.
func (r *ArgoCDReconciler) reconcilePrometheusIngress(cr *argoprojv1a1.ArgoCD) error {
	ingress := argocd.NewIngressWithSuffix("prometheus", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.Client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Ingress.Enabled {
		return nil // Prometheus itself or Ingress not enabled, move along...
	}

	// Add annotations
	atns := argocd.GetDefaultIngressAnnotations(cr)
	atns[common.ArgoCDKeyIngressSSLRedirect] = "true"
	atns[common.ArgoCDKeyIngressBackendProtocol] = "HTTP"

	// Override default annotations if specified
	if len(cr.Spec.Prometheus.Ingress.Annotations) > 0 {
		atns = cr.Spec.Prometheus.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	// Add rules
	ingress.Spec.Rules = []extv1beta1.IngressRule{
		{
			Host: argocd.GetPrometheusHost(cr),
			IngressRuleValue: extv1beta1.IngressRuleValue{
				HTTP: &extv1beta1.HTTPIngressRuleValue{
					Paths: []extv1beta1.HTTPIngressPath{
						{
							Path: argocd.GetPathOrDefault(cr.Spec.Prometheus.Ingress.Path),
							Backend: extv1beta1.IngressBackend{
								ServiceName: "prometheus-operated",
								ServicePort: intstr.FromString("web"),
							},
						},
					},
				},
			},
		},
	}

	// Add TLS options
	ingress.Spec.TLS = []extv1beta1.IngressTLS{
		{
			Hosts:      []string{cr.Name},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// Allow override of TLS options if specified
	if len(cr.Spec.Prometheus.Ingress.TLS) > 0 {
		ingress.Spec.TLS = cr.Spec.Prometheus.Ingress.TLS
	}

	if err := controllerutil.SetControllerReference(cr, ingress, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ingress)
}

// reconcileGrafanaIngress will ensure that the ArgoCD Server GRPC Ingress is present.
func (r *ArgoCDReconciler) reconcileGrafanaIngress(cr *argoprojv1a1.ArgoCD) error {
	ingress := argocd.NewIngressWithSuffix("grafana", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.Client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Ingress.Enabled {
		return nil // Grafana itself or Ingress not enabled, move along...
	}

	// Add annotations
	atns := argocd.GetDefaultIngressAnnotations(cr)
	atns[common.ArgoCDKeyIngressSSLRedirect] = "true"
	atns[common.ArgoCDKeyIngressBackendProtocol] = "HTTP"

	// Override default annotations if specified
	if len(cr.Spec.Grafana.Ingress.Annotations) > 0 {
		atns = cr.Spec.Grafana.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	// Add rules
	ingress.Spec.Rules = []extv1beta1.IngressRule{
		{
			Host: argocd.GetGrafanaHost(cr),
			IngressRuleValue: extv1beta1.IngressRuleValue{
				HTTP: &extv1beta1.HTTPIngressRuleValue{
					Paths: []extv1beta1.HTTPIngressPath{
						{
							Path: argocd.GetPathOrDefault(cr.Spec.Grafana.Ingress.Path),
							Backend: extv1beta1.IngressBackend{
								ServiceName: argocd.NameWithSuffix("grafana", cr),
								ServicePort: intstr.FromString("http"),
							},
						},
					},
				},
			},
		},
	}

	// Add TLS options
	ingress.Spec.TLS = []extv1beta1.IngressTLS{
		{
			Hosts: []string{
				cr.Name,
				argocd.GetGrafanaHost(cr),
			},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// Allow override of TLS options if specified
	if len(cr.Spec.Grafana.Ingress.TLS) > 0 {
		ingress.Spec.TLS = cr.Spec.Grafana.Ingress.TLS
	}

	if err := controllerutil.SetControllerReference(cr, ingress, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ingress)
}

// reconcileArgoServerGRPCIngress will ensure that the ArgoCD Server GRPC Ingress is present.
func (r *ArgoCDReconciler) reconcileArgoServerGRPCIngress(cr *argoprojv1a1.ArgoCD) error {
	ingress := argocd.NewIngressWithSuffix("grpc", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Server.GRPC.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.Client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Server.GRPC.Ingress.Enabled {
		return nil // Ingress not enabled, move along...
	}

	// Add annotations
	atns := argocd.GetDefaultIngressAnnotations(cr)
	atns[common.ArgoCDKeyIngressBackendProtocol] = "GRPC"

	// Override default annotations if specified
	if len(cr.Spec.Server.GRPC.Ingress.Annotations) > 0 {
		atns = cr.Spec.Server.GRPC.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	// Add rules
	ingress.Spec.Rules = []extv1beta1.IngressRule{
		{
			Host: argocd.GetArgoServerGRPCHost(cr),
			IngressRuleValue: extv1beta1.IngressRuleValue{
				HTTP: &extv1beta1.HTTPIngressRuleValue{
					Paths: []extv1beta1.HTTPIngressPath{
						{
							Path: argocd.GetPathOrDefault(cr.Spec.Server.GRPC.Ingress.Path),
							Backend: extv1beta1.IngressBackend{
								ServiceName: argocd.NameWithSuffix("server", cr),
								ServicePort: intstr.FromString("https"),
							},
						},
					},
				},
			},
		},
	}

	// Add TLS options
	ingress.Spec.TLS = []extv1beta1.IngressTLS{
		{
			Hosts: []string{
				argocd.GetArgoServerGRPCHost(cr),
			},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// Allow override of TLS options if specified
	if len(cr.Spec.Server.GRPC.Ingress.TLS) > 0 {
		ingress.Spec.TLS = cr.Spec.Server.GRPC.Ingress.TLS
	}

	if err := controllerutil.SetControllerReference(cr, ingress, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ingress)
}

// reconcileArgoServerIngress will ensure that the ArgoCD Server Ingress is present.
func (r *ArgoCDReconciler) reconcileArgoServerIngress(cr *argoprojv1a1.ArgoCD) error {
	ingress := argocd.NewIngressWithSuffix("server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Server.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.Client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Server.Ingress.Enabled {
		return nil // Ingress not enabled, move along...
	}

	// Add annotations
	atns := argocd.GetDefaultIngressAnnotations(cr)
	atns[common.ArgoCDKeyIngressSSLRedirect] = "true"
	atns[common.ArgoCDKeyIngressBackendProtocol] = "HTTP"

	// Override default annotations if specified
	if len(cr.Spec.Server.Ingress.Annotations) > 0 {
		atns = cr.Spec.Server.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	// Add rules
	ingress.Spec.Rules = []extv1beta1.IngressRule{
		{
			Host: argocd.GetArgoServerHost(cr),
			IngressRuleValue: extv1beta1.IngressRuleValue{
				HTTP: &extv1beta1.HTTPIngressRuleValue{
					Paths: []extv1beta1.HTTPIngressPath{
						{
							Path: argocd.GetPathOrDefault(cr.Spec.Server.Ingress.Path),
							Backend: extv1beta1.IngressBackend{
								ServiceName: argocd.NameWithSuffix("server", cr),
								ServicePort: intstr.FromString("http"),
							},
						},
					},
				},
			},
		},
	}

	// Add default TLS options
	ingress.Spec.TLS = []extv1beta1.IngressTLS{
		{
			Hosts: []string{
				argocd.GetArgoServerHost(cr),
			},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// Allow override of TLS options if specified
	if len(cr.Spec.Server.Ingress.TLS) > 0 {
		ingress.Spec.TLS = cr.Spec.Server.Ingress.TLS
	}

	if err := controllerutil.SetControllerReference(cr, ingress, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ingress)
}

// reconcileAutoscalers will ensure that all HorizontalPodAutoscalers are present for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileAutoscalers(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileServerHPA(cr); err != nil {
		return err
	}
	return nil
}

// reconcileServerHPA will ensure that the HorizontalPodAutoscaler is present for the Argo CD Server component.
func (r *ArgoCDReconciler) reconcileServerHPA(cr *argoprojv1a1.ArgoCD) error {
	hpa := argocd.NewHorizontalPodAutoscalerWithSuffix("server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, hpa.Name, hpa) {
		if !cr.Spec.Server.Autoscale.Enabled {
			return r.Client.Delete(context.TODO(), hpa) // HorizontalPodAutoscaler found but globally disabled, delete it.
		}
		return nil // HorizontalPodAutoscaler found and configured, nothing do to, move along...
	}

	if !cr.Spec.Server.Autoscale.Enabled {
		return nil // AutoScale not enabled, move along...
	}

	if cr.Spec.Server.Autoscale.HPA != nil {
		hpa.Spec = *cr.Spec.Server.Autoscale.HPA
	} else {
		hpa.Spec.MaxReplicas = 3

		var minrReplicas int32 = 1
		hpa.Spec.MinReplicas = &minrReplicas

		var tcup int32 = 50
		hpa.Spec.TargetCPUUtilizationPercentage = &tcup

		hpa.Spec.ScaleTargetRef = autoscaling.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       argocd.NameWithSuffix("server", cr),
		}
	}

	return r.Client.Create(context.TODO(), hpa)
}

// reconcileStatefulSets will ensure that all StatefulSets are present for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileStatefulSets(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileApplicationControllerStatefulSet(cr); err != nil {
		return err
	}
	if err := r.reconcileRedisStatefulSet(cr); err != nil {
		return err
	}
	return nil
}

func (r *ArgoCDReconciler) reconcileRedisStatefulSet(cr *argoprojv1a1.ArgoCD) error {
	ss := argocd.NewStatefulSetWithSuffix("redis-ha-server", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, ss.Name, ss) {
		if !cr.Spec.HA.Enabled {
			// StatefulSet exists but HA enabled flag has been set to false, delete the StatefulSet
			return r.Client.Delete(context.TODO(), ss)
		}

		desiredImage := argocd.GetRedisHAContainerImage(cr)
		changed := false

		for i, container := range ss.Spec.Template.Spec.Containers {
			if container.Image != desiredImage {
				ss.Spec.Template.Spec.Containers[i].Image = argocd.GetRedisHAContainerImage(cr)
				changed = true
			}
		}

		if changed {
			ss.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			return r.Client.Update(context.TODO(), ss)
		}

		return nil // StatefulSet found, do nothing
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

	ss.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
	ss.Spec.Replicas = argocd.GetRedisHAReplicas(cr)
	ss.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.ArgoCDKeyName: argocd.NameWithSuffix("redis-ha", cr),
		},
	}

	ss.Spec.ServiceName = argocd.NameWithSuffix("redis-ha", cr)

	ss.Spec.Template.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"checksum/init-config": "552ee3bec8fe5d9d865e371f7b615c6d472253649eb65d53ed4ae874f782647c", // TODO: Should this be hard-coded?
		},
		Labels: map[string]string{
			common.ArgoCDKeyName: argocd.NameWithSuffix("redis-ha", cr),
		},
	}

	ss.Spec.Template.Spec.Affinity = &corev1.Affinity{
		PodAffinity: &corev1.PodAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.ArgoCDKeyName: argocd.NameWithSuffix("redis-ha", cr),
						},
					},
					TopologyKey: common.ArgoCDKeyFailureDomainZone,
				},
				Weight: int32(100),
			}},
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						common.ArgoCDKeyName: argocd.NameWithSuffix("redis-ha", cr),
					},
				},
				TopologyKey: common.ArgoCDKeyHostname,
			}},
		},
	}

	ss.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Args: []string{
				"/data/conf/redis.conf",
			},
			Command: []string{
				"redis-server",
			},
			Image:           argocd.GetRedisHAContainerImage(cr),
			ImagePullPolicy: corev1.PullIfNotPresent,
			LivenessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(common.ArgoCDDefaultRedisPort),
					},
				},
				InitialDelaySeconds: int32(15),
			},
			Name: "redis",
			Ports: []corev1.ContainerPort{{
				ContainerPort: common.ArgoCDDefaultRedisPort,
				Name:          "redis",
			}},
			Resources: argocd.GetRedisResources(cr),
			VolumeMounts: []corev1.VolumeMount{
				{
					MountPath: "/data",
					Name:      "data",
				},
			},
		},
		{
			Args: []string{
				"/data/conf/sentinel.conf",
			},
			Command: []string{
				"redis-sentinel",
			},
			Image:           argocd.GetRedisHAContainerImage(cr),
			ImagePullPolicy: corev1.PullIfNotPresent,
			LivenessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(common.ArgoCDDefaultRedisSentinelPort),
					},
				},
				InitialDelaySeconds: int32(15),
			},
			Name: "sentinel",
			Ports: []corev1.ContainerPort{{
				ContainerPort: common.ArgoCDDefaultRedisSentinelPort,
				Name:          "sentinel",
			}},
			Resources: argocd.GetRedisResources(cr),
			VolumeMounts: []corev1.VolumeMount{
				{
					MountPath: "/data",
					Name:      "data",
				},
			},
		},
	}

	ss.Spec.Template.Spec.InitContainers = []corev1.Container{{
		Args: []string{
			"/readonly-config/init.sh",
		},
		Command: []string{
			"sh",
		},
		Env: []corev1.EnvVar{
			{
				Name:  "SENTINEL_ID_0",
				Value: "25b71bd9d0e4a51945d8422cab53f27027397c12", // TODO: Should this be hard-coded?
			},
			{
				Name:  "SENTINEL_ID_1",
				Value: "896627000a81c7bdad8dbdcffd39728c9c17b309", // TODO: Should this be hard-coded?
			},
			{
				Name:  "SENTINEL_ID_2",
				Value: "3acbca861108bc47379b71b1d87d1c137dce591f", // TODO: Should this be hard-coded?
			},
		},
		Image:           argocd.GetRedisHAContainerImage(cr),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Name:            "config-init",
		Resources:       argocd.GetRedisResources(cr),
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/readonly-config",
				Name:      "config",
				ReadOnly:  true,
			},
			{
				MountPath: "/data",
				Name:      "data",
			},
		},
	}}

	var fsGroup int64 = 1000
	var runAsNonRoot bool = true
	var runAsUser int64 = 1000

	ss.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
		FSGroup:      &fsGroup,
		RunAsNonRoot: &runAsNonRoot,
		RunAsUser:    &runAsUser,
	}

	ss.Spec.Template.Spec.ServiceAccountName = argocd.NameWithSuffix("argocd-redis-ha", cr)

	ss.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDRedisHAConfigMapName,
					},
				},
			},
		}, {
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	ss.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
		Type: appsv1.RollingUpdateStatefulSetStrategyType,
	}

	if err := argocd.ApplyReconcilerHook(cr, ss, ""); err != nil {
		return err
	}

	if err := controllerutil.SetControllerReference(cr, ss, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ss)
}

func (r *ArgoCDReconciler) reconcileApplicationControllerStatefulSet(cr *argoprojv1a1.ArgoCD) error {
	var replicas int32 = 1 // TODO: allow override using CR ?
	ss := argocd.NewStatefulSetWithSuffix("application-controller", "application-controller", cr)
	ss.Spec.Replicas = &replicas

	podSpec := &ss.Spec.Template.Spec
	podSpec.Containers = []corev1.Container{{
		Command:         argocd.GetArgoApplicationControllerCommand(cr),
		Image:           argocd.GetArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "argocd-application-controller",
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8082),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		Env: argocd.ProxyEnvVars(),
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 8082,
			},
		},
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8082),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		Resources: argocd.GetArgoApplicationControllerResources(cr),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "argocd-repo-server-tls",
				MountPath: "/app/config/controller/tls",
			},
		},
	}}
	podSpec.ServiceAccountName = argocd.NameWithSuffix("argocd-application-controller", cr)
	podSpec.Volumes = []corev1.Volume{
		{
			Name: "argocd-repo-server-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRepoServerTLSSecretName,
					Optional:   argocd.BoolPtr(true),
				},
			},
		},
	}

	ss.Spec.Template.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.ArgoCDKeyName: argocd.NameWithSuffix("argocd-application-controller", cr),
						},
					},
					TopologyKey: common.ArgoCDKeyHostname,
				},
				Weight: int32(100),
			},
				{
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								common.ArgoCDKeyPartOf: common.ArgoCDAppName,
							},
						},
						TopologyKey: common.ArgoCDKeyHostname,
					},
					Weight: int32(5),
				}},
		},
	}

	// Handle import/restore from ArgoCDExport
	export := r.getArgoCDExport(cr)
	if export == nil {
		logr.Info("existing argocd export not found, skipping import")
	} else {
		podSpec.InitContainers = []corev1.Container{{
			Command:         argocd.GetArgoImportCommand(r.Client, cr),
			Env:             argocd.ProxyEnvVars(argocd.GetArgoImportContainerEnv(export)...),
			Resources:       argocd.GetArgoApplicationControllerResources(cr),
			Image:           argocd.GetArgoImportContainerImage(export),
			ImagePullPolicy: corev1.PullAlways,
			Name:            "argocd-import",
			VolumeMounts:    argocd.GetArgoImportVolumeMounts(export),
		}}

		podSpec.Volumes = argocd.GetArgoImportVolumes(export)
	}

	existing := argocd.NewStatefulSetWithSuffix("application-controller", "application-controller", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := argocd.GetArgoContainerImage(cr)
		changed := false
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}
		desiredCommand := argocd.GetArgoApplicationControllerCommand(cr)
		if argocd.IsRepoServerTLSVerificationRequested(cr) {
			desiredCommand = append(desiredCommand, "--repo-server-strict-tls")
		}
		if !reflect.DeepEqual(desiredCommand, existing.Spec.Template.Spec.Containers[0].Command) {
			existing.Spec.Template.Spec.Containers[0].Command = desiredCommand
			changed = true
		}

		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			ss.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = ss.Spec.Template.Spec.Containers[0].Env
			changed = true
		}
		if !reflect.DeepEqual(ss.Spec.Template.Spec.Volumes, existing.Spec.Template.Spec.Volumes) {
			existing.Spec.Template.Spec.Volumes = ss.Spec.Template.Spec.Volumes
			changed = true
		}
		if !reflect.DeepEqual(ss.Spec.Template.Spec.Containers[0].VolumeMounts,
			existing.Spec.Template.Spec.Containers[0].VolumeMounts) {
			existing.Spec.Template.Spec.Containers[0].VolumeMounts = ss.Spec.Template.Spec.Containers[0].VolumeMounts
			changed = true
		}

		if changed {
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // StatefulSet found with nothing to do, move along...
	}

	// Delete existing deployment for Application Controller, if any ..
	deploy := argocd.NewDeploymentWithSuffix("application-controller", "application-controller", cr)
	if argoutil.IsObjectFound(r.Client, deploy.Namespace, deploy.Name, deploy) {
		r.Client.Delete(context.TODO(), deploy)
	}

	if err := controllerutil.SetControllerReference(cr, ss, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ss)
}

func (r *ArgoCDReconciler) getArgoCDExport(cr *argoprojv1a1.ArgoCD) *argoprojv1a1.ArgoCDExport {
	if cr.Spec.Import == nil {
		return nil
	}

	namespace := cr.ObjectMeta.Namespace
	if cr.Spec.Import.Namespace != nil && len(*cr.Spec.Import.Namespace) > 0 {
		namespace = *cr.Spec.Import.Namespace
	}

	export := &argoprojv1a1.ArgoCDExport{}
	if argoutil.IsObjectFound(r.Client, namespace, cr.Spec.Import.Name, export) {
		return export
	}
	return nil
}

// reconcileDeployments will ensure that all Deployment resources are present for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileDeployments(cr *argoprojv1a1.ArgoCD) error {
	err := r.reconcileDexDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRedisDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRedisHAProxyDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileRepoDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileServerDeployment(cr)
	if err != nil {
		return err
	}

	err = r.reconcileGrafanaDeployment(cr)
	if err != nil {
		return err
	}

	return nil
}

// reconcileGrafanaDeployment will ensure the Deployment resource is present for the ArgoCD Grafana component.
func (r *ArgoCDReconciler) reconcileGrafanaDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := argocd.NewDeploymentWithSuffix("grafana", "grafana", cr)
	deploy.Spec.Replicas = argocd.GetGrafanaReplicas(cr)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Image:           argocd.GetGrafanaContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "grafana",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 3000,
			},
		},
		Env:       argocd.ProxyEnvVars(),
		Resources: argocd.GetGrafanaResources(cr),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "grafana-config",
				MountPath: "/etc/grafana",
			}, {
				Name:      "grafana-datasources-config",
				MountPath: "/etc/grafana/provisioning/datasources",
			}, {
				Name:      "grafana-dashboards-config",
				MountPath: "/etc/grafana/provisioning/dashboards",
			}, {
				Name:      "grafana-dashboard-templates",
				MountPath: "/var/lib/grafana/dashboards",
			},
		},
	}}

	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "grafana-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: argocd.NameWithSuffix("grafana-config", cr),
					},
					Items: []corev1.KeyToPath{{
						Key:  "grafana.ini",
						Path: "grafana.ini",
					}},
				},
			},
		}, {
			Name: "grafana-datasources-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: argocd.NameWithSuffix("grafana-config", cr),
					},
					Items: []corev1.KeyToPath{{
						Key:  "datasource.yaml",
						Path: "datasource.yaml",
					}},
				},
			},
		}, {
			Name: "grafana-dashboards-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: argocd.NameWithSuffix("grafana-config", cr),
					},
					Items: []corev1.KeyToPath{{
						Key:  "provider.yaml",
						Path: "provider.yaml",
					}},
				},
			},
		}, {
			Name: "grafana-dashboard-templates",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: argocd.NameWithSuffix("grafana-dashboards", cr),
					},
				},
			},
		},
	}

	existing := argocd.NewDeploymentWithSuffix("grafana", "grafana", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		if !cr.Spec.Grafana.Enabled {
			// Deployment exists but enabled flag has been set to false, delete the Deployment
			return r.Client.Delete(context.TODO(), existing)
		}
		changed := false
		if argocd.HasGrafanaSpecChanged(existing, cr) {
			existing.Spec.Replicas = cr.Spec.Grafana.Size
			changed = true
		}

		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			deploy.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			changed = true
		}
		if changed {
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found, do nothing
	}

	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}
	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileServerDeployment will ensure the Deployment resource is present for the ArgoCD Server component.
func (r *ArgoCDReconciler) reconcileServerDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := argocd.NewDeploymentWithSuffix("server", "server", cr)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command:         argocd.GetArgoServerCommand(cr),
		Image:           argocd.GetArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Env:             argocd.ProxyEnvVars(),
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8080),
				},
			},
			InitialDelaySeconds: 3,
			PeriodSeconds:       30,
		},
		Name: "argocd-server",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 8080,
			}, {
				ContainerPort: 8083,
			},
		},
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8080),
				},
			},
			InitialDelaySeconds: 3,
			PeriodSeconds:       30,
		},
		Resources: argocd.GetArgoServerResources(cr),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "ssh-known-hosts",
				MountPath: "/app/config/ssh",
			}, {
				Name:      "tls-certs",
				MountPath: "/app/config/tls",
			},
			{
				Name:      "argocd-repo-server-tls",
				MountPath: "/app/config/server/tls",
			},
		},
	}}
	deploy.Spec.Template.Spec.ServiceAccountName = fmt.Sprintf("%s-%s", cr.Name, "argocd-server")
	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "ssh-known-hosts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDKnownHostsConfigMapName,
					},
				},
			},
		}, {
			Name: "tls-certs",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDTLSCertsConfigMapName,
					},
				},
			},
		}, {
			Name: "argocd-repo-server-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRepoServerTLSSecretName,
					Optional:   argocd.BoolPtr(true),
				},
			},
		},
	}

	existing := argocd.NewDeploymentWithSuffix("server", "server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := argocd.GetArgoContainerImage(cr)
		changed := false
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}
		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			deploy.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			changed = true
		}
		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Command,
			deploy.Spec.Template.Spec.Containers[0].Command) {
			existing.Spec.Template.Spec.Containers[0].Command = deploy.Spec.Template.Spec.Containers[0].Command
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Volumes, existing.Spec.Template.Spec.Volumes) {
			existing.Spec.Template.Spec.Volumes = deploy.Spec.Template.Spec.Volumes
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].VolumeMounts,
			existing.Spec.Template.Spec.Containers[0].VolumeMounts) {
			existing.Spec.Template.Spec.Containers[0].VolumeMounts = deploy.Spec.Template.Spec.Containers[0].VolumeMounts
			changed = true
		}
		if changed {
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileRepoDeployment will ensure the Deployment resource is present for the ArgoCD Repo component.
func (r *ArgoCDReconciler) reconcileRepoDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := argocd.NewDeploymentWithSuffix("repo-server", "repo-server", cr)
	automountToken := false
	if cr.Spec.Repo.MountSAToken {
		automountToken = cr.Spec.Repo.MountSAToken
	}

	deploy.Spec.Template.Spec.AutomountServiceAccountToken = &automountToken

	if cr.Spec.Repo.ServiceAccount != "" {
		deploy.Spec.Template.Spec.ServiceAccountName = cr.Spec.Repo.ServiceAccount
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command:         argocd.GetArgoRepoCommand(cr),
		Image:           argocd.GetArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(common.ArgoCDDefaultRepoServerPort),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		Env:  argocd.ProxyEnvVars(),
		Name: "argocd-repo-server",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultRepoServerPort,
				Name:          "server",
			}, {
				ContainerPort: common.ArgoCDDefaultRepoMetricsPort,
				Name:          "metrics",
			},
		},
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(common.ArgoCDDefaultRepoServerPort),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		Resources: argocd.GetArgoRepoResources(cr),
		VolumeMounts: []corev1.VolumeMount{
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
				Name:      "argocd-repo-server-tls",
				MountPath: "/app/config/reposerver/tls",
			},
		},
	}}

	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
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
			Name: "argocd-repo-server-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRepoServerTLSSecretName,
					Optional:   argocd.BoolPtr(true),
				},
			},
		},
	}

	existing := argocd.NewDeploymentWithSuffix("repo-server", "repo-server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		changed := false
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := argocd.GetArgoContainerImage(cr)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Volumes, existing.Spec.Template.Spec.Volumes) {
			existing.Spec.Template.Spec.Volumes = deploy.Spec.Template.Spec.Volumes
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].VolumeMounts,
			existing.Spec.Template.Spec.Containers[0].VolumeMounts) {
			existing.Spec.Template.Spec.Containers[0].VolumeMounts = deploy.Spec.Template.Spec.Containers[0].VolumeMounts
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Env,
			existing.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			changed = true
		}

		if changed {
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileRedisHAProxyDeployment will ensure the Deployment resource is present for the Redis HA Proxy component.
func (r *ArgoCDReconciler) reconcileRedisHAProxyDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := argocd.NewDeploymentWithSuffix("redis-ha-haproxy", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		if !cr.Spec.HA.Enabled {
			// Deployment exists but HA enabled flag has been set to false, delete the Deployment
			return r.Client.Delete(context.TODO(), deploy)
		}

		actualImage := deploy.Spec.Template.Spec.Containers[0].Image
		desiredImage := argocd.GetRedisHAProxyContainerImage(cr)

		if actualImage != desiredImage {
			deploy.Spec.Template.Spec.Containers[0].Image = desiredImage
			deploy.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			return r.Client.Update(context.TODO(), deploy)
		}
		return nil // Deployment found, do nothing
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

	deploy.Spec.Template.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								common.ArgoCDKeyName: argocd.NameWithSuffix("redis-ha-haproxy", cr),
							},
						},
						TopologyKey: common.ArgoCDKeyFailureDomainZone,
					},
					Weight: int32(100),
				},
			},
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.ArgoCDKeyName: argocd.NameWithSuffix("redis-ha-haproxy", cr),
						},
					},
					TopologyKey: common.ArgoCDKeyHostname,
				},
			},
		},
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Image:           argocd.GetRedisHAProxyContainerImage(cr),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Name:            "haproxy",
		Env:             argocd.ProxyEnvVars(),
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8888),
				},
			},
			InitialDelaySeconds: int32(5),
			PeriodSeconds:       int32(3),
		},
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultRedisPort,
				Name:          "redis",
			},
		},
		Resources: argocd.GetRedisHAProxyResources(cr),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "data",
				MountPath: "/usr/local/etc/haproxy",
			},
			{
				Name:      "shared-socket",
				MountPath: "/run/haproxy",
			},
		},
	}}

	deploy.Spec.Template.Spec.InitContainers = []corev1.Container{{
		Args: []string{
			"/readonly/haproxy_init.sh",
		},
		Command: []string{
			"sh",
		},
		Image:           argocd.GetRedisHAProxyContainerImage(cr),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Name:            "config-init",
		Env:             argocd.ProxyEnvVars(),
		Resources:       argocd.GetRedisHAProxyResources(cr),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "config-volume",
				MountPath: "/readonly",
				ReadOnly:  true,
			},
			{
				Name:      "data",
				MountPath: "/data",
			},
		},
	}}

	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDRedisHAConfigMapName,
					},
				},
			},
		},
		{
			Name: "shared-socket",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	deploy.Spec.Template.Spec.ServiceAccountName = fmt.Sprintf("%s-%s", cr.Name, "argocd-redis-ha")

	if err := argocd.ApplyReconcilerHook(cr, deploy, ""); err != nil {
		return err
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileRedisDeployment will ensure the Deployment resource is present for the ArgoCD Redis component.
func (r *ArgoCDReconciler) reconcileRedisDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := argocd.NewDeploymentWithSuffix("redis", "redis", cr)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Args: []string{
			"--save",
			"",
			"--appendonly",
			"no",
		},
		Image:           argocd.GetRedisContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "redis",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultRedisPort,
			},
		},
		Resources: argocd.GetRedisResources(cr),
		Env:       argocd.ProxyEnvVars(),
	}}

	if err := argocd.ApplyReconcilerHook(cr, deploy, ""); err != nil {
		return err
	}

	existing := argocd.NewDeploymentWithSuffix("redis", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		if cr.Spec.HA.Enabled {
			// Deployment exists but HA enabled flag has been set to true, delete the Deployment
			return r.Client.Delete(context.TODO(), deploy)
		}
		changed := false
		actualImage := deploy.Spec.Template.Spec.Containers[0].Image
		desiredImage := argocd.GetRedisContainerImage(cr)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}

		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			deploy.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			changed = true
		}

		if changed {
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if cr.Spec.HA.Enabled {
		return nil // HA enabled, do nothing.
	}
	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileDexDeployment will ensure the Deployment resource is present for the ArgoCD Dex component.
func (r *ArgoCDReconciler) reconcileDexDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := argocd.NewDeploymentWithSuffix("dex-server", "dex-server", cr)
	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command: []string{
			"/shared/argocd-dex",
			"rundex",
		},
		Image:           argocd.GetDexContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "dex",
		Env:             argocd.ProxyEnvVars(),
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultDexHTTPPort,
				Name:          "http",
			}, {
				ContainerPort: common.ArgoCDDefaultDexGRPCPort,
				Name:          "grpc",
			},
		},
		Resources: argocd.GetDexResources(cr),
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "static-files",
			MountPath: "/shared",
		}},
	}}

	deploy.Spec.Template.Spec.InitContainers = []corev1.Container{{
		Command: []string{
			"cp",
			"-n",
			"/usr/local/bin/argocd",
			"/shared/argocd-dex",
		},
		Env:             argocd.ProxyEnvVars(),
		Image:           argocd.GetArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "copyutil",
		Resources:       argocd.GetDexResources(cr),
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "static-files",
			MountPath: "/shared",
		}},
	}}

	deploy.Spec.Template.Spec.ServiceAccountName = fmt.Sprintf("%s-%s", cr.Name, common.ArgoCDDefaultDexServiceAccountName)
	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{{
		Name: "static-files",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}}
	dexDisabled := argocd.IsDexDisabled()
	if dexDisabled {
		logr.Info("reconciling for dex, but dex is disabled")
	}

	existing := argocd.NewDeploymentWithSuffix("dex-server", "dex-server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		if dexDisabled {
			logr.Info("deleting the existing dex deployment because dex is disabled")
			// Deployment exists but enabled flag has been set to false, delete the Deployment
			return r.Client.Delete(context.TODO(), existing)
		}
		changed := false

		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := argocd.GetDexContainerImage(cr)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}

		actualImage = existing.Spec.Template.Spec.InitContainers[0].Image
		desiredImage = argocd.GetArgoContainerImage(cr)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.InitContainers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}

		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			deploy.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			changed = true
		}

		if !reflect.DeepEqual(existing.Spec.Template.Spec.InitContainers[0].Env,
			deploy.Spec.Template.Spec.InitContainers[0].Env) {
			existing.Spec.Template.Spec.InitContainers[0].Env = deploy.Spec.Template.Spec.InitContainers[0].Env
			changed = true
		}

		if changed {
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if dexDisabled {
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileServices will ensure that all Services are present for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileServices(cr *argoprojv1a1.ArgoCD) error {
	err := r.reconcileDexService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileGrafanaService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileMetricsService(cr)
	if err != nil {
		return err
	}

	if cr.Spec.HA.Enabled {
		err = r.reconcileRedisHAServices(cr)
		if err != nil {
			return err
		}
	} else {
		err = r.reconcileRedisService(cr)
		if err != nil {
			return err
		}
	}

	err = r.reconcileRepoService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileServerMetricsService(cr)
	if err != nil {
		return err
	}

	err = r.reconcileServerService(cr)
	if err != nil {
		return err
	}
	return nil
}

// reconcileServerService will ensure that the Service is present for the Argo CD server component.
func (r *ArgoCDReconciler) reconcileServerService(cr *argoprojv1a1.ArgoCD) error {
	svc := argocd.NewServiceWithSuffix("server", "server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http",
			Port:       80,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8080),
		}, {
			Name:       "https",
			Port:       443,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8080),
		},
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: argocd.NameWithSuffix("server", cr),
	}

	svc.Spec.Type = argocd.GetArgoServerServiceType(cr)

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileServerMetricsService will ensure that the Service for the Argo CD server metrics is present.
func (r *ArgoCDReconciler) reconcileServerMetricsService(cr *argoprojv1a1.ArgoCD) error {
	svc := argocd.NewServiceWithSuffix("server-metrics", "server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: argocd.NameWithSuffix("server", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       8083,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8083),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileServerMetricsServiceMonitor will ensure that the ServiceMonitor is present for the ArgoCD Server metrics Service.
func (r *ArgoCDReconciler) reconcileServerMetricsServiceMonitor(cr *argoprojv1a1.ArgoCD) error {
	sm := argocd.NewServiceMonitorWithSuffix("server-metrics", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, sm.Name, sm) {
		if !cr.Spec.Prometheus.Enabled {
			// ServiceMonitor exists but enabled flag has been set to false, delete the ServiceMonitor
			return r.Client.Delete(context.TODO(), sm)
		}
		return nil // ServiceMonitor found, do nothing
	}

	if !cr.Spec.Prometheus.Enabled {
		return nil // Prometheus not enabled, do nothing.
	}

	sm.Spec.Selector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.ArgoCDKeyName: argocd.NameWithSuffix("server-metrics", cr),
		},
	}
	sm.Spec.Endpoints = []monitoringv1.Endpoint{
		{
			Port: common.ArgoCDKeyMetrics,
		},
	}

	if err := controllerutil.SetControllerReference(cr, sm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), sm)
}

// reconcileRepoService will ensure that the Service for the Argo CD repo server is present.
func (r *ArgoCDReconciler) reconcileRepoService(cr *argoprojv1a1.ArgoCD) error {
	svc := argocd.NewServiceWithSuffix("repo-server", "repo-server", cr)

	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		if argocd.EnsureAutoTLSAnnotation(cr, svc) {
			return r.Client.Update(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	argocd.EnsureAutoTLSAnnotation(cr, svc)

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: argocd.NameWithSuffix("repo-server", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "server",
			Port:       common.ArgoCDDefaultRepoServerPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ArgoCDDefaultRepoServerPort),
		}, {
			Name:       "metrics",
			Port:       common.ArgoCDDefaultRepoMetricsPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ArgoCDDefaultRepoMetricsPort),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileRedisService will ensure that the Service for Redis is present.
func (r *ArgoCDReconciler) reconcileRedisService(cr *argoprojv1a1.ArgoCD) error {
	svc := argocd.NewServiceWithSuffix("redis", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: argocd.NameWithSuffix("redis", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "tcp-redis",
			Port:       common.ArgoCDDefaultRedisPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ArgoCDDefaultRedisPort),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileRedisHAServices will ensure that all required Services are present for Redis when running in HA mode.
func (r *ArgoCDReconciler) reconcileRedisHAServices(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileRedisHAAnnounceServices(cr); err != nil {
		return err
	}

	if err := r.reconcileRedisHAMasterService(cr); err != nil {
		return err
	}

	if err := r.reconcileRedisHAProxyService(cr); err != nil {
		return err
	}
	return nil
}

func (r *ArgoCDReconciler) reconcileRedisHAProxyService(cr *argoprojv1a1.ArgoCD) error {
	svc := argocd.NewServiceWithSuffix("redis-ha-haproxy", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: argocd.NameWithSuffix("redis-ha-haproxy", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "haproxy",
			Port:       common.ArgoCDDefaultRedisPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString("redis"),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileRedisHAMasterService will ensure that the "master" Service is present for Redis when running in HA mode.
func (r *ArgoCDReconciler) reconcileRedisHAMasterService(cr *argoprojv1a1.ArgoCD) error {
	svc := argocd.NewServiceWithSuffix("redis-ha", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		return nil // Service found, do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: argocd.NameWithSuffix("redis-ha", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "server",
			Port:       common.ArgoCDDefaultRedisPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString("redis"),
		}, {
			Name:       "sentinel",
			Port:       common.ArgoCDDefaultRedisSentinelPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString("sentinel"),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileRedisHAAnnounceServices will ensure that the announce Services are present for Redis when running in HA mode.
func (r *ArgoCDReconciler) reconcileRedisHAAnnounceServices(cr *argoprojv1a1.ArgoCD) error {
	for i := int32(0); i < common.ArgoCDDefaultRedisHAReplicas; i++ {
		svc := argocd.NewServiceWithSuffix(fmt.Sprintf("redis-ha-announce-%d", i), "redis", cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
			return nil // Service found, do nothing
		}

		svc.ObjectMeta.Annotations = map[string]string{
			common.ArgoCDKeyTolerateUnreadyEndpounts: "true",
		}

		svc.Spec.PublishNotReadyAddresses = true

		svc.Spec.Selector = map[string]string{
			common.ArgoCDKeyName:               argocd.NameWithSuffix("redis-ha", cr),
			common.ArgoCDKeyStatefulSetPodName: argocd.NameWithSuffix(fmt.Sprintf("redis-ha-server-%d", i), cr),
		}

		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "server",
				Port:       common.ArgoCDDefaultRedisPort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromString("redis"),
			}, {
				Name:       "sentinel",
				Port:       common.ArgoCDDefaultRedisSentinelPort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromString("sentinel"),
			},
		}

		if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
			return err
		}

		if err := r.Client.Create(context.TODO(), svc); err != nil {
			return err
		}
	}
	return nil
}

// reconcileMetricsService will ensure that the Service for the Argo CD application controller metrics is present.
func (r *ArgoCDReconciler) reconcileMetricsService(cr *argoprojv1a1.ArgoCD) error {
	svc := argocd.NewServiceWithSuffix("metrics", "metrics", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		// Service found, do nothing
		return nil
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: argocd.NameWithSuffix("application-controller", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       8082,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8082),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileGrafanaService will ensure that the Service for Grafana is present.
func (r *ArgoCDReconciler) reconcileGrafanaService(cr *argoprojv1a1.ArgoCD) error {
	svc := argocd.NewServiceWithSuffix("grafana", "grafana", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		if !cr.Spec.Grafana.Enabled {
			// Service exists but enabled flag has been set to false, delete the Service
			return r.Client.Delete(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: argocd.NameWithSuffix("grafana", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http",
			Port:       80,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(3000),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileDexService will ensure that the Service for Dex is present.
func (r *ArgoCDReconciler) reconcileDexService(cr *argoprojv1a1.ArgoCD) error {
	svc := argocd.NewServiceWithSuffix("dex-server", "dex-server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		if argocd.IsDexDisabled() {
			// Service exists but enabled flag has been set to false, delete the Service
			return r.Client.Delete(context.TODO(), svc)
		}
		return nil
	}

	if argocd.IsDexDisabled() {
		return nil // Dex is disabled, do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: argocd.NameWithSuffix("dex-server", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http",
			Port:       common.ArgoCDDefaultDexHTTPPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ArgoCDDefaultDexHTTPPort),
		}, {
			Name:       "grpc",
			Port:       common.ArgoCDDefaultDexGRPCPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ArgoCDDefaultDexGRPCPort),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileConfigMaps will ensure that all ArgoCD ConfigMaps are present.
func (r *ArgoCDReconciler) reconcileConfigMaps(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileArgoConfigMap(cr); err != nil {
		return err
	}

	if err := r.reconcileRedisConfiguration(cr); err != nil {
		return err
	}

	if err := r.reconcileRBAC(cr); err != nil {
		return err
	}

	if err := r.reconcileSSHKnownHosts(cr); err != nil {
		return err
	}

	if err := r.reconcileTLSCerts(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaConfiguration(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaDashboards(cr); err != nil {
		return err
	}

	return r.reconcileGPGKeysConfigMap(cr)
}

// reconcileGPGKeysConfigMap creates a gpg-keys config map
func (r *ArgoCDReconciler) reconcileGPGKeysConfigMap(cr *argoprojv1a1.ArgoCD) error {
	cm := argocd.NewConfigMapWithName(common.ArgoCDGPGKeysConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil
	}
	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileGrafanaDashboards will ensure that the Grafana dashboards ConfigMap is present.
func (r *ArgoCDReconciler) reconcileGrafanaDashboards(cr *argoprojv1a1.ArgoCD) error {
	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	cm := argocd.NewConfigMapWithSuffix(common.ArgoCDGrafanaDashboardConfigMapSuffix, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	pattern := filepath.Join(argocd.GetGrafanaConfigPath(), "dashboards/*.json")
	dashboards, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	data := make(map[string]string)
	for _, f := range dashboards {
		dashboard, err := ioutil.ReadFile(f)
		if err != nil {
			return err
		}

		parts := strings.Split(f, "/")
		filename := parts[len(parts)-1]
		data[filename] = string(dashboard)
	}
	cm.Data = data

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileGrafanaConfiguration will ensure that the Grafana configuration ConfigMap is present.
func (r *ArgoCDReconciler) reconcileGrafanaConfiguration(cr *argoprojv1a1.ArgoCD) error {
	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	cm := argocd.NewConfigMapWithSuffix(common.ArgoCDGrafanaConfigMapSuffix, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	secret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "grafana")
	secret, err := argoutil.FetchSecret(r.Client, cr.ObjectMeta, secret.Name)
	if err != nil {
		return err
	}

	grafanaConfig := argocd.GrafanaConfig{
		Security: argocd.GrafanaSecurityConfig{
			AdminUser:     string(secret.Data[common.ArgoCDKeyGrafanaAdminUsername]),
			AdminPassword: string(secret.Data[common.ArgoCDKeyGrafanaAdminPassword]),
			SecretKey:     string(secret.Data[common.ArgoCDKeyGrafanaSecretKey]),
		},
	}

	data, err := argocd.LoadGrafanaConfigs()
	if err != nil {
		return err
	}

	tmpls, err := argocd.LoadGrafanaTemplates(&grafanaConfig)
	if err != nil {
		return err
	}

	for key, val := range tmpls {
		data[key] = val
	}
	cm.Data = data

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileTLSCerts will ensure that the ArgoCD TLS Certs ConfigMap is present.
func (r *ArgoCDReconciler) reconcileTLSCerts(cr *argoprojv1a1.ArgoCD) error {
	cm := argocd.NewConfigMapWithName(common.ArgoCDTLSCertsConfigMapName, cr)
	cm.Data = argocd.GetInitialTLSCerts(cr)
	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}

	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return r.Client.Update(context.TODO(), cm)
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileSSHKnownHosts will ensure that the ArgoCD SSH Known Hosts ConfigMap is present.
func (r *ArgoCDReconciler) reconcileSSHKnownHosts(cr *argoprojv1a1.ArgoCD) error {
	cm := argocd.NewConfigMapWithName(common.ArgoCDKnownHostsConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, move along...
	}

	cm.Data = map[string]string{
		common.ArgoCDKeySSHKnownHosts: argocd.GetInitialSSHKnownHosts(cr),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileRBAC will ensure that the ArgoCD RBAC ConfigMap is present.
func (r *ArgoCDReconciler) reconcileRBAC(cr *argoprojv1a1.ArgoCD) error {
	cm := argocd.NewConfigMapWithName(common.ArgoCDRBACConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return r.reconcileRBACConfigMap(cm, cr)
	}
	return r.createRBACConfigMap(cm, cr)
}

// createRBACConfigMap will create the Argo CD RBAC ConfigMap resource.
func (r *ArgoCDReconciler) createRBACConfigMap(cm *corev1.ConfigMap, cr *argoprojv1a1.ArgoCD) error {
	data := make(map[string]string)
	data[common.ArgoCDKeyRBACPolicyCSV] = argocd.GetRBACPolicy(cr)
	data[common.ArgoCDKeyRBACPolicyDefault] = argocd.GetRBACDefaultPolicy(cr)
	data[common.ArgoCDKeyRBACScopes] = argocd.GetRBACScopes(cr)
	cm.Data = data

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileRBACConfigMap will ensure that the RBAC ConfigMap is syncronized with the given ArgoCD.
func (r *ArgoCDReconciler) reconcileRBACConfigMap(cm *corev1.ConfigMap, cr *argoprojv1a1.ArgoCD) error {
	changed := false
	// Policy CSV
	if cr.Spec.RBAC.Policy != nil && cm.Data[common.ArgoCDKeyRBACPolicyCSV] != *cr.Spec.RBAC.Policy {
		cm.Data[common.ArgoCDKeyRBACPolicyCSV] = *cr.Spec.RBAC.Policy
		changed = true
	}

	// Default Policy
	if cr.Spec.RBAC.DefaultPolicy != nil && cm.Data[common.ArgoCDKeyRBACPolicyDefault] != *cr.Spec.RBAC.DefaultPolicy {
		cm.Data[common.ArgoCDKeyRBACPolicyDefault] = *cr.Spec.RBAC.DefaultPolicy
		changed = true
	}

	// Scopes
	if cr.Spec.RBAC.Scopes != nil && cm.Data[common.ArgoCDKeyRBACScopes] != *cr.Spec.RBAC.Scopes {
		cm.Data[common.ArgoCDKeyRBACScopes] = *cr.Spec.RBAC.Scopes
		changed = true
	}

	if changed {
		// TODO: Reload server (and dex?) if RBAC settings change?
		return r.Client.Update(context.TODO(), cm)
	}
	return nil // ConfigMap exists and nothing to do, move along...
}

// reconcileRedisConfiguration will ensure that all of the Redis ConfigMaps are present for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileRedisConfiguration(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileRedisHAConfigMap(cr); err != nil {
		return err
	}
	return nil
}

// reconcileRedisHAConfigMap will ensure that the Redis HA ConfigMap is present for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileRedisHAConfigMap(cr *argoprojv1a1.ArgoCD) error {
	cm := argocd.NewConfigMapWithName(common.ArgoCDRedisHAConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if !cr.Spec.HA.Enabled {
			// ConfigMap exists but HA enabled flag has been set to false, delete the ConfigMap
			return r.Client.Delete(context.TODO(), cm)
		}
		return nil // ConfigMap found with nothing changed, move along...
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

	cm.Data = map[string]string{
		"haproxy.cfg":     argocd.GetRedisHAProxyConfig(cr),
		"haproxy_init.sh": argocd.GetRedisHAProxyScript(cr),
		"init.sh":         argocd.GetRedisInitScript(cr),
		"redis.conf":      argocd.GetRedisConf(cr),
		"sentinel.conf":   argocd.GetRedisSentinelConf(cr),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileConfiguration will ensure that the main ConfigMap for ArgoCD is present.
func (r *ArgoCDReconciler) reconcileArgoConfigMap(cr *argoprojv1a1.ArgoCD) error {
	cm := argocd.NewConfigMapWithName(common.ArgoCDConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if err := r.reconcileDexConfiguration(cm, cr); err != nil {
			return err
		}
		return r.reconcileExistingArgoConfigMap(cm, cr)
	}

	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}

	cm.Data[common.ArgoCDKeyApplicationInstanceLabelKey] = argocd.GetApplicationInstanceLabelKey(cr)
	cm.Data[common.ArgoCDKeyConfigManagementPlugins] = argocd.GetConfigManagementPlugins(cr)
	cm.Data[common.ArgoCDKeyAdminEnabled] = fmt.Sprintf("%t", !cr.Spec.DisableAdmin)
	cm.Data[common.ArgoCDKeyGATrackingID] = argocd.GetGATrackingID(cr)
	cm.Data[common.ArgoCDKeyGAAnonymizeUsers] = fmt.Sprint(cr.Spec.GAAnonymizeUsers)
	cm.Data[common.ArgoCDKeyHelpChatURL] = argocd.GetHelpChatURL(cr)
	cm.Data[common.ArgoCDKeyHelpChatText] = argocd.GetHelpChatText(cr)
	cm.Data[common.ArgoCDKeyKustomizeBuildOptions] = argocd.GetKustomizeBuildOptions(cr)
	cm.Data[common.ArgoCDKeyOIDCConfig] = argocd.GetOIDCConfig(cr)
	if c := argocd.GetResourceCustomizations(cr); c != "" {
		cm.Data[common.ArgoCDKeyResourceCustomizations] = c
	}
	cm.Data[common.ArgoCDKeyResourceExclusions] = argocd.GetResourceExclusions(cr)
	cm.Data[common.ArgoCDKeyResourceInclusions] = argocd.GetResourceInclusions(cr)
	cm.Data[common.ArgoCDKeyRepositories] = argocd.GetInitialRepositories(cr)
	cm.Data[common.ArgoCDKeyRepositoryCredentials] = argocd.GetRepositoryCredentials(cr)
	cm.Data[common.ArgoCDKeyStatusBadgeEnabled] = fmt.Sprint(cr.Spec.StatusBadgeEnabled)
	cm.Data[common.ArgoCDKeyServerURL] = r.getArgoServerURI(cr)
	cm.Data[common.ArgoCDKeyUsersAnonymousEnabled] = fmt.Sprint(cr.Spec.UsersAnonymousEnabled)

	if !argocd.IsDexDisabled() {
		dexConfig := argocd.GetDexConfig(cr)
		if dexConfig == "" && cr.Spec.Dex.OpenShiftOAuth {
			cfg, err := r.getOpenShiftDexConfig(cr)
			if err != nil {
				return err
			}
			dexConfig = cfg
		}
		cm.Data[common.ArgoCDKeyDexConfig] = dexConfig
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

func (r *ArgoCDReconciler) reconcileExistingArgoConfigMap(cm *corev1.ConfigMap, cr *argoprojv1a1.ArgoCD) error {
	changed := false

	if cm.Data[common.ArgoCDKeyAdminEnabled] == fmt.Sprintf("%t", cr.Spec.DisableAdmin) {
		cm.Data[common.ArgoCDKeyAdminEnabled] = fmt.Sprintf("%t", !cr.Spec.DisableAdmin)
		changed = true
	}

	if cm.Data[common.ArgoCDKeyApplicationInstanceLabelKey] != cr.Spec.ApplicationInstanceLabelKey {
		cm.Data[common.ArgoCDKeyApplicationInstanceLabelKey] = cr.Spec.ApplicationInstanceLabelKey
		changed = true
	}

	if cm.Data[common.ArgoCDKeyConfigManagementPlugins] != cr.Spec.ConfigManagementPlugins {
		cm.Data[common.ArgoCDKeyConfigManagementPlugins] = cr.Spec.ConfigManagementPlugins
		changed = true
	}

	if cm.Data[common.ArgoCDKeyGATrackingID] != cr.Spec.GATrackingID {
		cm.Data[common.ArgoCDKeyGATrackingID] = cr.Spec.GATrackingID
		changed = true
	}

	if cm.Data[common.ArgoCDKeyGAAnonymizeUsers] != fmt.Sprint(cr.Spec.GAAnonymizeUsers) {
		cm.Data[common.ArgoCDKeyGAAnonymizeUsers] = fmt.Sprint(cr.Spec.GAAnonymizeUsers)
		changed = true
	}

	if cm.Data[common.ArgoCDKeyHelpChatURL] != cr.Spec.HelpChatURL {
		cm.Data[common.ArgoCDKeyHelpChatURL] = cr.Spec.HelpChatURL
		changed = true
	}

	if cm.Data[common.ArgoCDKeyHelpChatText] != cr.Spec.HelpChatText {
		cm.Data[common.ArgoCDKeyHelpChatText] = cr.Spec.HelpChatText
		changed = true
	}

	if cm.Data[common.ArgoCDKeyKustomizeBuildOptions] != cr.Spec.KustomizeBuildOptions {
		cm.Data[common.ArgoCDKeyKustomizeBuildOptions] = cr.Spec.KustomizeBuildOptions
		changed = true
	}

	if cr.Spec.SSO == nil {
		if cm.Data[common.ArgoCDKeyOIDCConfig] != cr.Spec.OIDCConfig {
			cm.Data[common.ArgoCDKeyOIDCConfig] = cr.Spec.OIDCConfig
			changed = true
		}
	}

	if cm.Data[common.ArgoCDKeyResourceCustomizations] != cr.Spec.ResourceCustomizations {
		cm.Data[common.ArgoCDKeyResourceCustomizations] = cr.Spec.ResourceCustomizations
		changed = true
	}

	if cm.Data[common.ArgoCDKeyResourceExclusions] != cr.Spec.ResourceExclusions {
		cm.Data[common.ArgoCDKeyResourceExclusions] = cr.Spec.ResourceExclusions
		changed = true
	}

	uri := r.getArgoServerURI(cr)
	if cm.Data[common.ArgoCDKeyServerURL] != uri {
		cm.Data[common.ArgoCDKeyServerURL] = uri
		changed = true
	}

	if cm.Data[common.ArgoCDKeyStatusBadgeEnabled] != fmt.Sprint(cr.Spec.StatusBadgeEnabled) {
		cm.Data[common.ArgoCDKeyStatusBadgeEnabled] = fmt.Sprint(cr.Spec.StatusBadgeEnabled)
		changed = true
	}

	if cm.Data[common.ArgoCDKeyUsersAnonymousEnabled] != fmt.Sprint(cr.Spec.UsersAnonymousEnabled) {
		cm.Data[common.ArgoCDKeyUsersAnonymousEnabled] = fmt.Sprint(cr.Spec.UsersAnonymousEnabled)
		changed = true
	}

	if cm.Data[common.ArgoCDKeyRepositoryCredentials] != cr.Spec.RepositoryCredentials {
		cm.Data[common.ArgoCDKeyRepositoryCredentials] = cr.Spec.RepositoryCredentials
		changed = true
	}

	if changed {
		return r.Client.Update(context.TODO(), cm) // TODO: Reload Argo CD server after ConfigMap change (which properties)?
	}

	return nil // Nothing changed, no update needed...
}

// reconcileDexConfiguration will ensure that Dex is configured properly.
func (r *ArgoCDReconciler) reconcileDexConfiguration(cm *corev1.ConfigMap, cr *argoprojv1a1.ArgoCD) error {
	actual := cm.Data[common.ArgoCDKeyDexConfig]
	desired := argocd.GetDexConfig(cr)
	if len(desired) <= 0 && cr.Spec.Dex.OpenShiftOAuth {
		cfg, err := r.getOpenShiftDexConfig(cr)
		if err != nil {
			return err
		}
		desired = cfg
	}

	if actual != desired {
		// Update ConfigMap with desired configuration.
		cm.Data[common.ArgoCDKeyDexConfig] = desired
		if err := r.Client.Update(context.TODO(), cm); err != nil {
			return err
		}

		// Trigger rollout of Dex Deployment to pick up changes.
		deploy := argocd.NewDeploymentWithSuffix("dex-server", "dex-server", cr)
		if !argoutil.IsObjectFound(r.Client, deploy.Namespace, deploy.Name, deploy) {
			logr.Info("unable to locate dex deployment")
			return nil
		}

		deploy.Spec.Template.ObjectMeta.Labels["dex.config.changed"] = time.Now().UTC().Format("01022006-150406-MST")
		return r.Client.Update(context.TODO(), deploy)
	}
	return nil
}

// getOpenShiftDexConfig will return the configuration for the Dex server running on OpenShift.
func (r *ArgoCDReconciler) getOpenShiftDexConfig(cr *argoprojv1a1.ArgoCD) (string, error) {
	clientSecret, err := r.getDexOAuthClientSecret(cr)
	if err != nil {
		return "", err
	}

	connector := argocd.DexConnector{
		Type: "openshift",
		ID:   "openshift",
		Name: "OpenShift",
		Config: map[string]interface{}{
			"issuer":       "https://kubernetes.default.svc", // TODO: Should this be hard-coded?
			"clientID":     argocd.GetDexOAuthClientID(cr),
			"clientSecret": *clientSecret,
			"redirectURI":  r.getDexOAuthRedirectURI(cr),
			"insecureCA":   true, // TODO: Configure for openshift CA
		},
	}

	connectors := make([]argocd.DexConnector, 0)
	connectors = append(connectors, connector)

	dex := make(map[string]interface{})
	dex["connectors"] = connectors

	bytes, err := yaml.Marshal(dex)
	return string(bytes), err
}

// getDexOAuthClientSecret will return the OAuth client secret for the given ArgoCD.
func (r *ArgoCDReconciler) getDexOAuthClientSecret(cr *argoprojv1a1.ArgoCD) (*string, error) {
	sa := argocd.NewServiceAccountWithName(common.ArgoCDDefaultDexServiceAccountName, cr)
	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		return nil, err
	}

	// Find the token secret
	var tokenSecret *corev1.ObjectReference
	for _, saSecret := range sa.Secrets {
		if strings.Contains(saSecret.Name, "token") {
			tokenSecret = &saSecret
			break
		}
	}

	if tokenSecret == nil {
		return nil, errors.New("unable to locate ServiceAccount token for OAuth client secret")
	}

	// Fetch the secret to obtain the token
	secret := argoutil.NewSecretWithName(cr.ObjectMeta, tokenSecret.Name)
	if err := argoutil.FetchObject(r.Client, cr.Namespace, secret.Name, secret); err != nil {
		return nil, err
	}

	token := string(secret.Data["token"])
	return &token, nil
}

func (r *ArgoCDReconciler) reconcileSecrets(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileClusterSecrets(cr); err != nil {
		return err
	}

	if err := r.reconcileArgoSecret(cr); err != nil {
		return err
	}

	return nil
}

// reconcileArgoSecret will ensure that the Argo CD Secret is present.
func (r *ArgoCDReconciler) reconcileArgoSecret(cr *argoprojv1a1.ArgoCD) error {
	clusterSecret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "cluster")
	secret := argoutil.NewSecretWithName(cr.ObjectMeta, common.ArgoCDSecretName)

	if !argoutil.IsObjectFound(r.Client, cr.Namespace, clusterSecret.Name, clusterSecret) {
		logr.Info(fmt.Sprintf("cluster secret [%s] not found, waiting to reconcile argo secret [%s]", clusterSecret.Name, secret.Name))
		return nil
	}

	tlsSecret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "tls")
	if !argoutil.IsObjectFound(r.Client, cr.Namespace, tlsSecret.Name, tlsSecret) {
		logr.Info(fmt.Sprintf("tls secret [%s] not found, waiting to reconcile argo secret [%s]", tlsSecret.Name, secret.Name))
		return nil
	}

	if argoutil.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return r.reconcileExistingArgoSecret(cr, secret, clusterSecret, tlsSecret)
	}

	// Secret not found, create it...
	hashedPassword, err := argopass.HashPassword(string(clusterSecret.Data[common.ArgoCDKeyAdminPassword]))
	if err != nil {
		return err
	}

	sessionKey, err := argocd.GenerateArgoServerSessionKey()
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		common.ArgoCDKeyAdminPassword:      []byte(hashedPassword),
		common.ArgoCDKeyAdminPasswordMTime: argocd.NowBytes(),
		common.ArgoCDKeyServerSecretKey:    sessionKey,
		common.ArgoCDKeyTLSCert:            tlsSecret.Data[common.ArgoCDKeyTLSCert],
		common.ArgoCDKeyTLSPrivateKey:      tlsSecret.Data[common.ArgoCDKeyTLSPrivateKey],
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), secret)
}

// reconcileExistingArgoSecret will ensure that the Argo CD Secret is up to date.
func (r *ArgoCDReconciler) reconcileExistingArgoSecret(cr *argoprojv1a1.ArgoCD, secret *corev1.Secret, clusterSecret *corev1.Secret, tlsSecret *corev1.Secret) error {
	changed := false

	if argocd.HasArgoAdminPasswordChanged(secret, clusterSecret) {
		hashedPassword, err := argopass.HashPassword(string(clusterSecret.Data[common.ArgoCDKeyAdminPassword]))
		if err != nil {
			return err
		}

		secret.Data[common.ArgoCDKeyAdminPassword] = []byte(hashedPassword)
		secret.Data[common.ArgoCDKeyAdminPasswordMTime] = argocd.NowBytes()
		changed = true
	}

	if argocd.HasArgoTLSChanged(secret, tlsSecret) {
		secret.Data[common.ArgoCDKeyTLSCert] = tlsSecret.Data[common.ArgoCDKeyTLSCert]
		secret.Data[common.ArgoCDKeyTLSPrivateKey] = tlsSecret.Data[common.ArgoCDKeyTLSPrivateKey]
		changed = true
	}

	if changed {
		logr.Info("updating argo secret")
		if err := r.Client.Update(context.TODO(), secret); err != nil {
			return err
		}

		// Trigger rollout of Argo Server Deployment
		deploy := argocd.NewDeploymentWithSuffix("server", "server", cr)
		return r.triggerRollout(deploy, "secret.changed")
	}

	return nil
}

// reconcileClusterSecrets will reconcile all Secret resources for the ArgoCD cluster.
func (r *ArgoCDReconciler) reconcileClusterSecrets(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileClusterMainSecret(cr); err != nil {
		return err
	}

	if err := r.reconcileClusterCASecret(cr); err != nil {
		return err
	}

	if err := r.reconcileClusterTLSSecret(cr); err != nil {
		return err
	}

	if err := r.reconcileClusterPermissionsSecret(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaSecret(cr); err != nil {
		return err
	}

	return nil
}

// reconcileGrafanaSecret will ensure that the Grafana Secret is present.
func (r *ArgoCDReconciler) reconcileGrafanaSecret(cr *argoprojv1a1.ArgoCD) error {
	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	clusterSecret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "cluster")
	secret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "grafana")

	if !argoutil.IsObjectFound(r.Client, cr.Namespace, clusterSecret.Name, clusterSecret) {
		logr.Info(fmt.Sprintf("cluster secret [%s] not found, waiting to reconcile grafana secret [%s]", clusterSecret.Name, secret.Name))
		return nil
	}

	if argoutil.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		actual := string(secret.Data[common.ArgoCDKeyGrafanaAdminPassword])
		expected := string(clusterSecret.Data[common.ArgoCDKeyAdminPassword])

		if actual != expected {
			logr.Info("cluster secret changed, updating and reloading grafana")
			secret.Data[common.ArgoCDKeyGrafanaAdminPassword] = clusterSecret.Data[common.ArgoCDKeyAdminPassword]
			if err := r.Client.Update(context.TODO(), secret); err != nil {
				return err
			}

			// Regenerate the Grafana configuration
			cm := argocd.NewConfigMapWithSuffix("grafana-config", cr)
			if !argoutil.IsObjectFound(r.Client, cm.Namespace, cm.Name, cm) {
				logr.Info("unable to locate grafana-config")
				return nil
			}

			if err := r.Client.Delete(context.TODO(), cm); err != nil {
				return err
			}

			// Trigger rollout of Grafana Deployment
			deploy := argocd.NewDeploymentWithSuffix("grafana", "grafana", cr)
			return r.triggerRollout(deploy, "admin.password.changed")
		}
		return nil // Nothing has changed, move along...
	}

	// Secret not found, create it...

	secretKey, err := argocd.GenerateGrafanaSecretKey()
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		common.ArgoCDKeyGrafanaAdminUsername: []byte(common.ArgoCDDefaultGrafanaAdminUsername),
		common.ArgoCDKeyGrafanaAdminPassword: clusterSecret.Data[common.ArgoCDKeyAdminPassword],
		common.ArgoCDKeyGrafanaSecretKey:     secretKey,
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), secret)
}

// triggerRollout will trigger a rollout of a Kubernetes resource specified as
// obj. It currently supports Deployment and StatefulSet resources.
func (r *ArgoCDReconciler) triggerRollout(obj interface{}, key string) error {
	switch res := obj.(type) {
	case *appsv1.Deployment:
		return r.triggerDeploymentRollout(res, key)
	case *appsv1.StatefulSet:
		return r.triggerStatefulSetRollout(res, key)
	default:
		return fmt.Errorf("resource of unknown type %T, cannot trigger rollout", res)
	}
}

// triggerStatefulSetRollout will update the label with the given key to trigger a new rollout of the StatefulSet.
func (r *ArgoCDReconciler) triggerStatefulSetRollout(sts *appsv1.StatefulSet, key string) error {
	if !argoutil.IsObjectFound(r.Client, sts.Namespace, sts.Name, sts) {
		logr.Info(fmt.Sprintf("unable to locate deployment with name: %s", sts.Name))
		return nil
	}

	sts.Spec.Template.ObjectMeta.Labels[key] = argocd.NowNano()
	return r.Client.Update(context.TODO(), sts)
}

// triggerDeploymentRollout will update the label with the given key to trigger a new rollout of the Deployment.
func (r *ArgoCDReconciler) triggerDeploymentRollout(deployment *appsv1.Deployment, key string) error {
	if !argoutil.IsObjectFound(r.Client, deployment.Namespace, deployment.Name, deployment) {
		logr.Info(fmt.Sprintf("unable to locate deployment with name: %s", deployment.Name))
		return nil
	}

	deployment.Spec.Template.ObjectMeta.Labels[key] = argocd.NowNano()
	return r.Client.Update(context.TODO(), deployment)
}

// reconcileClusterPermissionsSecret ensures ArgoCD instance is namespace-scoped
func (r *ArgoCDReconciler) reconcileClusterPermissionsSecret(cr *argoprojv1a1.ArgoCD) error {
	var clusterConfigInstance bool
	secret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "default-cluster-config")
	secret.Labels[common.ArgoCDSecretTypeLabel] = "cluster"
	dataBytes, _ := json.Marshal(map[string]interface{}{
		"tlsClientConfig": map[string]interface{}{
			"insecure": false,
		},
	})

	secret.Data = map[string][]byte{
		"config":     dataBytes,
		"name":       []byte("in-cluster"),
		"server":     []byte(common.ArgoCDDefaultServer),
		"namespaces": []byte(cr.Namespace),
	}

	if argocd.AllowedNamespace(cr.Namespace, os.Getenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")) {
		clusterConfigInstance = true
	}

	clusterSecrets := &corev1.SecretList{}
	opts := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			common.ArgoCDSecretTypeLabel: "cluster",
		}),
		Namespace: cr.Namespace,
	}

	if err := r.Client.List(context.TODO(), clusterSecrets, opts); err != nil {
		return err
	}
	for _, s := range clusterSecrets.Items {
		// check if cluster secret with default server address exists
		// do nothing if exists.
		if string(s.Data["server"]) == common.ArgoCDDefaultServer {
			if clusterConfigInstance {
				r.Client.Delete(context.TODO(), &s)
			} else {
				return nil
			}
		}
	}

	if clusterConfigInstance {
		// do nothing
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), secret)
}

// reconcileClusterTLSSecret ensures the TLS Secret is created for the ArgoCD cluster.
func (r *ArgoCDReconciler) reconcileClusterTLSSecret(cr *argoprojv1a1.ArgoCD) error {
	secret := argoutil.NewTLSSecret(cr.ObjectMeta, "tls")
	if argoutil.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return nil // Secret found, do nothing
	}

	caSecret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "ca")
	caSecret, err := argoutil.FetchSecret(r.Client, cr.ObjectMeta, caSecret.Name)
	if err != nil {
		return err
	}

	caCert, err := argoutil.ParsePEMEncodedCert(caSecret.Data[corev1.TLSCertKey])
	if err != nil {
		return err
	}

	caKey, err := argoutil.ParsePEMEncodedPrivateKey(caSecret.Data[corev1.TLSPrivateKeyKey])
	if err != nil {
		return err
	}

	secret, err = argocd.NewCertificateSecret("tls", caCert, caKey, cr)
	if err != nil {
		return err
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}

	return r.Client.Create(context.TODO(), secret)
}

// reconcileClusterMainSecret will ensure that the main Secret is present for the Argo CD cluster.
func (r *ArgoCDReconciler) reconcileClusterMainSecret(cr *argoprojv1a1.ArgoCD) error {
	secret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "cluster")
	if argoutil.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return nil // Secret found, do nothing
	}

	adminPassword, err := argocd.GenerateArgoAdminPassword()
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		common.ArgoCDKeyAdminPassword: adminPassword,
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), secret)
}

// reconcileCertificateAuthority will reconcile all Certificate Authority resources.
func (r *ArgoCDReconciler) reconcileCertificateAuthority(cr *argoprojv1a1.ArgoCD) error {
	logr.Info("reconciling CA secret")
	if err := r.reconcileClusterCASecret(cr); err != nil {
		return err
	}

	logr.Info("reconciling CA config map")
	if err := r.reconcileCAConfigMap(cr); err != nil {
		return err
	}
	return nil
}

// reconcileCAConfigMap will ensure that the Certificate Authority ConfigMap is present.
// This ConfigMap holds the CA Certificate data for client use.
func (r *ArgoCDReconciler) reconcileCAConfigMap(cr *argoprojv1a1.ArgoCD) error {
	cm := argocd.NewConfigMapWithName(argocd.GetCAConfigMapName(cr), cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	caSecret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, common.ArgoCDCASuffix)
	if !argoutil.IsObjectFound(r.Client, cr.Namespace, caSecret.Name, caSecret) {
		logr.Info(fmt.Sprintf("ca secret [%s] not found, waiting to reconcile ca configmap [%s]", caSecret.Name, cm.Name))
		return nil
	}

	cm.Data = map[string]string{
		common.ArgoCDKeyTLSCert: string(caSecret.Data[common.ArgoCDKeyTLSCert]),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileClusterCASecret ensures the CA Secret is created for the ArgoCD cluster.
func (r *ArgoCDReconciler) reconcileClusterCASecret(cr *argoprojv1a1.ArgoCD) error {
	secret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "ca")
	if argoutil.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return nil // Secret found, do nothing
	}

	secret, err := argocd.NewCASecret(cr)
	if err != nil {
		return err
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), secret)
}

// reconcileServiceAccounts will ensure that all ArgoCD Service Accounts are configured.
func (r *ArgoCDReconciler) reconcileServiceAccounts(cr *argoprojv1a1.ArgoCD) error {

	if err := r.reconcileServiceAccountPermissions(common.ArgoCDServerComponent, argocd.PolicyRuleForServer(), cr); err != nil {
		return err
	}

	if err := r.reconcileServiceAccountPermissions(common.ArgoCDDexServerComponent, argocd.PolicyRuleForDexServer(), cr); err != nil {
		return err
	}

	if err := r.reconcileServiceAccountPermissions(common.ArgoCDApplicationControllerComponent, argocd.PolicyRuleForApplicationController(), cr); err != nil {
		return err
	}

	if err := r.reconcileServiceAccountPermissions(common.ArgoCDRedisHAComponent, argocd.PolicyRuleForRedisHa(cr), cr); err != nil {
		return err
	}

	if err := r.reconcileServiceAccountClusterPermissions(common.ArgoCDServerComponent, argocd.PolicyRuleForServerClusterRole(), cr); err != nil {
		return err
	}

	if err := r.reconcileServiceAccountClusterPermissions(common.ArgoCDApplicationControllerComponent, argocd.PolicyRuleForApplicationController(), cr); err != nil {
		return err
	}

	// specialized handling for dex

	if err := r.reconcileDexServiceAccount(cr); err != nil {
		return err
	}

	return nil
}

func (r *ArgoCDReconciler) reconcileServiceAccountClusterPermissions(name string, rules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) error {
	var role *v1.ClusterRole
	var sa *corev1.ServiceAccount
	var err error

	sa, err = r.reconcileServiceAccount(name, cr)
	if err != nil {
		return err
	}

	if role, err = r.reconcileClusterRole(name, rules, cr); err != nil {
		return err
	}

	return r.reconcileClusterRoleBinding(name, role, sa, cr)
}

func (r *ArgoCDReconciler) reconcileClusterRoleBinding(name string, role *v1.ClusterRole, sa *corev1.ServiceAccount, cr *argoprojv1a1.ArgoCD) error {

	// get expected name
	roleBinding := argocd.NewClusterRoleBindingWithname(name, cr)
	// fetch existing rolebinding by name
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name}, roleBinding)
	roleBindingExists := true
	if err != nil {
		if !crierrors.IsNotFound(err) {
			return err
		}
		roleBindingExists = false
		roleBinding = argocd.NewClusterRoleBindingWithname(name, cr)
	}

	if roleBindingExists && role == nil {
		return r.Client.Delete(context.TODO(), roleBinding)
	}

	if !roleBindingExists && role == nil {
		// DO Nothing
		return nil
	}

	roleBinding.Subjects = []v1.Subject{
		{
			Kind:      v1.ServiceAccountKind,
			Name:      argocd.GenerateResourceName(name, cr),
			Namespace: cr.Namespace,
		},
	}
	roleBinding.RoleRef = v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     "ClusterRole",
		Name:     argocd.GenerateUniqueResourceName(name, cr),
	}

	controllerutil.SetControllerReference(cr, roleBinding, r.Scheme)
	if roleBindingExists {
		return r.Client.Update(context.TODO(), roleBinding)
	}
	return r.Client.Create(context.TODO(), roleBinding)
}

func (r *ArgoCDReconciler) reconcileServiceAccountPermissions(name string, rules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) error {
	return r.reconcileRoleBinding(name, rules, cr)
}

// reconcileRoleBindings will ensure that all ArgoCD RoleBindings are configured.
func (r *ArgoCDReconciler) reconcileRoleBindings(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileRoleBinding(applicationController, argocd.PolicyRuleForApplicationController(), cr); err != nil {
		return fmt.Errorf("error reconciling roleBinding for %q: %w", applicationController, err)
	}
	if err := r.reconcileRoleBinding(dexServer, argocd.PolicyRuleForDexServer(), cr); err != nil {
		return fmt.Errorf("error reconciling roleBinding for %q: %w", dexServer, err)
	}

	if err := r.reconcileRoleBinding(redisHa, argocd.PolicyRuleForRedisHa(cr), cr); err != nil {
		return fmt.Errorf("error reconciling roleBinding for %q: %w", redisHa, err)
	}

	if err := r.reconcileRoleBinding(server, argocd.PolicyRuleForServer(), cr); err != nil {
		return fmt.Errorf("error reconciling roleBinding for %q: %w", server, err)
	}
	return nil
}

func (r *ArgoCDReconciler) reconcileRoleBinding(name string, rules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) error {
	var role *v1.Role
	var sa *corev1.ServiceAccount
	var error error

	if role, error = r.reconcileRole(name, rules, cr); error != nil {
		return error
	}

	if sa, error = r.reconcileServiceAccount(name, cr); error != nil {
		return error
	}

	// get expected name
	roleBinding := argocd.NewRoleBindingWithname(name, cr)

	// fetch existing rolebinding by name
	existingRoleBinding := &v1.RoleBinding{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: cr.Namespace}, existingRoleBinding)
	roleBindingExists := true
	if err != nil {
		if !crierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s : %s", name, err)
		}
		if name == dexServer && argocd.IsDexDisabled() {
			return nil // Dex is disabled, do nothing
		}
		roleBindingExists = false
		roleBinding = argocd.NewRoleBindingWithname(name, cr)
	}

	roleBinding.Subjects = []v1.Subject{
		{
			Kind:      v1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
	}
	roleBinding.RoleRef = v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     "Role",
		Name:     role.Name,
	}

	if roleBindingExists {
		if name == dexServer && argocd.IsDexDisabled() {
			// Delete any existing RoleBinding created for Dex
			return r.Client.Delete(context.TODO(), roleBinding)
		}

		// if the RoleRef changes, delete the existing role binding and create a new one
		if !reflect.DeepEqual(roleBinding.RoleRef, existingRoleBinding.RoleRef) {
			if err = r.Client.Delete(context.TODO(), existingRoleBinding); err != nil {
				return err
			}
		} else {
			existingRoleBinding.Subjects = roleBinding.Subjects
			return r.Client.Update(context.TODO(), existingRoleBinding)
		}
	}

	controllerutil.SetControllerReference(cr, roleBinding, r.Scheme)
	return r.Client.Create(context.TODO(), roleBinding)
}

func (r *ArgoCDReconciler) reconcileServiceAccount(name string, cr *argoprojv1a1.ArgoCD) (*corev1.ServiceAccount, error) {
	sa := argocd.NewServiceAccountWithName(name, cr)

	exists := true
	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		if !crierrors.IsNotFound(err) {
			return nil, err
		}
		if name == dexServer && argocd.IsDexDisabled() {
			return sa, nil // Dex is disabled, do nothing
		}
		exists = false
	}
	if exists {
		if name == dexServer && argocd.IsDexDisabled() {
			// Delete any existing Service Account created for Dex
			return sa, r.Client.Delete(context.TODO(), sa)
		}
		return sa, nil
	}

	if err := controllerutil.SetControllerReference(cr, sa, r.Scheme); err != nil {
		return nil, err
	}

	err := r.Client.Create(context.TODO(), sa)
	if err != nil {
		return nil, err
	}

	return sa, nil
}

// reconcileDexServiceAccount will ensure that the Dex ServiceAccount is configured properly for OpenShift OAuth.
func (r *ArgoCDReconciler) reconcileDexServiceAccount(cr *argoprojv1a1.ArgoCD) error {
	if !cr.Spec.Dex.OpenShiftOAuth {
		return nil // OpenShift OAuth not enabled, move along...
	}

	logr.Info("oauth enabled, configuring dex service account")
	sa := argocd.NewServiceAccountWithName(common.ArgoCDDefaultDexServiceAccountName, cr)
	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		return err
	}

	// Get the OAuth redirect URI that should be used.
	uri := r.getDexOAuthRedirectURI(cr)
	logr.Info(fmt.Sprintf("URI: %s", uri))

	// Get the current redirect URI
	ann := sa.ObjectMeta.Annotations
	currentURI, found := ann[common.ArgoCDKeyDexOAuthRedirectURI]
	if found && currentURI == uri {
		return nil // Redirect URI annotation found and correct, move along...
	}

	logr.Info(fmt.Sprintf("current URI: %s is not correct, should be: %s", currentURI, uri))
	if len(ann) <= 0 {
		ann = make(map[string]string)
	}

	ann[common.ArgoCDKeyDexOAuthRedirectURI] = uri
	sa.ObjectMeta.Annotations = ann

	return r.Client.Update(context.TODO(), sa)
}

// getDexOAuthRedirectURI will return the OAuth redirect URI for the Dex server.
func (r *ArgoCDReconciler) getDexOAuthRedirectURI(cr *argoprojv1a1.ArgoCD) string {
	uri := r.getArgoServerURI(cr)
	return uri + common.ArgoCDDefaultDexOAuthRedirectPath
}

// getArgoServerURI will return the URI for the ArgoCD server.
// The hostname for argocd-server is from the route, ingress, an external hostname or service name in that order.
func (r *ArgoCDReconciler) getArgoServerURI(cr *argoprojv1a1.ArgoCD) string {
	host := argocd.NameWithSuffix("server", cr) // Default to service name

	// Use the external hostname provided by the user
	if cr.Spec.Server.Host != "" {
		host = cr.Spec.Server.Host
	}

	// Use Ingress host if enabled
	if cr.Spec.Server.Ingress.Enabled {
		ing := argocd.NewIngressWithSuffix("server", cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, ing.Name, ing) {
			host = ing.Spec.Rules[0].Host
		}
	}

	// Use Route host if available, override Ingress if both exist
	if argocd.IsRouteAPIAvailable() {
		route := argocd.NewRouteWithSuffix("server", cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, route.Name, route) {
			host = route.Spec.Host
		}
	}

	return fmt.Sprintf("https://%s", host) // TODO: Safe to assume HTTPS here?
}

// reconcileRoles will ensure that all ArgoCD Service Accounts are configured.
func (r *ArgoCDReconciler) reconcileRoles(cr *argoprojv1a1.ArgoCD) (role *v1.Role, err error) {
	if role, err := r.reconcileRole(applicationController, argocd.PolicyRuleForApplicationController(), cr); err != nil {
		return role, err
	}

	if role, err := r.reconcileRole(dexServer, argocd.PolicyRuleForDexServer(), cr); err != nil {
		return role, err
	}

	if role, err := r.reconcileRole(server, argocd.PolicyRuleForServer(), cr); err != nil {
		return role, err
	}

	if role, err := r.reconcileRole(redisHa, argocd.PolicyRuleForRedisHa(cr), cr); err != nil {
		return role, err
	}

	if _, err := r.reconcileClusterRole(applicationController, argocd.PolicyRuleForApplicationController(), cr); err != nil {
		return nil, err
	}

	if _, err := r.reconcileClusterRole(server, argocd.PolicyRuleForServerClusterRole(), cr); err != nil {
		return nil, err
	}

	return nil, nil
}

func (r *ArgoCDReconciler) reconcileClusterRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) (*v1.ClusterRole, error) {
	allowed := false
	if argocd.AllowedNamespace(cr.Namespace, os.Getenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")) {
		allowed = true
	}
	clusterRole := argocd.NewClusterRole(name, policyRules, cr)
	if err := argocd.ApplyReconcilerHook(cr, clusterRole, ""); err != nil {
		return nil, err
	}

	existingClusterRole := &v1.ClusterRole{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRole.Name}, existingClusterRole)
	if err != nil {
		if !crierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the cluster role for the service account associated with %s : %s", name, err)
		}
		if !allowed {
			// Do Nothing
			return nil, nil
		}
		controllerutil.SetControllerReference(cr, clusterRole, r.Scheme)
		return clusterRole, r.Client.Create(context.TODO(), clusterRole)
	}

	if !allowed {
		return nil, r.Client.Delete(context.TODO(), existingClusterRole)
	}

	existingClusterRole.Rules = clusterRole.Rules
	return existingClusterRole, r.Client.Update(context.TODO(), existingClusterRole)
}

// reconcileRole
func (r *ArgoCDReconciler) reconcileRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) (*v1.Role, error) {
	role := argocd.NewRole(name, policyRules, cr)
	if err := argocd.ApplyReconcilerHook(cr, role, ""); err != nil {
		return nil, err
	}
	existingRole := v1.Role{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: cr.Namespace}, &existingRole)
	if err != nil {
		if !crierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the role for the service account associated with %s : %s", name, err)
		}
		if name == dexServer && argocd.IsDexDisabled() {
			return role, nil // Dex is disabled, do nothing
		}
		controllerutil.SetControllerReference(cr, role, r.Scheme)
		return role, r.Client.Create(context.TODO(), role)
	}

	if name == dexServer && argocd.IsDexDisabled() {
		// Delete any existing Role created for Dex
		return role, r.Client.Delete(context.TODO(), role)
	}
	existingRole.Rules = role.Rules
	return &existingRole, r.Client.Update(context.TODO(), &existingRole)
}

// reconcileStatus will ensure that all of the Status properties are updated for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileStatus(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileStatusApplicationController(cr); err != nil {
		return err
	}

	if err := r.reconcileStatusDex(cr); err != nil {
		return err
	}

	if err := r.reconcileStatusPhase(cr); err != nil {
		return err
	}

	if err := r.reconcileStatusRedis(cr); err != nil {
		return err
	}

	if err := r.reconcileStatusRepo(cr); err != nil {
		return err
	}

	if err := r.reconcileStatusServer(cr); err != nil {
		return err
	}
	return nil
}

// reconcileStatusServer will ensure that the Server status is updated for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileStatusServer(cr *argoprojv1a1.ArgoCD) error {
	status := "Unknown"

	deploy := argocd.NewDeploymentWithSuffix("server", "server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		status = "Pending"

		// TODO: Refactor these checks.
		if deploy.Spec.Replicas != nil {
			if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
				status = "Running"
			}
		}
	}

	if cr.Status.Server != status {
		cr.Status.Server = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}

// reconcileStatusRepo will ensure that the Repo status is updated for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileStatusRepo(cr *argoprojv1a1.ArgoCD) error {
	status := "Unknown"

	deploy := argocd.NewDeploymentWithSuffix("repo-server", "repo-server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		status = "Pending"

		if deploy.Spec.Replicas != nil {
			if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
				status = "Running"
			}
		}
	}

	if cr.Status.Repo != status {
		cr.Status.Repo = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}

// reconcileStatusRedis will ensure that the Redis status is updated for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileStatusRedis(cr *argoprojv1a1.ArgoCD) error {
	status := "Unknown"

	if !cr.Spec.HA.Enabled {
		deploy := argocd.NewDeploymentWithSuffix("redis", "redis", cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
			status = "Pending"

			if deploy.Spec.Replicas != nil {
				if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
					status = "Running"
				}
			}
		}
	} else {
		ss := argocd.NewStatefulSetWithSuffix("redis-ha-server", "redis-ha-server", cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, ss.Name, ss) {
			status = "Pending"

			if ss.Status.ReadyReplicas == *ss.Spec.Replicas {
				status = "Running"
			}
		}
		// TODO: Add check for HA proxy deployment here as well?
	}

	if cr.Status.Redis != status {
		cr.Status.Redis = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}

// reconcileStatusPhase will ensure that the Status Phase is updated for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileStatusPhase(cr *argoprojv1a1.ArgoCD) error {
	var phase string

	if cr.Status.ApplicationController == "Running" && cr.Status.Redis == "Running" && cr.Status.Repo == "Running" && cr.Status.Server == "Running" {
		phase = "Available"
	} else {
		phase = "Pending"
	}

	if cr.Status.Phase != phase {
		cr.Status.Phase = phase
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}

// reconcileStatusApplicationController will ensure that the ApplicationController Status is updated for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileStatusApplicationController(cr *argoprojv1a1.ArgoCD) error {
	status := "Unknown"

	ss := argocd.NewStatefulSetWithSuffix("application-controller", "application-controller", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, ss.Name, ss) {
		status = "Pending"

		if ss.Spec.Replicas != nil {
			if ss.Status.ReadyReplicas == *ss.Spec.Replicas {
				status = "Running"
			}
		}
	}

	if cr.Status.ApplicationController != status {
		cr.Status.ApplicationController = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}

// reconcileStatusDex will ensure that the Dex status is updated for the given ArgoCD.
func (r *ArgoCDReconciler) reconcileStatusDex(cr *argoprojv1a1.ArgoCD) error {
	status := "Unknown"

	deploy := argocd.NewDeploymentWithSuffix("dex-server", "dex-server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		status = "Pending"

		if deploy.Spec.Replicas != nil {
			if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
				status = "Running"
			}
		}
	}

	if cr.Status.Dex != status {
		cr.Status.Dex = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}
