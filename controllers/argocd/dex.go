package argocd

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"

	"gopkg.in/yaml.v2"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// getDexServerTokenSecretName returns the name of the Secret that stores the Dex OAuth client token.
func getDexServerTokenSecretName(cr *argoproj.ArgoCD) string {
	return argoutil.GetSecretNameWithSuffix(cr, common.ArgoCDDefaultDexServiceAccountName+"-token")
}

// needsDexTokenRenewal returns true when the token is missing, unparseable, or within the renewal window.
func needsDexTokenRenewal(secret *corev1.Secret) bool {
	expiryBytes, ok := secret.Data["expiry"]
	if !ok {
		return true
	}
	expiry, err := time.Parse(time.RFC3339, string(expiryBytes))
	if err != nil {
		return true
	}
	renewThreshold := time.Duration(common.ArgoCDDexServerTokenExpirySecs/common.ArgoCDDexServerTokenRenewalThresholdFraction) * time.Second
	return time.Until(expiry) < renewThreshold
}

// getDexOAuthClientSecret returns a time-limited Dex OAuth client token via the TokenRequest API.
func (r *ReconcileArgoCD) getDexOAuthClientSecret(cr *argoproj.ArgoCD) (*string, error) {
	sa := newServiceAccountWithName(common.ArgoCDDefaultDexServiceAccountName, cr)
	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		return nil, err
	}

	tokenSecretName := getDexServerTokenSecretName(cr)
	tokenSecret := &corev1.Secret{}
	fetchErr := argoutil.FetchObject(r.Client, cr.Namespace, tokenSecretName, tokenSecret)
	if fetchErr != nil && !apierrors.IsNotFound(fetchErr) {
		return nil, fetchErr
	}
	secretExists := fetchErr == nil

	// Return the cached token if it is still valid.
	if secretExists && !needsDexTokenRenewal(tokenSecret) {
		token := string(tokenSecret.Data["token"])
		// Schedule the next reconcile to run just before the renewal threshold so
		// the token is proactively renewed without waiting for an external event.
		if expiry, parseErr := time.Parse(time.RFC3339, string(tokenSecret.Data["expiry"])); parseErr == nil {
			renewThreshold := time.Duration(common.ArgoCDDexServerTokenExpirySecs/common.ArgoCDDexServerTokenRenewalThresholdFraction) * time.Second
			if d := time.Until(expiry) - renewThreshold; d > 0 {
				r.dexTokenRequeueAfter.Store(cr.Namespace, d)
			}
		}
		return &token, nil
	}

	// Request a new time-limited token via the TokenRequest API.
	expirationSeconds := common.ArgoCDDexServerTokenExpirySecs
	tokenRequest, err := r.K8sClient.CoreV1().ServiceAccounts(cr.Namespace).CreateToken(
		context.TODO(),
		sa.Name,
		&authv1.TokenRequest{
			Spec: authv1.TokenRequestSpec{
				ExpirationSeconds: &expirationSeconds,
			},
		},
		metav1.CreateOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create token for dex service account %s: %w", sa.Name, err)
	}

	expiryStr := tokenRequest.Status.ExpirationTimestamp.UTC().Format(time.RFC3339)
	tokenData := map[string][]byte{
		"token":  []byte(tokenRequest.Status.Token),
		"expiry": []byte(expiryStr),
	}

	if !secretExists {
		newSecret := argoutil.NewSecretWithSuffix(cr, common.ArgoCDDefaultDexServiceAccountName+"-token")
		newSecret.Type = corev1.SecretTypeOpaque
		newSecret.Data = tokenData
		argoutil.AddTrackedByOperatorLabel(&newSecret.ObjectMeta)
		if err := controllerutil.SetControllerReference(cr, newSecret, r.Scheme); err != nil {
			return nil, err
		}
		argoutil.LogResourceCreation(log, newSecret)
		if err := r.Create(context.TODO(), newSecret); err != nil {
			return nil, err
		}
	} else {
		tokenSecret.Data = tokenData
		argoutil.LogResourceUpdate(log, tokenSecret, "renewing dex OAuth client token")
		if err := r.Update(context.TODO(), tokenSecret); err != nil {
			return nil, err
		}
	}

	// Schedule the next reconcile just before the renewal threshold.
	renewThreshold := time.Duration(common.ArgoCDDexServerTokenExpirySecs/common.ArgoCDDexServerTokenRenewalThresholdFraction) * time.Second
	if d := time.Until(tokenRequest.Status.ExpirationTimestamp.Time) - renewThreshold; d > 0 {
		r.dexTokenRequeueAfter.Store(cr.Namespace, d)
	}

	token := tokenRequest.Status.Token
	return &token, nil
}

