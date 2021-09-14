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
	b64 "encoding/base64"
	json "encoding/json"
	"fmt"
	"os"

	keycloakv1alpha1 "github.com/keycloak/keycloak-operator/pkg/apis/keycloak/v1alpha1"
	appsv1 "github.com/openshift/api/apps/v1"
	oappsv1 "github.com/openshift/api/apps/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	template "github.com/openshift/api/template/v1"
	oappsv1client "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	templatev1client "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	"gopkg.in/yaml.v2"
	k8sappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
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
	// Default Keycloak Instance Admin user.
	defaultKeycloakAdminUser = "admin"
	// Default Keycloak Instance Admin password.
	defaultKeycloakAdminPassword = "admin"
	// Default Hostname for Keycloak Ingress.
	keycloakIngressHost = "keycloak-ingress"
)

var (
	privilegedMode bool  = false
	graceTime      int64 = 75
	portTLS        int32 = 8443
	httpPort       int32 = 8080
	controllerRef  bool  = true
)

// KeycloakPostData defines the values required to update Keycloak Realm.
type keycloakConfig struct {
	ArgoName           string
	ArgoNamespace      string
	Username           string
	Password           string
	KeycloakURL        string
	ArgoCDURL          string
	KeycloakServerCert []byte
	VerifyTLS          bool
}

type oidcConfig struct {
	Name           string   `json:"name"`
	Issuer         string   `json:"issuer"`
	ClientID       string   `json:"clientID"`
	ClientSecret   string   `json:"clientSecret"`
	RequestedScope []string `json:"requestedScopes"`
}

// getKeycloakContainerImage will return the container image for the Keycloak.
//
// There are three possible options for configuring the image, and this is the
// order of preference.
//
// 1. from the Spec, the spec.sso field has an image and version to use for
// generating an image reference.
// 2. From the Environment, this looks for the `ARGOCD_KEYCLOAK_IMAGE` field and uses
// that if the spec is not configured.
// 3. the default is configured in common.ArgoCDKeycloakVersion and
// common.ArgoCDKeycloakImageName.
func getKeycloakContainerImage(cr *argoprojv1a1.ArgoCD) string {
	defaultImg, defaultTag := false, false
	img := cr.Spec.SSO.Image
	if img == "" {
		img = common.ArgoCDKeycloakImage
		if IsTemplateAPIAvailable() {
			img = common.ArgoCDKeycloakImageForOpenShift
		}
		defaultImg = true
	}

	tag := cr.Spec.SSO.Version
	if tag == "" {
		tag = common.ArgoCDKeycloakVersion
		if IsTemplateAPIAvailable() {
			tag = common.ArgoCDKeycloakVersionForOpenShift
		}
		defaultTag = true
	}
	if e := os.Getenv(common.ArgoCDKeycloakImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}
	return argoutil.CombineImageTag(img, tag)
}

func getKeycloakConfigMapTemplate(ns string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"description": "ConfigMap providing service ca bundle",
				"service.beta.openshift.io/inject-cabundle": "true",
			},
			Labels: map[string]string{
				"application": "${APPLICATION_NAME}",
			},
			Name:      "${APPLICATION_NAME}-service-ca",
			Namespace: ns,
		},
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
	}
}

func getKeycloakSecretTemplate(ns string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"application": "${APPLICATION_NAME}",
			},
			Name:      "${APPLICATION_NAME}-secret",
			Namespace: ns,
		},
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		StringData: map[string]string{
			"SSO_USERNAME": "${SSO_ADMIN_USERNAME}",
			"SSO_PASSWORD": "${SSO_ADMIN_PASSWORD}",
		},
	}
}

// defaultKeycloakResources for Keycloak container.
func defaultKeycloakResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("512Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("500m"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("1024Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("1000m"),
		},
	}
}

// getKeycloakResources will return the ResourceRequirements for the Keycloak container.
func getKeycloakResources(cr *argoprojv1a1.ArgoCD) corev1.ResourceRequirements {

	// Default values for Keycloak resources requirements.
	resources := defaultKeycloakResources()

	// Allow override of resource requirements from CR
	if cr.Spec.SSO.Resources != nil {
		resources = *cr.Spec.SSO.Resources
		return resources
	}

	return resources
}

