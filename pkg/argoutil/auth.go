package argoutil

import (
	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"

	argopass "github.com/argoproj/argo-cd/v2/util/password"

	"github.com/argoproj-labs/argocd-operator/common"
)

// GenerateArgoAdminPassword will generate and return the admin password for Argo CD.
func GenerateArgoAdminPassword() ([]byte, error) {
	pass, err := password.Generate(
		common.ArgoCDDefaultAdminPasswordLength,
		common.ArgoCDDefaultAdminPasswordNumDigits,
		common.ArgoCDDefaultAdminPasswordNumSymbols,
		false, false)

	return []byte(pass), err
}

// GenerateArgoServerSessionKey will generate and return the server signature key for session validation.
func GenerateArgoServerSessionKey() ([]byte, error) {
	pass, err := password.Generate(
		common.ArgoCDDefaultServerSessionKeyLength,
		common.ArgoCDDefaultServerSessionKeyNumDigits,
		common.ArgoCDDefaultServerSessionKeyNumSymbols,
		false, false)

	return []byte(pass), err
}

// HasArgoAdminPasswordChanged will return true if the Argo admin password has changed.
func HasArgoAdminPasswordChanged(actual *corev1.Secret, expected *corev1.Secret) bool {
	actualPwd := string(actual.Data[common.ArgoCDKeyAdminPassword])
	expectedPwd := string(expected.Data[common.ArgoCDKeyAdminPassword])

	validPwd, _ := argopass.VerifyPassword(expectedPwd, actualPwd)
	return !validPwd
}
