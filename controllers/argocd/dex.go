package argocd

import (
	"context"
	"errors"
	e "errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	"github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/scheme"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
		// This change of creating secret for dex service account,is due to
		// change of reduction of secret-based service account tokens in k8s
		// v1.24 so from k8s v1.24 no default secret for service account is
		// created, but for dex to work we need to provide token of secret used
		// by dex service account as a oauth token, this change helps to achieve
		// it, in long run we should see do dex really requires a secret or it
		// manages to create one using TokenRequest API or may be change how dex
		// is used or configured by operator
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
		argoutil.AddTrackedByOperatorLabel(&secret.ObjectMeta)
		argoutil.LogResourceCreation(log, secret)
		err := r.Create(context.TODO(), secret)
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
		argoutil.LogResourceUpdate(log, sa, "adding ServiceAccount token for OAuth client secret")
		err = r.Update(context.TODO(), sa)
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

func oAuthEndpointReachable(cfg *rest.Config) (bool, error) {
	if cfg == nil {
		return false, fmt.Errorf("rest.Config is nil")
	}

	restCfg := rest.CopyConfig(cfg)
	restCfg.APIPath = "/"
	restCfg.GroupVersion = &schema.GroupVersion{}
	restCfg.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	client, err := rest.UnversionedRESTClientFor(restCfg)
	if err != nil {
		return false, err
	}

	raw, err := client.Get().AbsPath("/.well-known/oauth-authorization-server").Do(context.TODO()).Raw()

	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, errors.New("OAuth endpoint not found at /.well-known/oauth-authorization-server")
		}
		return false, err
	}

	return len(raw) > 0, nil
}

// reconcileDexConfiguration will ensure that Dex is configured properly.
func (r *ReconcileArgoCD) reconcileDexConfiguration(cm *corev1.ConfigMap, cr *argoproj.ArgoCD) error {
	actual := cm.Data[common.ArgoCDKeyDexConfig]
	desired := getDexConfig(cr)
	// Append the default OpenShift dex config if the openShiftOAuth is requested through `.spec.sso.dex`.
	if cr.Spec.SSO != nil && cr.Spec.SSO.Dex != nil && cr.Spec.SSO.Dex.OpenShiftOAuth {
		if reachable, oauthErr := oAuthEndpointReachable(r.Config); reachable && oauthErr == nil {
			cfg, err := r.getOpenShiftDexConfig(cr)
			if err != nil {
				return err
			}
			desired = cfg
		}
	}

	if actual != desired {
		// Update ConfigMap with desired configuration.
		cm.Data[common.ArgoCDKeyDexConfig] = desired
		argoutil.LogResourceUpdate(log, cm, "updating dex configuration")
		if err := r.Update(context.TODO(), cm); err != nil {
			return err
		}

		// Trigger rollout of Dex Deployment to pick up changes.
		deploy := newDeploymentWithSuffix("dex-server", "dex-server", cr)
		deplExists, err := argoutil.IsObjectFound(r.Client, deploy.Namespace, deploy.Name, deploy)
		if err != nil {
			return err
		}
		if !deplExists {
			log.Info("unable to locate dex deployment")
			return nil
		}

		deploy.Spec.Template.Labels["dex.config.changed"] = time.Now().UTC().Format("01022006-150406-MST")
		argoutil.LogResourceUpdate(log, deploy, "to trigger dex deployment rollout")
		return r.Update(context.TODO(), deploy)
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

	redirectURI, err := r.getDexOAuthRedirectURI(cr)
	if err != nil {
		return "", err
	}

	connector := DexConnector{
		Type: "openshift",
		ID:   "openshift",
		Name: "OpenShift",
		Config: map[string]interface{}{
			"issuer":       "https://kubernetes.default.svc", // TODO: Should this be hard-coded?
			"clientID":     getDexOAuthClientID(cr),
			"clientSecret": "$oidc.dex.clientSecret",
			"redirectURI":  redirectURI,
			"insecureCA":   true, // TODO: Configure for openshift CA,
			"groups":       groups,
		},
	}

	connectors := make([]DexConnector, 0)
	connectors = append(connectors, connector)

	dex := make(map[string]interface{})
	dex["connectors"] = connectors

	// add dex config from the Argo CD CR.
	if err := addDexConfigFromCR(cr, dex); err != nil {
		return "", err
	}

	bytes, err := yaml.Marshal(dex)
	return string(bytes), err
}

func addDexConfigFromCR(cr *argoproj.ArgoCD, dex map[string]interface{}) error {
	dexCfgStr := getDexConfig(cr)
	if dexCfgStr == "" {
		return nil
	}

	dexCfg := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(dexCfgStr), dexCfg); err != nil {
		return err
	}

	for k, v := range dexCfg {
		dex[k] = v
	}

	return nil
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
	uri, err := r.getDexOAuthRedirectURI(cr)
	if err != nil {
		return err
	}
	log.Info(fmt.Sprintf("URI: %s", uri))

	// Get the current redirect URI
	ann := sa.Annotations
	currentURI, found := ann[common.ArgoCDKeyDexOAuthRedirectURI]
	if found && currentURI == uri {
		return nil // Redirect URI annotation found and correct, move along...
	}

	log.Info(fmt.Sprintf("current URI: %s is not correct, should be: %s", currentURI, uri))
	if len(ann) <= 0 {
		ann = make(map[string]string)
	}

	ann[common.ArgoCDKeyDexOAuthRedirectURI] = uri
	sa.Annotations = ann

	argoutil.LogResourceUpdate(log, sa, "updating redirect uri")
	return r.Update(context.TODO(), sa)
}