func getKeycloakContainer(cr *argoprojv1a1.ArgoCD) corev1.Container {
	return corev1.Container{
		Env: []corev1.EnvVar{
			{Name: "SSO_HOSTNAME", Value: "${SSO_HOSTNAME}"},
			{Name: "DB_MIN_POOL_SIZE", Value: "${DB_MIN_POOL_SIZE}"},
			{Name: "DB_MAX_POOL_SIZE", Value: "${DB_MAX_POOL_SIZE}"},
			{Name: "DB_TX_ISOLATION", Value: "${DB_TX_ISOLATION}"},
			{Name: "OPENSHIFT_DNS_PING_SERVICE_NAME", Value: "${APPLICATION_NAME}-ping"},
			{Name: "OPENSHIFT_DNS_PING_SERVICE_PORT", Value: "8888"},
			{Name: "X509_CA_BUNDLE", Value: "/var/run/configmaps/service-ca/service-ca.crt /var/run/secrets/kubernetes.io/serviceaccount/*.crt"},
			{Name: "SSO_ADMIN_USERNAME", Value: "${SSO_ADMIN_USERNAME}"},
			{Name: "SSO_ADMIN_PASSWORD", Value: "${SSO_ADMIN_PASSWORD}"},
			{Name: "SSO_REALM", Value: "${SSO_REALM}"},
			{Name: "SSO_SERVICE_USERNAME", Value: "${SSO_SERVICE_USERNAME}"},
			{Name: "SSO_SERVICE_PASSWORD", Value: "${SSO_SERVICE_PASSWORD}"},
		},
		Image:           getKeycloakContainerImage(cr),
		ImagePullPolicy: "Always",
		LivenessProbe: &corev1.Probe{
			FailureThreshold: 3,
			Handler: corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"/bin/bash",
						"-c",
						"/opt/eap/bin/livenessProbe.sh",
					},
				},
			},
			InitialDelaySeconds: 60,
		},
		Name: "${APPLICATION_NAME}",
		Ports: []corev1.ContainerPort{
			{ContainerPort: 8778, Name: "jolokia", Protocol: "TCP"},
			{ContainerPort: 8080, Name: "http", Protocol: "TCP"},
			{ContainerPort: 8443, Name: "https", Protocol: "TCP"},
			{ContainerPort: 8888, Name: "ping", Protocol: "TCP"},
		},
		ReadinessProbe: &corev1.Probe{
			FailureThreshold: 10,
			Handler: corev1.Handler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"/bin/bash",
						"-c",
						"/opt/eap/bin/readinessProbe.sh",
					},
				},
			},
			InitialDelaySeconds: 60,
		},
		Resources: getKeycloakResources(cr),
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/etc/x509/https",
				Name:      "sso-x509-https-volume",
				ReadOnly:  true,
			},
			{
				MountPath: "/var/run/configmaps/service-ca",
				Name:      "service-ca",
				ReadOnly:  true,
			},
		},
	}
}

func getKeycloakDeploymentConfigTemplate(cr *argoprojv1a1.ArgoCD) *appsv1.DeploymentConfig {
	ns := cr.Namespace
	keycloakContainer := getKeycloakContainer(cr)

	dc := &appsv1.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"argocd.argoproj.io/realm-created": "false",
			},
			Labels:    map[string]string{"application": "${APPLICATION_NAME}"},
			Name:      "${APPLICATION_NAME}",
			Namespace: ns,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "argoproj.io/v1alpha1",
					UID:        cr.UID,
					Name:       cr.Name,
					Controller: &controllerRef,
					Kind:       "ArgoCD",
				},
			},
		},
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "DeploymentConfig"},
		Spec: appsv1.DeploymentConfigSpec{
			Replicas: 1,
			Selector: map[string]string{"deploymentConfig": "${APPLICATION_NAME}"},
			Strategy: appsv1.DeploymentStrategy{
				Type: "Recreate",
			},
			Template: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"application":      "${APPLICATION_NAME}",
						"deploymentConfig": "${APPLICATION_NAME}",
					},
					Name: "${APPLICATION_NAME}",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						keycloakContainer,
					},
					TerminationGracePeriodSeconds: &graceTime,
					Volumes: []corev1.Volume{
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
					},
				},
			},
			Triggers: appsv1.DeploymentTriggerPolicies{
				appsv1.DeploymentTriggerPolicy{
					Type: "ConfigChange",
				},
			},
		},
	}

	if cr.Spec.NodePlacement != nil {
		dc.Spec.Template.Spec.NodeSelector = cr.Spec.NodePlacement.NodeSelector
		dc.Spec.Template.Spec.Tolerations = cr.Spec.NodePlacement.Tolerations
	}

	return dc

}

