//go:build !debug

package argocd

import (
	"time"

	"github.com/argoproj-labs/argocd-operator/common"
)

// dexServerTokenRenewalThreshold is how much nominal lifetime may remain before we treat the Dex token
// as due for renewal (ExpirySecs * ArgoCDDexServerTokenRenewalThresholdPercent / 100).
func dexServerTokenRenewalThreshold() time.Duration {
	return time.Duration(common.ArgoCDDexServerTokenExpirySecs*common.ArgoCDDexServerTokenRenewalThresholdPercent/100) * time.Second
}

// getTokenExpirySeconds returns the default dex token expiry seconds (3600s)
func getTokenExpirySeconds() int64 {
	return common.ArgoCDDexServerTokenExpirySecs
}
