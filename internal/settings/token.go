// Package settings provides local account types.
// Token struct copied from github.com/argoproj/argo-cd/v3/util/settings to remove the dependency.
package settings

// Token holds the information about an API token for a local user account.
type Token struct {
	ID        string `json:"id"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp,omitempty"`
}
