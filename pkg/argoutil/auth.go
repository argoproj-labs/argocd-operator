package argoutil

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/sethvargo/go-password/password"
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
