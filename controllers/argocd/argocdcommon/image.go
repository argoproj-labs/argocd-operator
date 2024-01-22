package argocdcommon

import (
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

// GetContainerImage is a general purpose function to retrieve the img and tag to be deployed for a given component. First priority is given to the CR spec field, which is calculated from the supplied instance and function. If CR spec does not specify an image, 2nd priority is given to an environment variable. If no env var is specified, or specified env var is not set, specified default img and tag values are returned
func GetContainerImage(f func(cr *argoproj.ArgoCD) (string, string), cr *argoproj.ArgoCD, envVar, defaultImg, defaultTag string) string {
	img, tag := "", ""

	// set values defined in CR
	img, tag = f(cr)

	if img == "" && tag == "" {
		if envVar != "" {
			// return image set in env var
			if _, val := util.CaseInsensitiveGetenv(envVar); val != "" {
				return val
			}
		}
	}

	// return defaults if still unset
	if img == "" {
		img = defaultImg
	}

	if tag == "" {
		tag = defaultTag
	}

	return util.CombineImageTag(img, tag)
}

// GetArgoContainerImage will return the container image for given Argo CD instance
func GetArgoContainerImage(cr *argoproj.ArgoCD) string {
	fn := func(cr *argoproj.ArgoCD) (string, string) {
		return cr.Spec.Image, cr.Spec.Version
	}
	return GetContainerImage(fn, cr, common.ArgoCDImageEnvVar, common.ArgoCDDefaultArgoImage, common.ArgoCDDefaultArgoVersion)
}
