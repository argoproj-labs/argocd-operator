package argocd

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// DexConnector represents an authentication connector for Dex.
type DexConnector struct {
	Config map[string]interface{} `yaml:"config,omitempty"`
	ID     string                 `yaml:"id"`
	Name   string                 `yaml:"name"`
	Type   string                 `yaml:"type"`
}

// getDexOAuthClientSecret will return the OAuth client secret for the given ArgoCD.
func (r *ReconcileArgoCD) getDexOAuthClientSecret(cr *argoprojv1a1.ArgoCD) (*string, error) {
	sa := newServiceAccountWithName(common.ArgoCDDefaultDexServiceAccountName, cr)
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
	secret := argoutil.NewSecretWithName(cr, tokenSecret.Name)
	if err := argoutil.FetchObject(r.Client, cr.Namespace, secret.Name, secret); err != nil {
		return nil, err
	}

	token := string(secret.Data["token"])
	return &token, nil
}

// reconcileDexConfiguration will ensure that Dex is configured properly.
func (r *ReconcileArgoCD) reconcileDexConfiguration(cm *corev1.ConfigMap, cr *argoprojv1a1.ArgoCD) error {
	actual := cm.Data[common.ArgoCDKeyDexConfig]
	desired := getDexConfig(cr)

	// If no dexConfig expressed but openShiftOAuth is requested through either `.spec.dex` or `.spec.sso.dex`, use default
	// openshift dex config
	if len(desired) <= 0 && (!reflect.DeepEqual(cr.Spec.Dex, &v1alpha1.ArgoCDDexSpec{}) && cr.Spec.Dex.OpenShiftOAuth ||
		cr.Spec.SSO != nil && cr.Spec.SSO.Dex != nil && cr.Spec.SSO.Dex.OpenShiftOAuth) {
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
		deploy := newDeploymentWithSuffix("dex-server", "dex-server", cr)
		if !argoutil.IsObjectFound(r.Client, deploy.Namespace, deploy.Name, deploy) {
			log.Info("unable to locate dex deployment")
			return nil
		}

		deploy.Spec.Template.ObjectMeta.Labels["dex.config.changed"] = time.Now().UTC().Format("01022006-150406-MST")
		return r.Client.Update(context.TODO(), deploy)
	}
	return nil
}

// getOpenShiftDexConfig will return the configuration for the Dex server running on OpenShift.
func (r *ReconcileArgoCD) getOpenShiftDexConfig(cr *argoprojv1a1.ArgoCD) (string, error) {
	clientSecret, err := r.getDexOAuthClientSecret(cr)
	if err != nil {
		return "", err
	}

	groups := []string{}

	// Allow override of groups from CR
	if !reflect.DeepEqual(cr.Spec.Dex, v1alpha1.ArgoCDDexSpec{}) && cr.Spec.Dex.Groups != nil {
		groups = cr.Spec.Dex.Groups
	} else if cr.Spec.SSO != nil && cr.Spec.SSO.Dex != nil && cr.Spec.SSO.Dex.Groups != nil {
		groups = cr.Spec.SSO.Dex.Groups
	}

	connector := DexConnector{
		Type: "openshift",
		ID:   "openshift",
		Name: "OpenShift",
		Config: map[string]interface{}{
			"issuer":       "https://kubernetes.default.svc", // TODO: Should this be hard-coded?
			"clientID":     getDexOAuthClientID(cr),
			"clientSecret": *clientSecret,
			"redirectURI":  r.getDexOAuthRedirectURI(cr),
			"insecureCA":   true, // TODO: Configure for openshift CA,
			"groups":       groups,
		},
	}

	connectors := make([]DexConnector, 0)
	connectors = append(connectors, connector)

	dex := make(map[string]interface{})
	dex["connectors"] = connectors

	bytes, err := yaml.Marshal(dex)
	return string(bytes), err
}