// reconcileDexLegacySATokenSecrets deletes non-expiring kubernetes.io/service-account-token
// Secrets for the Dex SA and removes their stale references from the SA.
func (r *ReconcileArgoCD) reconcileDexLegacySATokenSecrets(cr *argoproj.ArgoCD) error {
	dexSAName := newServiceAccountWithName(common.ArgoCDDefaultDexServiceAccountName, cr).Name
	secretList := &corev1.SecretList{}
	if err := r.List(context.TODO(), secretList,
		client.InNamespace(cr.Namespace),
		client.MatchingLabels(map[string]string{
			common.ArgoCDTrackedByOperatorLabel: common.ArgoCDAppName,
		}),
	); err != nil {
		return err
	}
	for i := range secretList.Items {
		s := &secretList.Items[i]
		if s.Type != corev1.SecretTypeServiceAccountToken {
			continue
		}
		if s.Annotations[corev1.ServiceAccountNameKey] != dexSAName {
			continue
		}
		if !strings.HasPrefix(s.Name, "argocd-dex-server-token-") {
			continue
		}
		argoutil.LogResourceDeletion(log, s, "removing legacy Dex service account token secret")
		if err := r.Delete(context.TODO(), s); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	sa := newServiceAccountWithName(common.ArgoCDDefaultDexServiceAccountName, cr)
	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	var filtered []corev1.ObjectReference
	for _, ref := range sa.Secrets {
		if strings.Contains(ref.Name, "dex-server-token") {
			continue
		}
		filtered = append(filtered, ref)
	}
	if len(filtered) != len(sa.Secrets) {
		sa.Secrets = filtered
		argoutil.LogResourceUpdate(log, sa, "removing legacy token secret references from Dex service account")
		if err := r.Update(context.TODO(), sa); err != nil {
			return err
		}
	}
	return nil
}

// reconcileDexConfiguration will ensure that Dex is configured properly.
func (r *ReconcileArgoCD) reconcileDexConfiguration(cm *corev1.ConfigMap, cr *argoproj.ArgoCD) error {
	actual := cm.Data[common.ArgoCDKeyDexConfig]
	desired := getDexConfig(cr)

	// Append the default OpenShift dex config if the openShiftOAuth is requested through `.spec.sso.dex`.
	if cr.Spec.SSO != nil && cr.Spec.SSO.Dex != nil && cr.Spec.SSO.Dex.OpenShiftOAuth {
		cfg, err := r.getOpenShiftDexConfig(cr)
		if err != nil {
			return err
		}
		desired = cfg
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

func IsExternalAuthenticationEnabledOnCluster(ctx context.Context, c client.Client) bool {
	var authConfig configv1.Authentication
	if err := c.Get(ctx, types.NamespacedName{Name: "cluster"}, &authConfig); err != nil {
		log.Error(err, "unable to fetch cluster authentication configuration")
		return false
	}
	return authConfig.Spec.Type == "OIDC"
}

// getOpenShiftDexConfig will return the configuration for the Dex server running on OpenShift.
func (r *ReconcileArgoCD) getOpenShiftDexConfig(cr *argoproj.ArgoCD) (string, error) {
	if IsOpenShiftCluster() && IsExternalAuthenticationEnabledOnCluster(context.TODO(), r.Client) {
		if updateStatusErr := updateStatusAndConditionsOfArgoCD(context.TODO(), createCondition(argoproj.OpenShiftOAuthErrorMessage), cr, &cr.Status, r.Client, log); updateStatusErr != nil {
			log.Error(updateStatusErr, "unable to update status of ArgoCD")
			return "", updateStatusErr
		}
		return "", nil
	}
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

		var changes []string
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getDexContainerImage(cr)
		actualImagePullPolicy := existing.Spec.Template.Spec.Containers[0].ImagePullPolicy
		desiredImagePullPolicy := argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changes = append(changes, "container image")
		}
		if actualImagePullPolicy != desiredImagePullPolicy {
			existing.Spec.Template.Spec.Containers[0].ImagePullPolicy = desiredImagePullPolicy
			changes = append(changes, "image pull policy")
		}

		actualImage = existing.Spec.Template.Spec.InitContainers[0].Image
		desiredImage = getArgoContainerImage(cr)
		actualInitImagePullPolicy := existing.Spec.Template.Spec.InitContainers[0].ImagePullPolicy
		desiredInitImagePullPolicy := argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.InitContainers[0].Image = desiredImage
			existing.Spec.Template.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changes = append(changes, "init container image")
		}
		if actualInitImagePullPolicy != desiredInitImagePullPolicy {
			existing.Spec.Template.Spec.InitContainers[0].ImagePullPolicy = desiredInitImagePullPolicy
			changes = append(changes, "init container image pull policy")
		}

		changes = append(changes, updateNodePlacement(existing, deploy)...)

		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			deploy.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			changes = append(changes, "container env")
		}

		if !reflect.DeepEqual(existing.Spec.Template.Spec.InitContainers[0].Env,
			deploy.Spec.Template.Spec.InitContainers[0].Env) {
			existing.Spec.Template.Spec.InitContainers[0].Env = deploy.Spec.Template.Spec.InitContainers[0].Env
			changes = append(changes, "init container env")
		}
		if !reflect.DeepEqual(existing.Spec.Template.Spec.InitContainers[0].SecurityContext,
			deploy.Spec.Template.Spec.InitContainers[0].SecurityContext) {
			existing.Spec.Template.Spec.InitContainers[0].SecurityContext = deploy.Spec.Template.Spec.InitContainers[0].SecurityContext
			changes = append(changes, "init container security context")
		}

		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Resources, existing.Spec.Template.Spec.Containers[0].Resources) {
			existing.Spec.Template.Spec.Containers[0].Resources = deploy.Spec.Template.Spec.Containers[0].Resources
			changes = append(changes, "container resources")
		}

		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].SecurityContext, existing.Spec.Template.Spec.Containers[0].SecurityContext) {
			existing.Spec.Template.Spec.Containers[0].SecurityContext = deploy.Spec.Template.Spec.Containers[0].SecurityContext
			changes = append(changes, "container security context")
		}

		if !reflect.DeepEqual(deploy.Spec.Template.Spec.SecurityContext, existing.Spec.Template.Spec.SecurityContext) {
			existing.Spec.Template.Spec.SecurityContext = deploy.Spec.Template.Spec.SecurityContext
			changes = append(changes, "pod security context")
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].VolumeMounts, existing.Spec.Template.Spec.Containers[0].VolumeMounts) {
			existing.Spec.Template.Spec.Containers[0].VolumeMounts = deploy.Spec.Template.Spec.Containers[0].VolumeMounts
			changes = append(changes, "container volume mounts")
		}
		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Volumes, existing.Spec.Template.Spec.Volumes) {
			existing.Spec.Template.Spec.Volumes = deploy.Spec.Template.Spec.Volumes
			changes = append(changes, "volumes")
		}

		if len(changes) > 0 {
			argoutil.LogResourceUpdate(log, existing, "updating", strings.Join(changes, ", "))
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
