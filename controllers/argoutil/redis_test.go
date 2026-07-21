package argoutil

import (
	"bytes"
	"maps"
	"regexp"
	"strings"
	"testing"
	"text/template"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"

	"github.com/argoproj-labs/argocd-operator/pkg/tlsprofile"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

// TestGetRedisHAProxyConfigRenderedTLSValues verifies that TLS minVersion and ciphers
// are correctly rendered in the final HAProxy configuration template output
func TestGetRedisHAProxyConfigRenderedTLSValues(t *testing.T) {
	tests := []struct {
		name                    string
		useTLS                  bool
		centralTLSConfigProfile tlsprofile.TLSConfigProfile
		expectedInOutput        []string
		notExpectedInOut        []string
		validatePattern         *regexp.Regexp
		description             string
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
			notExpectedInOut: []string{},
			validatePattern:  regexp.MustCompile(`ssl-min-ver\s+TLSv1\.2`),
			description:      "Verify TLS 1.2 is rendered with proper HAProxy syntax",
		},
		{
			name:   "TLS 1.3 with modern ciphers",
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
			notExpectedInOut: []string{
				"1.2",
				"VersionTLS13",
				"ssl-default-bind-ciphers ",
				"ssl-default-server-ciphers ",
			},
			validatePattern: regexp.MustCompile(`ssl-min-ver\s+TLSv1\.3`),
			description:     "Verify TLS 1.3 is rendered correctly",
		},
		{
			name:   "TLS enabled with min version only",
			useTLS: true,
			centralTLSConfigProfile: tlsprofile.TLSConfigProfile{
				MinVersion: configv1.VersionTLS13,
				Ciphers:    nil,
			},
			expectedInOutput: []string{
				"ssl-default-bind-options ssl-min-ver TLSv1.3",
				"ssl-default-server-options ssl-min-ver TLSv1.3",
			},
			notExpectedInOut: []string{
				"ssl-default-bind-ciphers",
				"ssl-default-bind-ciphersuites",
			},
		},
		{
			name:   "TLS disabled - no TLS configuration",
			useTLS: false,
			centralTLSConfigProfile: tlsprofile.TLSConfigProfile{
				MinVersion: "",
				Ciphers:    nil,
			},
			expectedInOutput: []string{},
			notExpectedInOut: []string{
				"ca-base",
				"ssl-default-bind-options",
				"ssl-default-server-options",
				"ssl-default-bind-ciphers",
				"ssl-default-server-ciphers",
				"ssl-default-bind-ciphersuites",
				"ssl-default-server-ciphersuites",
			},
			description: "Verify no TLS directives when TLS is disabled",
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
			// Capture the vars passed to the template
			var capturedVars map[string]string
			original := loadTemplateFile
			defer func() {
				loadTemplateFile = original
			}()
			loadTemplateFile = func(path string, vars map[string]string) (string, error) {
				capturedVars = maps.Clone(vars)
				return renderMockTemplate(vars), nil
			}
			// Call the function
			result := GetRedisHAProxyConfig(cr, tt.useTLS, tt.centralTLSConfigProfile)
			// Validate captured variables exist as expected
			t.Logf("Test: %s\nCaptured vars: %+v\nRendered output:\n%s\n", tt.name, capturedVars, result)
			// Check expected values are in output
			for _, expected := range tt.expectedInOutput {
				assert.Contains(t, result, expected)
			}
			// Check values that should NOT be in output
			for _, notExpected := range tt.notExpectedInOut {
				assert.NotContains(t, result, notExpected)
			}
			// Check regex pattern if provided
			if tt.validatePattern != nil {
				assert.Regexp(t, tt.validatePattern.String(), result)
			}
			// Validate vars structure
			if tt.useTLS {
				if tlsVersion := capturedVars["TLSMinVersion"]; tt.centralTLSConfigProfile.MinVersion != "" && TLSProtocolVersionString(tt.centralTLSConfigProfile.MinVersion) != "" {
					if tlsVersion != TLSProtocolVersionString(tt.centralTLSConfigProfile.MinVersion) {
						t.Errorf("TLSMinVersion var = %q, want %q", tlsVersion, TLSProtocolVersionString(tt.centralTLSConfigProfile.MinVersion))
					}
				}
				if len(tt.centralTLSConfigProfile.Ciphers) > 0 {
					expectedCiphers := strings.Join(tt.centralTLSConfigProfile.Ciphers, ":")
					assert.Equal(t, expectedCiphers, capturedVars["TLSCiphers"])
				}
			}
		})
	}
}

// renderMockTemplate renders the HAProxy TLS template fragment
// used by haproxy.cfg.tpl for unit testing.
func renderMockTemplate(vars map[string]string) string {
	const tpl = `
{{- if eq .UseTLS "true"}}
global
    ca-base /app/config/redis/tls

{{- if .TLSMinVersion}}
    ssl-default-bind-options ssl-min-ver TLSv{{.TLSMinVersion}}
    ssl-default-server-options ssl-min-ver TLSv{{.TLSMinVersion}}
{{- end}}

{{- if .TLSCiphers}}
{{- if eq .TLSMinVersion "1.3"}}
    # TLS 1.3 cipher suites
    ssl-default-bind-ciphersuites {{.TLSCiphers}}
    ssl-default-server-ciphersuites {{.TLSCiphers}}
{{- else}}
    # TLS 1.2 and below cipher lists
    ssl-default-bind-ciphers {{.TLSCiphers}}
    ssl-default-server-ciphers {{.TLSCiphers}}

    # Also configure TLS 1.3 cipher suites when TLS 1.3 is negotiated
    ssl-default-bind-ciphersuites {{.TLSCiphers}}
    ssl-default-server-ciphersuites {{.TLSCiphers}}
{{- end}}
{{- end}}
{{- end}}
`

	t, err := template.New("haproxy").Parse(tpl)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		panic(err)
	}

	return buf.String()
}
