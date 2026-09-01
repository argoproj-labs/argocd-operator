//go:build debug

package argocd

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/argoproj-labs/argocd-operator/common"
)

const (
	EnvKeyDexServerTokenExpirySecs       = "ARGOCD_DEX_SERVER_TOKEN_EXPIRY_SECONDS"
	EnvKeyDexServerTokenRenewalThreshold = "ARGOCD_DEX_SERVER_TOKEN_RENEWAL_THRESHOLD"
)

// dexServerTokenRenewalThreshold is how much nominal lifetime may remain before we treat the Dex token
// values can be overridden with environment variables when debug build tag is enabled.
func dexServerTokenRenewalThreshold() time.Duration {
	dexServerTokenRenewalThresholdEnv := os.Getenv(EnvKeyDexServerTokenRenewalThreshold)
	var threshold int64
	var err error
	log.Info(fmt.Sprintf("dex config: using debug settings with env override %s:%s", EnvKeyDexServerTokenRenewalThreshold, dexServerTokenRenewalThresholdEnv))
	if dexServerTokenRenewalThresholdEnv != "" {
		threshold, err = strconv.ParseInt(dexServerTokenRenewalThresholdEnv, 10, 64)
		if err != nil {
			log.Error(err, "failed to parse DEX_SERVER_TOKEN_RENEWAL_THRESHOLD as duration")
			threshold = common.ArgoCDDexServerTokenRenewalThresholdPercent
		}
	}
	return time.Duration(getTokenExpirySeconds()*threshold/100) * time.Second
}

// getTokenExpirySeconds returns the token expiry seconds from env variable if set or else returns the default value 3600s
func getTokenExpirySeconds() int64 {
	dexServerTokenExpirySecsEnv := os.Getenv(EnvKeyDexServerTokenExpirySecs)
	log.Info(fmt.Sprintf("dex config: using debug settings with env override %s:%s", EnvKeyDexServerTokenExpirySecs, dexServerTokenExpirySecsEnv))
	if dexServerTokenExpirySecsEnv != "" {
		tokenExpirySeconds, err := strconv.ParseInt(dexServerTokenExpirySecsEnv, 10, 64)
		if err == nil {
			return tokenExpirySeconds
		}
		log.Error(err, "failed to parse DEX_SERVER_TOKEN_EXPIRY_SECS as duration")
	}
	return common.ArgoCDDexServerTokenExpirySecs
}
