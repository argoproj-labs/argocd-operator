package argocd

import (
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// getDexContainerImage will return the container image for the Dex server.
//
// There are three possible options for configuring the image, and this is the
// order of preference.
//
// 1. from the Spec, the spec.sso.dex field has an image and version to use for
// generating an image reference.
// 2. from the Environment, this looks for the `ARGOCD_DEX_IMAGE` field and uses
// that if the spec is not configured.
// 3. the default is configured in common.ArgoCDDefaultDexVersion and
// common.ArgoCDDefaultDexImage.
func getDexContainerImage(cr *argoproj.ArgoCD) string {
	defaultImg, defaultTag := false, false

	img := ""
	tag := ""

	if cr.Spec.SSO != nil && cr.Spec.SSO.Dex != nil && cr.Spec.SSO.Dex.Image != "" {
		img = cr.Spec.SSO.Dex.Image
	}

	if img == "" {
		img = common.ArgoCDDefaultDexImage
		defaultImg = true
	}

	if cr.Spec.SSO != nil && cr.Spec.SSO.Dex != nil && cr.Spec.SSO.Dex.Version != "" {
		tag = cr.Spec.SSO.Dex.Version
	}

	if tag == "" {
		tag = common.ArgoCDDefaultDexVersion
		defaultTag = true
	}
	if e := os.Getenv(common.ArgoCDDexImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}
	return argoutil.CombineImageTag(img, tag)
}

// getDexOAuthRedirectURI will return the OAuth redirect URI for the Dex server.
func (r *ReconcileArgoCD) getDexOAuthRedirectURI(cr *argoproj.ArgoCD) string {
	uri := r.getArgoServerURI(cr)
	return uri + common.ArgoCDDefaultDexOAuthRedirectPath
}

// getDexOAuthClientID will return the OAuth client ID for the given ArgoCD.
func getDexOAuthClientID(cr *argoproj.ArgoCD) string {
	return fmt.Sprintf("system:serviceaccount:%s:%s", cr.Namespace, fmt.Sprintf("%s-%s", cr.Name, common.ArgoCDDefaultDexServiceAccountName))
}

// getDexResources will return the ResourceRequirements for the Dex container.
func getDexResources(cr *argoproj.ArgoCD) corev1.ResourceRequirements {

	resources := v1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.SSO != nil && cr.Spec.SSO.Dex != nil && cr.Spec.SSO.Dex.Resources != nil {
		resources = *cr.Spec.SSO.Dex.Resources
	}

	return resources
}

func getDexConfig(cr *argoproj.ArgoCD) string {
	config := common.ArgoCDDefaultDexConfig

	// Allow override of config from CR
	if cr.Spec.ExtraConfig["dex.config"] != "" {
		config = cr.Spec.ExtraConfig["dex.config"]
	} else if cr.Spec.SSO != nil && cr.Spec.SSO.Dex != nil && len(cr.Spec.SSO.Dex.Config) > 0 {
		config = cr.Spec.SSO.Dex.Config
	}
	return config
}