// reconcileDexDeployment will ensure the Deployment resource is present for the ArgoCD Dex component.
func (r *ReconcileArgoCD) reconcileDexDeployment(cr *argoproj.ArgoCD) error {
	deploy := newDeploymentWithSuffix("dex-server", "dex-server", cr)

	AddSeccompProfileForOpenShift(r.Client, &deploy.Spec.Template.Spec)

	dexEnv := proxyEnvVars()

	dexVolumes := []corev1.Volume{
		{
			Name: "static-files",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "dexconfig",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	dexVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "static-files",
			MountPath: "/shared",
		},
		{
			Name:      "dexconfig",
			MountPath: "/tmp",
		},
	}

	if cr.Spec.SSO != nil && cr.Spec.SSO.Dex != nil {
		dexEnv = append(dexEnv, cr.Spec.SSO.Dex.Env...)

		if cr.Spec.SSO.Dex.Volumes != nil {
			dexVolumes = append(dexVolumes, cr.Spec.SSO.Dex.Volumes...)
		}

		if cr.Spec.SSO.Dex.VolumeMounts != nil {
			dexVolumeMounts = append(dexVolumeMounts, cr.Spec.SSO.Dex.VolumeMounts...)
		}
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command: []string{
			"/shared/argocd-dex",
			"rundex",
		},
		Image:           getDexContainerImage(cr),
		ImagePullPolicy: argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy),
		Name:            "dex",
		Env:             dexEnv,
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
		Resources:       getDexResources(cr),
		SecurityContext: argoutil.DefaultSecurityContext(),
		VolumeMounts:    dexVolumeMounts,
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
		ImagePullPolicy: argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy),
		Name:            "copyutil",
		Resources:       getDexResources(cr),
		SecurityContext: argoutil.DefaultSecurityContext(),
		VolumeMounts:    dexVolumeMounts,
	}}

	deploy.Spec.Template.Spec.ServiceAccountName = fmt.Sprintf("%s-%s", cr.Name, common.ArgoCDDefaultDexServiceAccountName)
	deploy.Spec.Template.Spec.Volumes = dexVolumes

	existing := newDeploymentWithSuffix("dex-server", "dex-server", cr)
	deplExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing)
	if err != nil {
		return err
	}
	if deplExists {

		// dex uninstallation requested
		if !UseDex(cr) {
			argoutil.LogResourceDeletion(log, existing, "dex uninstallation has been requested")
			return r.Delete(context.TODO(), existing)
		}
		changed := false
		explanation := ""

		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getDexContainerImage(cr)
		actualImagePullPolicy := existing.Spec.Template.Spec.Containers[0].ImagePullPolicy
		desiredImagePullPolicy := argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			explanation = "container image"
			changed = true
		}
		if actualImagePullPolicy != desiredImagePullPolicy {
			existing.Spec.Template.Spec.Containers[0].ImagePullPolicy = desiredImagePullPolicy
			if changed {
				explanation += ", "
			}
			explanation += "image pull policy"
			changed = true
		}

		actualImage = existing.Spec.Template.Spec.InitContainers[0].Image
		desiredImage = getArgoContainerImage(cr)
		actualInitImagePullPolicy := existing.Spec.Template.Spec.InitContainers[0].ImagePullPolicy
		desiredInitImagePullPolicy := argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.InitContainers[0].Image = desiredImage
			existing.Spec.Template.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			if changed {
				explanation += ", "
			}
			explanation += "init container image"
			changed = true
		}
		if actualInitImagePullPolicy != desiredInitImagePullPolicy {
			existing.Spec.Template.Spec.InitContainers[0].ImagePullPolicy = desiredInitImagePullPolicy
			if changed {
				explanation += ", "
			}
			explanation += "init container image pull policy"
			changed = true
		}
		updateNodePlacement(existing, deploy, &changed, &explanation)
		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			deploy.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			if changed {
				explanation += ", "
			}
			explanation += "container env"
			changed = true
		}

		if !reflect.DeepEqual(existing.Spec.Template.Spec.InitContainers[0].Env,
			deploy.Spec.Template.Spec.InitContainers[0].Env) {
			existing.Spec.Template.Spec.InitContainers[0].Env = deploy.Spec.Template.Spec.InitContainers[0].Env
			if changed {
				explanation += ", "
			}
			explanation += "init container env"
			changed = true
		}
		if !reflect.DeepEqual(existing.Spec.Template.Spec.InitContainers[0].SecurityContext,
			deploy.Spec.Template.Spec.InitContainers[0].SecurityContext) {
			existing.Spec.Template.Spec.InitContainers[0].SecurityContext = deploy.Spec.Template.Spec.InitContainers[0].SecurityContext
			if changed {
				explanation += ", "
			}
			explanation += "init container security context"
			changed = true
		}

		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Resources, existing.Spec.Template.Spec.Containers[0].Resources) {
			existing.Spec.Template.Spec.Containers[0].Resources = deploy.Spec.Template.Spec.Containers[0].Resources
			if changed {
				explanation += ", "
			}
			explanation += "container resources"
			changed = true
		}

		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].SecurityContext, existing.Spec.Template.Spec.Containers[0].SecurityContext) {
			existing.Spec.Template.Spec.Containers[0].SecurityContext = deploy.Spec.Template.Spec.Containers[0].SecurityContext
			if changed {
				explanation += ", "
			}
			explanation += "container security context"
			changed = true
		}

		if !reflect.DeepEqual(deploy.Spec.Template.Spec.SecurityContext, existing.Spec.Template.Spec.SecurityContext) {
			existing.Spec.Template.Spec.SecurityContext = deploy.Spec.Template.Spec.SecurityContext
			if changed {
				explanation += ", "
			}
			explanation += "pod security context"
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].VolumeMounts, existing.Spec.Template.Spec.Containers[0].VolumeMounts) {
			existing.Spec.Template.Spec.Containers[0].VolumeMounts = deploy.Spec.Template.Spec.Containers[0].VolumeMounts
			if changed {
				explanation += ", "
			}
			explanation += "container volume mounts"
			changed = true
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Volumes, existing.Spec.Template.Spec.Volumes) {
			existing.Spec.Template.Spec.Volumes = deploy.Spec.Template.Spec.Volumes
			if changed {
				explanation += ", "
			}
			explanation += "volumes"
			changed = true
		}

		if changed {
			argoutil.LogResourceUpdate(log, existing, "updating", explanation)
			return r.Update(context.TODO(), existing)
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

	argoutil.LogResourceCreation(log, deploy)
	return r.Create(context.TODO(), deploy)
}