func getKeycloakServiceTemplate(ns string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    map[string]string{"application": "${APPLICATION_NAME}"},
			Name:      "${APPLICATION_NAME}",
			Namespace: ns,
			Annotations: map[string]string{
				"description": "The web server's https port",
				"service.alpha.openshift.io/serving-cert-secret-name": servingCertSecretName,
			},
		},
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: portTLS, TargetPort: intstr.FromInt(int(portTLS))},
			},
			Selector: map[string]string{
				"deploymentConfig": "${APPLICATION_NAME}",
			},
		},
	}
}

func getKeycloakRouteTemplate(ns string) *routev1.Route {
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      map[string]string{"application": "${APPLICATION_NAME}"},
			Name:        "${APPLICATION_NAME}",
			Namespace:   ns,
			Annotations: map[string]string{"description": "Route for application's https service"},
		},
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Route"},
		Spec: routev1.RouteSpec{
			TLS: &routev1.TLSConfig{
				Termination: "reencrypt",
			},
			To: routev1.RouteTargetReference{
				Name: "${APPLICATION_NAME}",
			},
		},
	}
}

func newKeycloakTemplateInstance(cr *argoprojv1a1.ArgoCD) (*template.TemplateInstance, error) {
	tpl, err := newKeycloakTemplate(cr)
	if err != nil {
		return nil, err
	}
	return &template.TemplateInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultTemplateIdentifier,
			Namespace: cr.Namespace,
		},
		Spec: template.TemplateInstanceSpec{
			Template: tpl,
		},
	}, nil
}

func newKeycloakTemplate(cr *argoprojv1a1.ArgoCD) (template.Template, error) {
	ns := cr.Namespace
	tmpl := template.Template{}
	configMapTemplate := getKeycloakConfigMapTemplate(ns)
	secretTemplate := getKeycloakSecretTemplate(ns)
	deploymentConfigTemplate := getKeycloakDeploymentConfigTemplate(cr)
	serviceTemplate := getKeycloakServiceTemplate(ns)
	routeTemplate := getKeycloakRouteTemplate(ns)

	configMap, err := json.Marshal(configMapTemplate)
	if err != nil {
		return tmpl, err
	}

	secret, err := json.Marshal(secretTemplate)
	if err != nil {
		return tmpl, err
	}

	deploymentConfig, err := json.Marshal(deploymentConfigTemplate)
	if err != nil {
		return tmpl, err
	}

	service, err := json.Marshal(serviceTemplate)
	if err != nil {
		return tmpl, err
	}

	route, err := json.Marshal(routeTemplate)
	if err != nil {
		return tmpl, err
	}

	tmpl = template.Template{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"description":               "RH-SSO Template for Installing keycloak",
				"iconClass":                 "icon-sso",
				"openshift.io/display-name": "Keycloak",
				"tags":                      "keycloak",
				"version":                   "9.0.4-SNAPSHOT",
			},
			Name:      defaultTemplateIdentifier,
			Namespace: ns,
		},
		Objects: []runtime.RawExtension{
			{
				Raw: json.RawMessage(configMap),
			},
			{
				Raw: json.RawMessage(secret),
			},
			{
				Raw: json.RawMessage(deploymentConfig),
			},
			{
				Raw: json.RawMessage(service),
			},
			{
				Raw: json.RawMessage(route),
			},
		},
		Parameters: []template.Parameter{
			{Name: "APPLICATION_NAME", Value: "keycloak", Required: true},
			{Name: "SSO_HOSTNAME"},
			{Name: "DB_MIN_POOL_SIZE"},
			{Name: "DB_MAX_POOL_SIZE"},
			{Name: "DB_TX_ISOLATION"},
			{Name: "IMAGE_STREAM_NAMESPACE", Value: "openshift", Required: true},
			{Name: "SSO_ADMIN_USERNAME", Generate: "expression", From: "[a-zA-Z0-9]{8}", Required: true},
			{Name: "SSO_ADMIN_PASSWORD", Generate: "expression", From: "[a-zA-Z0-9]{8}", Required: true},
			{Name: "SSO_REALM", DisplayName: "RH-SSO Realm"},
			{Name: "SSO_SERVICE_USERNAME", DisplayName: "RH-SSO Service Username"},
			{Name: "SSO_SERVICE_PASSWORD", DisplayName: "RH-SSO Service Password"},
			{Name: "MEMORY_LIMIT", Value: "1Gi"},
		},
	}
	return tmpl, err
}

