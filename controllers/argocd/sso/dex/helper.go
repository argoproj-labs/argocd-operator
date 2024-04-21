package dex

import (
	"fmt"
	"strings"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	dexConfigKey       = "dex.config"
	dexTokenSecretName = "argocd-dex-server-token-secret"
)

type DexConnector struct {
	Config map[string]interface{} `yaml:"config,omitempty"`
	ID     string                 `yaml:"id"`
	Name   string                 `yaml:"name"`
	Type   string                 `yaml:"type"`
}

// getContainerImage will return the container image for the Redis server.
func (dr *DexReconciler) getContainerImage() string {
	fn := func(cr *argoproj.ArgoCD) (string, string) {
		return cr.Spec.SSO.Dex.Image, cr.Spec.SSO.Dex.Version
	}
	return argocdcommon.GetContainerImage(fn, dr.Instance, common.ArgoCDDexImageEnvVar, common.DefaultDexImage, common.DefaultDexVersion)
}

func (dr *DexReconciler) getOAuthClientID() string {
	return fmt.Sprintf("system:serviceaccount:%s:%s", dr.Instance.Namespace, resourceName)
}

// GetServerAddress will return the Redis service address for the given ArgoCD instance
func (dr *DexReconciler) GetServerAddress() string {
	return argoutil.FQDNwithPort(resourceName, dr.Instance.Namespace, common.DefaultDexHTTPPort)
}

func (dr *DexReconciler) GetOAuthRedirectURI() string {
	uri := dr.Server.GetURI()
	return uri + common.DefaultDexOAuthRedirectPath
}

func (dr *DexReconciler) getResources() corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}
	// Allow override of resource requirements from dr.Instance
	if dr.Instance.Spec.SSO.Dex.Resources != nil {
		resources = *dr.Instance.Spec.SSO.Dex.Resources
	}

	return resources
}

func (dr *DexReconciler) getConfig() string {
	config := common.DefaultDexConfig

	// Allow override of config from dr.Instance
	if dr.Instance.Spec.ExtraConfig[dexConfigKey] != "" {
		config = dr.Instance.Spec.ExtraConfig[dexConfigKey]
	} else if dr.Instance.Spec.SSO.Dex != nil && len(dr.Instance.Spec.SSO.Dex.Config) > 0 {
		config = dr.Instance.Spec.SSO.Dex.Config
	}

	return config
}

func (dr *DexReconciler) GetOAuthClientSecret() (*string, error) {
	dr.varSetter()
	sa, err := permissions.GetServiceAccount(resourceName, dr.Instance.Namespace, dr.Client)
	if err != nil {
		return nil, errors.Wrapf(err, "getOAuthClientSecret: failed to retrieve service account %s", resourceName)
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
		// This change of creating secret for dex service account,is due to change of reduction of secret-based service account tokens in k8s v1.24 so from k8s v1.24 no default secret for service account is created, but for dex to work we need to provide token of secret used by dex service account as a oauth token, this change helps to achieve it, in long run we should see if dex really requires a secret or it manages to create one using TokenRequest API or may be change how dex is used or configured by operator

		req := workloads.SecretRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dexTokenSecretName,
				Namespace: dr.Instance.Namespace,
				Annotations: map[string]string{
					corev1.ServiceAccountNameKey: sa.Name,
				},
			},
			Type: corev1.SecretTypeServiceAccountToken,
		}

		secret, err := workloads.RequestSecret(req)
		if err != nil {
			return nil, errors.Wrapf(err, "getOAuthClientSecret: failed to request secret %s", dexTokenSecretName)
		}

		err = controllerutil.SetControllerReference(dr.Instance, secret, dr.Scheme)
		if err != nil {
			dr.Logger.Error(err, "getOAuthClientSecret")
		}

		if err := workloads.CreateSecret(secret, dr.Client); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return nil, errors.Wrapf(err, "getOAuthClientSecret: failed to create secret %s", dexTokenSecretName)
			}
		}

		tokenSecret = &corev1.ObjectReference{
			Name:      secret.Name,
			Namespace: dr.Instance.Namespace,
		}
		sa.Secrets = append(sa.Secrets, *tokenSecret)

		if err := workloads.UpdateSecret(secret, dr.Client); err != nil {
			return nil, errors.Wrapf(err, "getOAuthClientSecret: failed to update secret %s", dexTokenSecretName)
		}
	}

	// Fetch the secret to obtain the token
	secret, err := workloads.GetSecret(dexTokenSecretName, dr.Instance.Namespace, dr.Client)
	if err != nil {
		return nil, errors.Wrapf(err, "getOAuthClientSecret: failed to retrieve secret %s", dexTokenSecretName)
	}

	token := string(secret.Data["token"])
	return &token, nil
}

func getMapFromConfigStr(dexCfgStr string, dex map[string]interface{}) error {
	dexCfg := make(map[string]interface{})

	if err := yaml.Unmarshal([]byte(dexCfgStr), dexCfg); err != nil {
		return err
	}

	return nil
}

func (dr *DexReconciler) GetConfig() string {
	desiredCfg := dr.getConfig()

	if dr.Instance.Spec.SSO.Dex.OpenShiftOAuth {
		cfg, err := dr.getOpenShiftConfig()
		if err != nil {
			dr.Logger.Error(err, "GetConfig: failed to get openshift config")
			return ""
		}
		desiredCfg = cfg
	}

	return desiredCfg
}