// reconcileDexService will ensure that the Service for Dex is present.
func (r *ReconcileArgoCD) reconcileDexService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("dex-server", "dex-server", cr)

	svcExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc)
	if err != nil {
		return err
	}
	if svcExists {

		// dex uninstallation requested
		if !UseDex(cr) {
			argoutil.LogResourceDeletion(log, svc, "dex uninstallation has been requested")
			return r.Delete(context.TODO(), svc)
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

	argoutil.LogResourceCreation(log, svc)
	return r.Create(context.TODO(), svc)
}

// reconcileDexResources consolidates all dex resources reconciliation calls. It serves as the single place to trigger both creation
// and deletion of dex resources based on the specified configuration of dex
func (r *ReconcileArgoCD) reconcileDexResources(cr *argoproj.ArgoCD) error {
	if _, err := r.reconcileRole(common.ArgoCDDexServerComponent, policyRuleForDexServer(), cr); err != nil {
		log.Error(err, "error reconciling dex role")
		return err
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
		return err
	}

	// Reconcile dex config in argocd-cm, create dex config in argocd-cm if required (right after dex is enabled)
	if err := r.reconcileArgoConfigMap(cr); err != nil {
		log.Error(err, "error reconciling argocd-cm configmap")
		return err
	}

	if err := r.reconcileDexService(cr); err != nil {
		log.Error(err, "error reconciling dex service")
		return err
	}

	if err := r.reconcileDexDeployment(cr); err != nil {
		log.Error(err, "error reconciling dex deployment")
		return err
	}

	return nil
}