func newKeycloakIngress(cr *argoprojv1a1.ArgoCD) *extv1beta1.Ingress {

	return &extv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultKeycloakIdentifier,
			Namespace: cr.Namespace,
		},
		Spec: extv1beta1.IngressSpec{
			TLS: []extv1beta1.IngressTLS{
				{
					Hosts: []string{keycloakIngressHost},
				},
			},
			Rules: []extv1beta1.IngressRule{
				{
					Host: keycloakIngressHost,
					IngressRuleValue: extv1beta1.IngressRuleValue{
						HTTP: &extv1beta1.HTTPIngressRuleValue{
							Paths: []extv1beta1.HTTPIngressPath{
								{
									Backend: extv1beta1.IngressBackend{
										ServiceName: defaultKeycloakIdentifier,
										ServicePort: intstr.FromInt(int(httpPort)),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func newKeycloakService(cr *argoprojv1a1.ArgoCD) *corev1.Service {

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultKeycloakIdentifier,
			Namespace: cr.Namespace,
			Labels: map[string]string{
				"app": defaultKeycloakIdentifier,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: "http", Port: httpPort, TargetPort: intstr.FromInt(int(httpPort))},
			},
			Selector: map[string]string{
				"app": defaultKeycloakIdentifier,
			},
			Type: "LoadBalancer",
		},
	}
}

func getKeycloakContainerEnv() []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "KEYCLOAK_USER", Value: defaultKeycloakAdminUser},
		{Name: "KEYCLOAK_PASSWORD", Value: defaultKeycloakAdminPassword},
		{Name: "PROXY_ADDRESS_FORWARDING", Value: "true"},
	}
}

func newKeycloakDeployment(cr *argoprojv1a1.ArgoCD) *k8sappsv1.Deployment {

	var replicas int32 = 1
	return &k8sappsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultKeycloakIdentifier,
			Namespace: cr.Namespace,
			Annotations: map[string]string{
				"argocd.argoproj.io/realm-created": "false",
			},
			Labels: map[string]string{
				"app": defaultKeycloakIdentifier,
			},
		},
		Spec: k8sappsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": defaultKeycloakIdentifier,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": defaultKeycloakIdentifier,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  defaultKeycloakIdentifier,
							Image: getKeycloakContainerImage(cr),
							Env:   proxyEnvVars(getKeycloakContainerEnv()...),
							Ports: []corev1.ContainerPort{
								{Name: "http", ContainerPort: httpPort},
								{Name: "https", ContainerPort: portTLS},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/auth/realms/master",
										Port: intstr.FromInt(int(httpPort)),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *ReconcileArgoCD) newKeycloakInstance(cr *argoprojv1a1.ArgoCD) error {

	// Create Keycloak Ingress
	ing := newKeycloakIngress(cr)
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: ing.Name,
		Namespace: ing.Namespace}, ing)

	if err != nil {
		if errors.IsNotFound(err) {
			if err := controllerutil.SetControllerReference(cr, ing, r.Scheme); err != nil {
				return err
			}
			err = r.Client.Create(context.TODO(), ing)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// Create Keycloak Service
	svc := newKeycloakService(cr)
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: svc.Name,
		Namespace: svc.Namespace}, svc)

	if err != nil {
		if errors.IsNotFound(err) {
			if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
				return err
			}
			err = r.Client.Create(context.TODO(), svc)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// Create Keycloak Deployment
	dep := newKeycloakDeployment(cr)
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: dep.Name,
		Namespace: dep.Namespace}, dep)

	if err != nil {
		if errors.IsNotFound(err) {
			if err := controllerutil.SetControllerReference(cr, dep, r.Scheme); err != nil {
				return err
			}
			err = r.Client.Create(context.TODO(), dep)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

// prepares a keycloak config which is used in creating keycloak realm configuration.
func (r *ReconcileArgoCD) prepareKeycloakConfig(cr *argoprojv1a1.ArgoCD) (*keycloakConfig, error) {

	var tlsVerification bool
	// Get keycloak hostname from route.
	// keycloak hostname is required to post realm configuration to keycloak when keycloak cannot be accessed using service name
	// due to network policies or operator running outside the cluster or development purpose.
	existingKeycloakRoute := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultKeycloakIdentifier,
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
			Name:      fmt.Sprintf("%s-%s", defaultKeycloakIdentifier, "secret"),
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

	cfg := &keycloakConfig{
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

// prepares a keycloak config which is used in creating keycloak realm configuration for kubernetes.
func (r *ReconcileArgoCD) prepareKeycloakConfigForK8s(cr *argoprojv1a1.ArgoCD) (*keycloakConfig, error) {

	// Get keycloak hostname from ingress.
	// keycloak hostname is required to post realm configuration to keycloak when keycloak cannot be accessed using service name
	// due to network policies or operator running outside the cluster or development purpose.
	existingKeycloakIng := &extv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultKeycloakIdentifier,
			Namespace: cr.Namespace,
		},
	}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: existingKeycloakIng.Name,
		Namespace: existingKeycloakIng.Namespace}, existingKeycloakIng)
	if err != nil {
		return nil, err
	}
	kIngURL := fmt.Sprintf("https://%s", existingKeycloakIng.Spec.Rules[0].Host)

	// Get ArgoCD hostname from Ingress. ArgoCD hostname is used in the keycloak client configuration.
	existingArgoCDIng := &extv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, "server"),
			Namespace: cr.Namespace,
		},
	}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: existingArgoCDIng.Name,
		Namespace: existingArgoCDIng.Namespace}, existingArgoCDIng)
	if err != nil {
		return nil, err
	}
	aIngURL := fmt.Sprintf("https://%s", existingArgoCDIng.Spec.Rules[0].Host)

	cfg := &keycloakConfig{
		ArgoName:      cr.Name,
		ArgoNamespace: cr.Namespace,
		Username:      defaultKeycloakAdminUser,
		Password:      defaultKeycloakAdminPassword,
		KeycloakURL:   kIngURL,
		ArgoCDURL:     aIngURL,
		VerifyTLS:     false,
	}

	return cfg, nil
}

