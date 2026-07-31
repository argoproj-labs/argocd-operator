package argoutil

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/tlsprofile"
)

// TestGetRedisHAProxyConfigRenderedTLSValues verifies that TLS minVersion and ciphers
// are correctly rendered in the final HAProxy configuration template output.
func TestGetRedisHAProxyConfigRenderedTLSValues(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	t.Setenv("REDIS_CONFIG_PATH", filepath.Join(wd, "../../build", "redis"))
	tests := []struct {
		name                    string
		useTLS                  bool
		centralTLSConfigProfile tlsprofile.TLSConfigProfile
		expectedInOutput        []string
		notExpectedInOutput     []string
		validatePattern         *regexp.Regexp
	}{
		{
			name:   "TLS 1.2 with two cipher suites",
			useTLS: true,
			centralTLSConfigProfile: tlsprofile.TLSConfigProfile{
				MinVersion: configv1.VersionTLS12,
				Ciphers: []string{
					"ECDHE-RSA-AES256-GCM-SHA384",
					"ECDHE-RSA-AES128-GCM-SHA256",
				},
			},
			expectedInOutput: []string{
				"ssl-default-bind-options ssl-min-ver TLSv1.2",
				"ssl-default-server-options ssl-min-ver TLSv1.2",
				"ssl-default-bind-ciphers ECDHE-RSA-AES256-GCM-SHA384:ECDHE-RSA-AES128-GCM-SHA256",
				"ssl-default-server-ciphers ECDHE-RSA-AES256-GCM-SHA384:ECDHE-RSA-AES128-GCM-SHA256",
				"ssl-default-bind-ciphersuites ECDHE-RSA-AES256-GCM-SHA384:ECDHE-RSA-AES128-GCM-SHA256",
				"ssl-default-server-ciphersuites ECDHE-RSA-AES256-GCM-SHA384:ECDHE-RSA-AES128-GCM-SHA256",
			},
			validatePattern: regexp.MustCompile(`ssl-min-ver\s+TLSv1\.2`),
		},
		{
			name:   "TLS 1.3 with modern cipher suites",
			useTLS: true,
			centralTLSConfigProfile: tlsprofile.TLSConfigProfile{
				MinVersion: configv1.VersionTLS13,
				Ciphers: []string{
					"TLS_AES_256_GCM_SHA384",
					"TLS_CHACHA20_POLY1305_SHA256",
					"TLS_AES_128_GCM_SHA256",
				},
			},
			expectedInOutput: []string{
				"ssl-default-bind-options ssl-min-ver TLSv1.3",
				"ssl-default-server-options ssl-min-ver TLSv1.3",
				"ssl-default-bind-ciphersuites TLS_AES_256_GCM_SHA384:TLS_CHACHA20_POLY1305_SHA256:TLS_AES_128_GCM_SHA256",
				"ssl-default-server-ciphersuites TLS_AES_256_GCM_SHA384:TLS_CHACHA20_POLY1305_SHA256:TLS_AES_128_GCM_SHA256",
			},
			notExpectedInOutput: []string{
				"ssl-default-bind-ciphers ",
				"ssl-default-server-ciphers ",
			},
			validatePattern: regexp.MustCompile(`ssl-min-ver\s+TLSv1\.3`),
		},
		{
			name:   "TLS enabled with minimum version only",
			useTLS: true,
			centralTLSConfigProfile: tlsprofile.TLSConfigProfile{
				MinVersion: configv1.VersionTLS13,
			},
			expectedInOutput: []string{
				"ssl-default-bind-options ssl-min-ver TLSv1.3",
				"ssl-default-server-options ssl-min-ver TLSv1.3",
			},
			notExpectedInOutput: []string{
				"ssl-default-bind-ciphers ",
				"ssl-default-server-ciphers ",
				"ssl-default-bind-ciphersuites",
				"ssl-default-server-ciphersuites",
			},
		},
		{
			name:                    "TLS disabled",
			useTLS:                  false,
			centralTLSConfigProfile: tlsprofile.TLSConfigProfile{},
			notExpectedInOutput: []string{
				"ca-base",
				"ssl-default-bind-options",
				"ssl-default-server-options",
				"ssl-default-bind-ciphers",
				"ssl-default-server-ciphers",
				"ssl-default-bind-ciphersuites",
				"ssl-default-server-ciphersuites",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := &argoproj.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd",
					Namespace: "argocd",
				},
			}

			var capturedVars map[string]string
			original := loadTemplateFile
			defer func() {
				loadTemplateFile = original
			}()
			loadTemplateFile = func(path string, vars map[string]string) (string, error) {
				capturedVars = maps.Clone(vars)
				return original(path, vars)
			}
			result := GetRedisHAProxyConfig(cr, tt.useTLS, tt.centralTLSConfigProfile)
			t.Logf("Rendered HAProxy config:\n%s", result)
			for _, expected := range tt.expectedInOutput {
				assert.Contains(t, result, expected)
			}
			for _, unexpected := range tt.notExpectedInOutput {
				assert.NotContains(t, result, unexpected)
			}
			if tt.validatePattern != nil {
				assert.Regexp(t, tt.validatePattern, result)
			}
			if tt.useTLS {
				expectedVersion := TLSProtocolVersionString(tt.centralTLSConfigProfile.MinVersion)
				if expectedVersion != "" {
					assert.Equal(t, expectedVersion, capturedVars["TLSMinVersion"])
				}
				if len(tt.centralTLSConfigProfile.Ciphers) > 0 {
					assert.Equal(
						t,
						strings.Join(tt.centralTLSConfigProfile.Ciphers, ":"),
						capturedVars["TLSCiphers"],
					)
				} else {
					assert.Empty(t, capturedVars["TLSCiphers"])
				}
			}
		})
	}
}