// reconcileDexServiceAccount will ensure that the Dex ServiceAccount is configured properly for OpenShift OAuth.
func (r *ReconcileArgoCD) reconcileDexServiceAccount(cr *argoprojv1a1.ArgoCD) error {
	if reflect.DeepEqual(cr.Spec.Dex, &v1alpha1.ArgoCDDexSpec{}) || !cr.Spec.Dex.OpenShiftOAuth ||
		(cr.Spec.SSO == nil || reflect.DeepEqual(cr.Spec.SSO.Dex, &v1alpha1.ArgoCDDexSpec{}) || !cr.Spec.SSO.Dex.OpenShiftOAuth) {
		return nil // OpenShift OAuth not enabled, move along...
	}

	log.Info("oauth enabled, configuring dex service account")
	sa := newServiceAccountWithName(common.ArgoCDDefaultDexServiceAccountName, cr)
	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		return err
	}

	// Get the OAuth redirect URI that should be used.
	uri := r.getDexOAuthRedirectURI(cr)
	log.Info(fmt.Sprintf("URI: %s", uri))

	// Get the current redirect URI
	ann := sa.ObjectMeta.Annotations
	currentURI, found := ann[common.ArgoCDKeyDexOAuthRedirectURI]
	if found && currentURI == uri {
		return nil // Redirect URI annotation found and correct, move along...
	}

	log.Info(fmt.Sprintf("current URI: %s is not correct, should be: %s", currentURI, uri))
	if len(ann) <= 0 {
		ann = make(map[string]string)
	}

	ann[common.ArgoCDKeyDexOAuthRedirectURI] = uri
	sa.ObjectMeta.Annotations = ann

	return r.Client.Update(context.TODO(), sa)
}