// creates a keycloak realm configuration which when posted to keycloak using http client creates a keycloak realm.
func createRealmConfig(cfg *keycloakConfig) ([]byte, error) {

	ks := &keycloakv1alpha1.KeycloakAPIRealm{
		Realm:       keycloakRealm,
		Enabled:     true,
		SslRequired: "external",
		Clients: []*keycloakv1alpha1.KeycloakAPIClient{
			{
				ClientID:                keycloakClient,
				Name:                    keycloakClient,
				RootURL:                 cfg.ArgoCDURL,
				AdminURL:                cfg.ArgoCDURL,
				ClientAuthenticatorType: "client-secret",
				Secret:                  argocdClientSecret,
				RedirectUris: []string{fmt.Sprintf("%s/%s",
					cfg.ArgoCDURL, "auth/callback")},
				WebOrigins: []string{cfg.ArgoCDURL},
				DefaultClientScopes: []string{
					"web-origins",
					"role_list",
					"roles",
					"profile",
					"groups",
					"email",
				},
				StandardFlowEnabled: true,
			},
		},
		ClientScopes: []keycloakv1alpha1.KeycloakClientScope{
			{
				Name:     "groups",
				Protocol: "openid-connect",
				ProtocolMappers: []keycloakv1alpha1.KeycloakProtocolMapper{
					{
						Name:           "groups",
						Protocol:       "openid-connect",
						ProtocolMapper: "oidc-group-membership-mapper",
						Config: map[string]string{
							"full.path":            "false",
							"id.token.claim":       "true",
							"access.token.claim":   "true",
							"claim.name":           "groups",
							"userinfo.token.claim": "true",
						},
					},
				},
			},
			{
				Name:     "email",
				Protocol: "openid-connect",
				ProtocolMappers: []keycloakv1alpha1.KeycloakProtocolMapper{
					{
						Name:           "email",
						Protocol:       "openid-connect",
						ProtocolMapper: "oidc-usermodel-property-mapper",
						Config: map[string]string{
							"userinfo.token.claim": "true",
							"user.attribute":       "email",
							"id.token.claim":       "true",
							"access.token.claim":   "true",
							"claim.name":           "email",
							"jsonType.label":       "String",
						},
					},
				},
			},
		},
	}

	// Add OpenShift-v4 as Identity Provider only for OpenShift environment.
	// No Identity Provider is configured by default for non-openshift environments.
	if IsTemplateAPIAvailable() {
		ks.IdentityProviders = []*keycloakv1alpha1.KeycloakIdentityProvider{
			{
				Alias:       "openshift-v4",
				DisplayName: "Login with OpenShift",
				ProviderID:  "openshift-v4",
				Config: map[string]string{
					"baseUrl":      "https://kubernetes.default.svc.cluster.local",
					"clientSecret": oAuthClientSecret,
					"clientId":     getOAuthClient(cfg.ArgoNamespace),
					"defaultScope": "user:full",
				},
			},
		}
	}

	json, err := json.Marshal(ks)
	if err != nil {
		return nil, err
	}

	return json, nil
}

