package argocd

import (
	"context"
	e "errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
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

// UseDex determines whether Dex resources should be created and configured or not
func UseDex(cr *argoproj.ArgoCD) bool {
	if cr.Spec.SSO != nil {
		return cr.Spec.SSO.Provider.ToLower() == argoproj.SSOProviderTypeDex
	}

	return false
}

// getDexOAuthClientSecret will return the OAuth client secret for the given ArgoCD.
func (r *ReconcileArgoCD) getDexOAuthClientSecret(cr *argoproj.ArgoCD) (*string, error) {
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
		// This change of creating secret for dex service account,is due to change of reduction of secret-based service account tokens in k8s v1.24 so from k8s v1.24 no default secret for service account is created, but for dex to work we need to provide token of secret used by dex service account as a oauth token, this change helps to achieve it, in long run we should see do dex really requires a secret or it manages to create one using TokenRequest API or may be change how dex is used or configured by operator
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "argocd-dex-server-token-",
				Namespace:    cr.Namespace,
				Annotations: map[string]string{
					corev1.ServiceAccountNameKey: sa.Name,
				},
			},
			Type: corev1.SecretTypeServiceAccountToken,
		}
		err := r.Client.Create(context.TODO(), secret)
		if err != nil {
			return nil, e.New("unable to locate and create ServiceAccount token for OAuth client secret")
		}
		err = controllerutil.SetControllerReference(cr, secret, r.Scheme)
		if err != nil {
			return nil, err
		}
		tokenSecret = &corev1.ObjectReference{
			Name:      secret.Name,
			Namespace: cr.Namespace,
		}
		sa.Secrets = append(sa.Secrets, *tokenSecret)
		err = r.Client.Update(context.TODO(), sa)
		if err != nil {
			return nil, e.New("failed to add ServiceAccount token for OAuth client secret")
		}
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
func (r *ReconcileArgoCD) reconcileDexConfiguration(cm *corev1.ConfigMap, cr *argoproj.ArgoCD) error {
	actual := cm.Data[common.ArgoCDKeyDexConfig]
	desired := getDexConfig(cr)

	// If no dexConfig expressed but openShiftOAuth is requested through `.spec.sso.dex`, use default
	// openshift dex config
	if len(desired) <= 0 && (cr.Spec.SSO != nil && cr.Spec.SSO.Dex != nil && cr.Spec.SSO.Dex.OpenShiftOAuth) {
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
func (r *ReconcileArgoCD) getOpenShiftDexConfig(cr *argoproj.ArgoCD) (string, error) {

	groups := []string{}

	// Allow override of groups from CR
	if cr.Spec.SSO != nil && cr.Spec.SSO.Dex != nil && cr.Spec.SSO.Dex.Groups != nil {
		groups = cr.Spec.SSO.Dex.Groups
	}

	connector := DexConnector{
		Type: "openshift",
		ID:   "openshift",
		Name: "OpenShift",
		Config: map[string]interface{}{
			"issuer":       "https://kubernetes.default.svc", // TODO: Should this be hard-coded?
			"clientID":     getDexOAuthClientID(cr),
			"clientSecret": "$oidc.dex.clientSecret",
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
func (r *ReconcileArgoCD) reconcileDexServiceAccount(cr *argoproj.ArgoCD) error {

	// if openShiftOAuth set to false in `.spec.sso.dex`, no need to configure it
	if cr.Spec.SSO == nil || cr.Spec.SSO.Dex == nil || !cr.Spec.SSO.Dex.OpenShiftOAuth {
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
func (r *ReconcileArgoCD) reconcileDexDeployment(cr *argoproj.ArgoCD) error {
	deploy := newDeploymentWithSuffix("dex-server", "dex-server", cr)

	AddSeccompProfileForOpenShift(r.Client, &deploy.Spec.Template.Spec)

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command: []string{
			"/shared/argocd-dex",
			"rundex",
		},
		Image: getDexContainerImage(cr),
		Name:  "dex",
		Env:   proxyEnvVars(),
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
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			RunAsNonRoot: boolPtr(true),
		},
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
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			RunAsNonRoot: boolPtr(true),
		},
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

		// dex uninstallation requested
		if !UseDex(cr) {
			log.Info("deleting the existing dex deployment because dex uninstallation has been requested")
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

	// if Dex installation has not been requested, do nothing
	if !UseDex(cr) {
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("creating deployment %s for Argo CD instance %s in namespace %s", deploy.Name, cr.Name, cr.Namespace))
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileDexService will ensure that the Service for Dex is present.
func (r *ReconcileArgoCD) reconcileDexService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("dex-server", "dex-server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {

		// dex uninstallation requested
		if !UseDex(cr) {
			log.Info("deleting the existing Dex service because dex uninstallation has been requested")
			return r.Client.Delete(context.TODO(), svc)
		}
		return nil
	}

	// if Dex installation has not been requested, do nothing
	if !UseDex(cr) {
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

	log.Info(fmt.Sprintf("creating service %s for Argo CD instance %s in namespace %s", svc.Name, cr.Name, cr.Namespace))
	return r.Client.Create(context.TODO(), svc)
}

// reconcileDexResources consolidates all dex resources reconciliation calls. It serves as the single place to trigger both creation
// and deletion of dex resources based on the specified configuration of dex
func (r *ReconcileArgoCD) reconcileDexResources(cr *argoproj.ArgoCD) error {

	if _, err := r.reconcileRole(common.ArgoCDDexServerComponent, policyRuleForDexServer(), cr); err != nil {
		log.Error(err, "error reconciling dex role")
	}

	if err := r.reconcileRoleBinding(common.ArgoCDDexServerComponent, policyRuleForDexServer(), cr); err != nil {
		log.Error(err, "error reconciling dex rolebinding")
	}

	if err := r.reconcileServiceAccountPermissions(common.ArgoCDDexServerComponent, policyRuleForDexServer(), cr); err != nil {
		return err
	}

	// specialized handling for dex
	if err := r.reconcileDexServiceAccount(cr); err != nil {
		log.Error(err, "error reconciling dex serviceaccount")
	}

	// Reconcile dex config in argocd-cm, create dex config in argocd-cm if required (right after dex is enabled)
	if err := r.reconcileArgoConfigMap(cr); err != nil {
		log.Error(err, "error reconciling argocd-cm configmap")
	}

	if err := r.reconcileDexService(cr); err != nil {
		log.Error(err, "error reconciling dex service")
	}

	if err := r.reconcileDexDeployment(cr); err != nil {
		log.Error(err, "error reconciling dex deployment")
	}

	if err := r.reconcileStatusSSO(cr); err != nil {
		log.Error(err, "error reconciling dex status")
	}

	return nil
}

// The code to create/delete notifications resources is written within the reconciliation logic itself. However, these functions must be called
// in the right order depending on whether resources are getting created or deleted. During creation we must create the role and sa first.
// RoleBinding and deployment are dependent on these resouces. During deletion the order is reversed.
// Deployment and RoleBinding must be deleted before the role and sa. deleteDexResources will only be called during
// delete events, so we don't need to worry about duplicate, recurring reconciliation calls
func (r *ReconcileArgoCD) deleteDexResources(cr *argoproj.ArgoCD) error {

	sa := &corev1.ServiceAccount{}
	role := &rbacv1.Role{}

	if err := argoutil.FetchObject(r.Client, cr.Namespace, fmt.Sprintf("%s-%s", cr.Name, common.ArgoCDDexServerComponent), sa); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}
	if err := argoutil.FetchObject(r.Client, cr.Namespace, fmt.Sprintf("%s-%s", cr.Name, common.ArgoCDDexServerComponent), role); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	if err := r.reconcileDexDeployment(cr); err != nil {
		log.Error(err, "error reconciling dex deployment")
	}

	if err := r.reconcileDexService(cr); err != nil {
		log.Error(err, "error reconciling dex service")
	}

	// Reconcile dex config in argocd-cm (right after dex is disabled)
	// this is required for a one time trigger of reconcileDexConfiguration directly in case of a dex deletion event,
	// since reconcileArgoConfigMap won't call reconcileDexConfiguration once dex has been disabled (to avoid reconciling on
	// dexconfig unnecessarily when it isn't enabled)
	cm := newConfigMapWithName(common.ArgoCDConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if err := r.reconcileDexConfiguration(cm, cr); err != nil {
			log.Error(err, "error reconciling dex configuration in configmap")
		}
	}

	if err := r.reconcileRoleBinding(common.ArgoCDDexServerComponent, policyRuleForDexServer(), cr); err != nil {
		log.Error(err, "error reconciling dex rolebinding")
	}

	if err := r.reconcileStatusSSO(cr); err != nil {
		log.Error(err, "error reconciling dex status")
	}

	return nil
}
