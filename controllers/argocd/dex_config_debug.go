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
	dexServerTokenExpirySecsEnv := os.Getenv(EnvKeyDexServerTokenExpirySecs)
	var err error
	var duration, threshold int64
	log.Info(fmt.Sprintf("dex config: using debug settings with env override %s:%s", EnvKeyDexServerTokenExpirySecs, dexServerTokenExpirySecsEnv))
	if dexServerTokenExpirySecsEnv != "" {
		duration, err = strconv.ParseInt(dexServerTokenExpirySecsEnv, 10, 64)
		if err != nil {
			log.Error(err, "failed to parse DEX_SERVER_TOKEN_EXPIRY_SECS as duration")
			duration = common.ArgoCDDexServerTokenExpirySecs
		}

	}
	dexServerTokenRenewalThresholdEnv := os.Getenv(EnvKeyDexServerTokenRenewalThreshold)
	log.Info(fmt.Sprintf("dex config: using debug settings with env override %s:%s", EnvKeyDexServerTokenRenewalThreshold, dexServerTokenRenewalThresholdEnv))
	if dexServerTokenRenewalThresholdEnv != "" {
		threshold, err = strconv.ParseInt(dexServerTokenRenewalThresholdEnv, 10, 64)
		if err != nil {
			log.Error(err, "failed to parse DEX_SERVER_TOKEN_RENEWAL_THRESHOLD as duration")
			threshold = common.ArgoCDDexServerTokenRenewalThresholdPercent
		}
	}
	log.Info(fmt.Sprintf("dex config: using debug settings with env override %v:%v", duration, threshold))
	return time.Duration(duration*threshold/100) * time.Second
}