// Gets Keycloak Server cert. This cert is used to authenticate the api calls to the Keycloak service.
func (r *ReconcileArgoCD) getKCServerCert(cr *argoprojv1a1.ArgoCD) ([]byte, error) {

	sslCertsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      servingCertSecretName,
			Namespace: cr.Namespace,
		},
	}

	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: sslCertsSecret.Name, Namespace: sslCertsSecret.Namespace}, sslCertsSecret)

	switch {
	case err == nil:
		return sslCertsSecret.Data["tls.crt"], nil
	case errors.IsNotFound(err):
		return nil, nil
	default:
		return nil, err
	}
}

func getOAuthClient(ns string) string {
	return fmt.Sprintf("%s-%s", defaultKeycloakBrokerName, ns)
}

// Updates OIDC configuration for ArgoCD.
func (r *ReconcileArgoCD) updateArgoCDConfiguration(cr *argoprojv1a1.ArgoCD, kRouteURL string) error {

	// Update the ArgoCD client secret for OIDC in argocd-secret.
	argoCDSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDSecretName,
			Namespace: cr.Namespace,
		},
	}

	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: argoCDSecret.Name, Namespace: argoCDSecret.Namespace}, argoCDSecret)
	if err != nil {
		log.Error(err, fmt.Sprintf("ArgoCD secret not found for ArgoCD %s in namespace %s",
			cr.Name, cr.Namespace))
		return err
	}

	argoCDSecret.Data["oidc.keycloak.clientSecret"] = []byte(argocdClientSecret)
	err = r.Client.Update(context.TODO(), argoCDSecret)
	if err != nil {
		log.Error(err, fmt.Sprintf("Error updating ArgoCD Secret for ArgoCD %s in namespace %s",
			cr.Name, cr.Namespace))
		return err
	}

	// Create openshift OAuthClient
	if IsTemplateAPIAvailable() {
		oAuthClient := &oauthv1.OAuthClient{
			TypeMeta: metav1.TypeMeta{
				Kind:       "OAuthClient",
				APIVersion: "oauth.openshift.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      getOAuthClient(cr.Namespace),
				Namespace: cr.Namespace,
			},
			Secret: oAuthClientSecret,
			RedirectURIs: []string{fmt.Sprintf("%s/auth/realms/%s/broker/openshift-v4/endpoint",
				kRouteURL, keycloakClient)},
			GrantMethod: "prompt",
		}

		err = controllerutil.SetOwnerReference(cr, oAuthClient, r.Scheme)
		if err != nil {
			return err
		}

		err = r.Client.Get(context.TODO(), types.NamespacedName{Name: oAuthClient.Name}, oAuthClient)
		if err != nil {
			if errors.IsNotFound(err) {
				err = r.Client.Create(context.TODO(), oAuthClient)
				if err != nil {
					return err
				}
			}
		}
	}

	// Update ArgoCD instance for OIDC Config with Keycloakrealm URL
	o, err := yaml.Marshal(oidcConfig{
		Name: "Keycloak",
		Issuer: fmt.Sprintf("%s/auth/realms/%s",
			kRouteURL, keycloakRealm),
		ClientID:       keycloakClient,
		ClientSecret:   "$oidc.keycloak.clientSecret",
		RequestedScope: []string{"openid", "profile", "email", "groups"},
	})

	if err != nil {
		return err
	}

	argoCDCM := newConfigMapWithName(common.ArgoCDConfigMapName, cr)
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: argoCDCM.Name, Namespace: argoCDCM.Namespace}, argoCDCM)
	if err != nil {
		log.Error(err, fmt.Sprintf("ArgoCD configmap not found for ArgoCD %s in namespace %s",
			cr.Name, cr.Namespace))

		return err
	}

	argoCDCM.Data[common.ArgoCDKeyOIDCConfig] = string(o)
	err = r.Client.Update(context.TODO(), argoCDCM)
	if err != nil {
		log.Error(err, fmt.Sprintf("Error updating OIDC Configuration for ArgoCD %s in namespace %s",
			cr.Name, cr.Namespace))
		return err
	}

	// Update RBAC for ArgoCD Instance.
	argoRBACCM := newConfigMapWithName(common.ArgoCDRBACConfigMapName, cr)
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: argoRBACCM.Name, Namespace: argoRBACCM.Namespace}, argoRBACCM)
	if err != nil {
		log.Error(err, fmt.Sprintf("ArgoCD RBAC configmap not found for ArgoCD %s in namespace %s",
			cr.Name, cr.Namespace))

		return err
	}

	argoRBACCM.Data["scopes"] = "[groups,email]"
	err = r.Client.Update(context.TODO(), argoRBACCM)
	if err != nil {
		log.Error(err, fmt.Sprintf("Error updating ArgoCD RBAC configmap %s in namespace %s",
			cr.Name, cr.Namespace))
		return err
	}

	return nil
}

