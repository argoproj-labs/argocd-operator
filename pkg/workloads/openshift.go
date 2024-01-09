package workloads

import (
	templatev1 "github.com/openshift/api/template/v1"

	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
)

var (
	templateAPIFound = false
)

// IsTemplateAPIAvailable returns true if the template API is present.
func IsTemplateAPIAvailable() bool {
	return templateAPIFound
}

// SetTemplateAPIFound sets the value of prometheusAPIFound to provided input
func SetTemplateAPIFound(found bool) {
	templateAPIFound = found
}

// VerifyTemplateAPI will verify that the template API is present.
func VerifyTemplateAPI() error {
	found, err := argoutil.VerifyAPI(templatev1.SchemeGroupVersion.Group, templatev1.SchemeGroupVersion.Version)
	if err != nil {
		return err
	}
	templateAPIFound = found
	return nil
}