// reconcileDexDeployment will ensure the Deployment resource is present for the ArgoCD Dex component.
func (r *ReconcileArgoCD) reconcileDexDeployment(cr *argoprojv1a1.ArgoCD) error {
	deploy := newDeploymentWithSuffix("dex-server", "dex-server", cr)

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command: []string{
			"/shared/argocd-dex",
			"rundex",
		},
		Image:           getDexContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "dex",
		Env:             proxyEnvVars(),
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz/live",
					Port: intstr.FromInt(common.ArgoCDDefaultDexMetricsPort),
				},
			},
			InitialDelaySeconds: 60,
			PeriodSeconds:       30,
		},
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultDexHTTPPort,
				Name:          "http",
			}, {
				ContainerPort: common.ArgoCDDefaultDexGRPCPort,
				Name:          "grpc",
			}, {
				ContainerPort: common.ArgoCDDefaultDexMetricsPort,
				Name:          "metrics",
			},
		},
		Resources: getDexResources(cr),
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
		Env:             proxyEnvVars(),
		Image:           getArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "copyutil",
		Resources:       getDexResources(cr),
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

	existing := newDeploymentWithSuffix("dex-server", "dex-server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {

		// make sure old workloads using DISABLE_DEX=true don't slip through here because their .spec.sso is nil
		if isDexDisabled() && isDisableDexSet && (cr.Spec.SSO == nil || cr.Spec.SSO.Provider != v1alpha1.SSOProviderTypeDex) ||
			// make sure new workloads that don't set the env var isDisbaleDexSet also have their .spec.sso == nil in order to return from here
			(!isDisableDexSet && (cr.Spec.SSO == nil || cr.Spec.SSO.Provider != v1alpha1.SSOProviderTypeDex)) {

			// Don't delete dex deployment if dex configuration is present in argocd-cm. This is done to prevent a breaking change to users
			// who may be using dex without setting DISABLE_DEX in their env
			if (!reflect.DeepEqual(cr.Spec.Dex, v1alpha1.ArgoCDDexSpec{}) && (cr.Spec.Dex.OpenShiftOAuth || cr.Spec.Dex.Config != "")) {
				log.Info("Could not delete dex deployment due to existing dex configuration in argocd-cm configmap. Remove dexConfig to allow deletion of dex deployment")
				return nil
			}

			log.Info("deleting the existing dex deployment because dex is disabled")
			// Deployment exists but enabled flag has been set to false, delete the Deployment
			return r.Client.Delete(context.TODO(), existing)
		}
		changed := false

		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getDexContainerImage(cr)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}

		actualImage = existing.Spec.Template.Spec.InitContainers[0].Image
		desiredImage = getArgoContainerImage(cr)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.InitContainers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}
		updateNodePlacement(existing, deploy, &changed)
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

		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Resources, existing.Spec.Template.Spec.Containers[0].Resources) {
			existing.Spec.Template.Spec.Containers[0].Resources = deploy.Spec.Template.Spec.Containers[0].Resources
			changed = true
		}

		if changed {
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	// if (isDisableDexSet && isDexDisabled()) || (cr.Spec.SSO == nil) || (cr.Spec.SSO.Provider != v1alpha1.SSOProviderTypeDex) {
	// 	return nil
	// }

	// make sure old workloads using DISABLE_DEX=false don't slip through here because their .spec.sso is nil
	if isDexDisabled() && isDisableDexSet && (cr.Spec.SSO == nil || cr.Spec.SSO.Provider != v1alpha1.SSOProviderTypeDex) ||
		// make sure new workloads that don't set the env var isDisbaleDexSet also have their .spec.sso == nil in order to return from here
		(!isDisableDexSet && (cr.Spec.SSO == nil || cr.Spec.SSO.Provider != v1alpha1.SSOProviderTypeDex)) {

		return nil
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileDexService will ensure that the Service for Dex is present.
func (r *ReconcileArgoCD) reconcileDexService(cr *argoprojv1a1.ArgoCD) error {
	svc := newServiceWithSuffix("dex-server", "dex-server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {

		// make sure old workloads using DISABLE_DEX don't slip through here because their .spec.sso is nil
		if isDexDisabled() && isDisableDexSet && (cr.Spec.SSO == nil || cr.Spec.SSO.Provider != v1alpha1.SSOProviderTypeDex) ||
			// make sure new workloads that don't set the env var DISABLE_DEX also have their .spec.sso == nil in order to return from here
			(!isDisableDexSet && (cr.Spec.SSO == nil || cr.Spec.SSO.Provider != v1alpha1.SSOProviderTypeDex)) {

			// Don't delete dex service if dex configuration is present in argocd-cm. This is done to prevent a breaking change to users
			// who may be using dex without setting DISABLE_DEX in their env
			if (!reflect.DeepEqual(cr.Spec.Dex, v1alpha1.ArgoCDDexSpec{}) && (cr.Spec.Dex.OpenShiftOAuth || cr.Spec.Dex.Config != "")) {
				log.Info("Could not delete dex service due to existing dex configuration in argocd-cm configmap. Remove dexConfig to allow deletion of dex service")
				return nil
			}

			// Service exists but enabled flag has been set to false, delete the Service
			log.Info("deleting the existing Dex service because dex is not configured")
			return r.Client.Delete(context.TODO(), svc)
		}
		return nil
	}

	// make sure old workloads using DISABLE_DEX don't slip through here because their .spec.sso is nil
	if isDexDisabled() && isDisableDexSet && (cr.Spec.SSO == nil || cr.Spec.SSO.Provider != v1alpha1.SSOProviderTypeDex) ||
		// make sure new workloads that don't set the env var DISABLE_DEX also have their .spec.sso == nil in order to return from here
		(!isDisableDexSet && (cr.Spec.SSO == nil || cr.Spec.SSO.Provider != v1alpha1.SSOProviderTypeDex)) {

		return nil // Dex is disabled, do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("dex-server", cr),
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

// reconcileDexResources consolidates all dex resources reconciliation calls. It serves as the single place to trigger both creation
// and deletion of dex resources based on the specified configuration of dex
func (r *ReconcileArgoCD) reconcileDexResources(cr *argoprojv1a1.ArgoCD) error {

	if _, err := r.reconcileRole(common.ArgoCDDexServerComponent, policyRuleForDexServer(), cr); err != nil {
		return err
	}

	if err := r.reconcileRoleBinding(common.ArgoCDDexServerComponent, policyRuleForDexServer(), cr); err != nil {
		return fmt.Errorf("error reconciling roleBinding for %q: %w", common.ArgoCDDexServerComponent, err)
	}

	if err := r.reconcileServiceAccountPermissions(common.ArgoCDDexServerComponent, policyRuleForDexServer(), cr); err != nil {
		return err
	}

	// specialized handling for dex
	if err := r.reconcileDexServiceAccount(cr); err != nil {
		return err
	}

	// Reconcile dex config in argocd-cm, create dex config in argocd-cm if required (right after dex is enabled)
	if err := r.reconcileArgoConfigMap(cr); err != nil {
		return err
	}

	// Reconcile dex config in argocd-cm (right after dex is disabled)-- Might seem like a duplicate call to the above line,
	// but this is required for a one time trigger of reconcileDexConfiguration directly in case of a dex deletion event,
	// since reconcileArgoConfigMap won't call reconcileDexConfiguration once dex has been disabled (to avoid reconciling on
	// dexconfig unnecessarily when it isn't enabled)
	cm := newConfigMapWithName(common.ArgoCDConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if err := r.reconcileDexConfiguration(cm, cr); err != nil {
			return err
		}
	}

	if err := r.reconcileDexService(cr); err != nil {
		return err
	}

	if err := r.reconcileDexDeployment(cr); err != nil {
		return err
	}

	if err := r.reconcileStatusDex(cr); err != nil {
		return err
	}

	return nil
}