// HandleKeycloakPodDeletion resets the Realm Creation Status to false when keycloak pod is deleted.
func handleKeycloakPodDeletion(dc *oappsv1.DeploymentConfig) error {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to get k8s config")
		return err
	}

	// Initialize deployment config client.
	dcClient, err := oappsv1client.NewForConfig(cfg)
	if err != nil {
		log.Error(err, fmt.Sprintf("unable to create apps client for Deployment config %s in namespace %s",
			dc.Name, dc.Namespace))
		return err
	}

	log.Info("Set the Realm Creation status annoation to false")
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

// Delete Keycloak configuration for OpenShift
func deleteKeycloakConfigForOpenShift(cr *argoprojv1a1.ArgoCD) error {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, fmt.Sprintf("unable to get k8s config for ArgoCD %s in namespace %s",
			cr.Name, cr.Namespace))
		return err
	}

	// Initialize template client.
	templateclient, err := templatev1client.NewForConfig(cfg)
	if err != nil {
		log.Error(err, fmt.Sprintf("unable to create Template client for ArgoCD %s in namespace %s",
			cr.Name, cr.Namespace))
		return err
	}

	log.Info(fmt.Sprintf("Delete Template Instance for ArgoCD %s in namespace %s",
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
		log.Error(err, fmt.Sprintf("unable to create oAuth client for ArgoCD %s in namespace %s",
			cr.Name, cr.Namespace))
		return err
	}
	log.Info(fmt.Sprintf("Delete OAuthClient for ArgoCD %s in namespace %s",
		cr.Name, cr.Namespace))

	oa := getOAuthClient(cr.Namespace)
	err = oauth.OAuthClients().Delete(context.TODO(), oa, deleteOptions)
	if err != nil {
		return err
	}

	return nil
}

