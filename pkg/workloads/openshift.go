package workloads

import (
	util "github.com/argoproj-labs/argocd-operator/pkg/util"
	template "github.com/openshift/api/template/v1"
)

var (
	templateAPIFound = false
)

// IsTemplateAPIAvailable returns true if the template API is present.
func IsTemplateAPIAvailable() bool {
	return templateAPIFound
}

// VerifyTemplateAPI will verify that the template API is present.
func VerifyTemplateAPI() error {
	found, err := util.VerifyAPI(template.SchemeGroupVersion.Group, template.SchemeGroupVersion.Version)
	if err != nil {
		return err
	}
	templateAPIFound = found
	return nil
}

func SetTemplateAPIFound(val bool) {
	templateAPIFound = val
}