// Delete Keycloak configuration for Kubernetes
func deleteKeycloakConfigForK8s(cr *argoprojv1a1.ArgoCD) error {

	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, fmt.Sprintf("unable to get k8s config for ArgoCD %s in namespace %s",
			cr.Name, cr.Namespace))
		return err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Delete Keycloak deployment for ArgoCD %s in namespace %s",
		cr.Name, cr.Namespace))

	// We use the foreground propagation policy to ensure that the garbage
	// collector removes all instantiated objects before the TemplateInstance
	// itself disappears.
	foreground := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{PropagationPolicy: &foreground}
	err = clientset.AppsV1().Deployments(cr.Namespace).Delete(context.TODO(), defaultKeycloakIdentifier, deleteOptions)
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Delete Keycloak Service for ArgoCD %s in namespace %s",
		cr.Name, cr.Namespace))

	err = clientset.CoreV1().Services(cr.Namespace).Delete(context.TODO(), defaultKeycloakIdentifier, deleteOptions)
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Delete Keycloak Ingress for ArgoCD %s in namespace %s",
		cr.Name, cr.Namespace))

	err = clientset.ExtensionsV1beta1().Ingresses(cr.Namespace).Delete(context.TODO(), defaultKeycloakIdentifier, deleteOptions)
	if err != nil {
		return err
	}

	return nil
}

// Installs and configures Keycloak for OpenShift
func (r *ReconcileArgoCD) reconcileKeycloakForOpenShift(cr *argoprojv1a1.ArgoCD) error {

	templateInstanceRef, err := newKeycloakTemplateInstance(cr)
	if err != nil {
		return err
	}
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
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: existingDC.Name, Namespace: existingDC.Namespace}, existingDC)
	if err != nil {
		log.Error(err, fmt.Sprintf("Keycloak Deployment not found or being created for ArgoCD %s in namespace %s",
			cr.Name, cr.Namespace))
	}

	// If Keycloak deployment exists and a realm is already created for ArgoCD, Do not create a new one.
	if existingDC.Status.AvailableReplicas == expectedReplicas &&
		existingDC.Annotations["argocd.argoproj.io/realm-created"] == "false" {

		cfg, err := r.prepareKeycloakConfig(cr)
		if err != nil {
			return err
		}

		// keycloakRouteURL is used to update the OIDC configuration for ArgoCD.
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
			err = r.Client.Update(context.TODO(), existingDC)
			if err != nil {
				return err
			}

			err = r.updateArgoCDConfiguration(cr, keycloakRouteURL)
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed to update OIDC Configuration for ArgoCD %s in namespace %s",
					cr.Name, cr.Namespace))
				return err
			}
		}
	}

	return nil
}

// Installs and configures Keycloak for Kubernetes
func (r *ReconcileArgoCD) reconcileKeycloak(cr *argoprojv1a1.ArgoCD) error {

	err := r.newKeycloakInstance(cr)
	if err != nil {
		log.Error(err, fmt.Sprintf("Failed creating keycloak instance for ArgoCD %s in Namespace %s",
			cr.Name, cr.Namespace))
		return err
	}

	existingDeployment := &k8sappsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultKeycloakIdentifier,
			Namespace: cr.Namespace,
		},
	}

	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: existingDeployment.Name, Namespace: existingDeployment.Namespace}, existingDeployment)
	if err != nil {
		log.Error(err, fmt.Sprintf("Keycloak Deployment not found or being created for ArgoCD %s in namespace %s",
			cr.Name, cr.Namespace))
	}

	// If Keycloak deployment exists and a realm is already created for ArgoCD, Do not create a new one.
	if existingDeployment.Status.AvailableReplicas == expectedReplicas &&
		existingDeployment.Annotations["argocd.argoproj.io/realm-created"] == "false" {

		cfg, err := r.prepareKeycloakConfigForK8s(cr)
		if err != nil {
			return err
		}

		// kIngURL is used to update the OIDC configuration for ArgoCD.
		kIngURL := cfg.KeycloakURL

		// Create a keycloak realm and publish.
		response, err := createRealm(cfg)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed posting keycloak realm configuration for ArgoCD %s in namespace %s",
				cr.Name, cr.Namespace))
			return err
		}

		if response == successResponse {
			log.Info("Successfully created keycloak realm for ArgoCD %s in namespace %s")

			// Update Realm creation. This will avoid posting of realm configuration on further reconciliations.
			existingDeployment.Annotations["argocd.argoproj.io/realm-created"] = "true"
			err = r.Client.Update(context.TODO(), existingDeployment)
			if err != nil {
				return err
			}
		}

		err = r.updateArgoCDConfiguration(cr, kIngURL)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to update OIDC Configuration for ArgoCD %s in namespace %s",
				cr.Name, cr.Namespace))
			return err
		}
	}

	return nil
}
