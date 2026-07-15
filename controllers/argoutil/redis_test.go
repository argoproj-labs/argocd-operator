package argoutil

import (
	"bytes"
	"maps"
	"regexp"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

// TestTLSVersionToHAProxy tests the TLS version mapping function
func TestTLSVersionToHAProxy(t *testing.T) {
	tests := []struct {
		name        string
		tlsVersion  string
		expected    string
		description string
	}{
		{
			name:        "VersionTLS10",
			tlsVersion:  "VersionTLS10",
			expected:    "1.0",
			description: "should map VersionTLS10 to 1.0",
		},
		{
			name:        "VersionTLS11",
			tlsVersion:  "VersionTLS11",
			expected:    "1.1",
			description: "should map VersionTLS11 to 1.1",
		},
		{
			name:        "VersionTLS12",
			tlsVersion:  "VersionTLS12",
			expected:    "1.2",
			description: "should map VersionTLS12 to 1.2",
		},
		{
			name:        "VersionTLS13",
			tlsVersion:  "VersionTLS13",
			expected:    "1.3",
			description: "should map VersionTLS13 to 1.3",
		},
		{
			name:        "EmptyString",
			tlsVersion:  "",
			expected:    "",
			description: "should return empty string for empty input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TLSVersionToHAProxy(tt.tlsVersion)
			assert.Equal(t, tt.expected, result, tt.description)
		})
	}
}

// TestGetRedisHAProxyConfigRenderedTLSValues verifies that TLS minVersion and ciphers
// are correctly rendered in the final HAProxy configuration template output
func TestGetRedisHAProxyConfigRenderedTLSValues(t *testing.T) {
	tests := []struct {
		name             string
		useTLS           bool
		tlsMinVersion    string
		tlsCiphers       []string
		expectedInOutput []string
		notExpectedInOut []string
		validatePattern  *regexp.Regexp
		description      string
	}{
		{
			name:          "TLS 1.2 with two cipher suites",
			useTLS:        true,
			tlsMinVersion: "VersionTLS12",
			tlsCiphers: []string{
				"ECDHE-RSA-AES256-GCM-SHA384",
				"ECDHE-RSA-AES128-GCM-SHA256",
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
			name:          "TLS 1.3 with modern ciphers",
			useTLS:        true,
			tlsMinVersion: "VersionTLS13",
			tlsCiphers: []string{
				"TLS_AES_256_GCM_SHA384",
				"TLS_CHACHA20_POLY1305_SHA256",
				"TLS_AES_128_GCM_SHA256",
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
			name:          "TLS enabled with min version only",
			useTLS:        true,
			tlsMinVersion: "VersionTLS13",
			tlsCiphers:    nil,
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
			name:             "TLS disabled - no TLS configuration",
			useTLS:           false,
			tlsMinVersion:    "",
			tlsCiphers:       nil,
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
		{
			name:          "Multiple ciphers are colon-separated",
			useTLS:        true,
			tlsMinVersion: "VersionTLS12",
			tlsCiphers: []string{
				"CIPHER1",
				"CIPHER2",
				"CIPHER3",
				"CIPHER4",
			},
			expectedInOutput: []string{
				"CIPHER1:CIPHER2:CIPHER3:CIPHER4",
			},
			notExpectedInOut: []string{
				"CIPHER1, CIPHER2",
				"CIPHER1;CIPHER2",
			},
			description: "Verify ciphers are joined with colon separator",
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
			result := GetRedisHAProxyConfig(cr, tt.useTLS, tt.tlsMinVersion, tt.tlsCiphers)

			// Validate captured variables exist as expected
			t.Logf("Test: %s\nCaptured vars: %+v\nRendered output:\n%s\n", tt.name, capturedVars, result)

			// Check expected values are in output
			for _, expected := range tt.expectedInOutput {
				if !strings.Contains(result, expected) {
					t.Errorf("expected %q to be in rendered output, but it wasn't\nOutput:\n%s", expected, result)
				}
			}

			// Check values that should NOT be in output
			for _, notExpected := range tt.notExpectedInOut {
				if strings.Contains(result, notExpected) {
					t.Errorf("did not expect %q to be in rendered output, but it was\nOutput:\n%s", notExpected, result)
				}
			}

			// Check regex pattern if provided
			if tt.validatePattern != nil {
				if !tt.validatePattern.MatchString(result) {
					t.Errorf("expected pattern %q to match rendered output\nOutput:\n%s", tt.validatePattern.String(), result)
				}
			}

			// Validate vars structure
			if tt.useTLS {
				if tlsVersion := capturedVars["tlsMinVersion"]; tt.tlsMinVersion != "" && TLSVersionToHAProxy(tt.tlsMinVersion) != "" {
					if tlsVersion != TLSVersionToHAProxy(tt.tlsMinVersion) {
						t.Errorf("tlsMinVersion var = %q, want %q", tlsVersion, TLSVersionToHAProxy(tt.tlsMinVersion))
					}
				}

				if len(tt.tlsCiphers) > 0 {
					expectedCiphers := strings.Join(tt.tlsCiphers, ":")
					if ciphers := capturedVars["tlsCiphers"]; ciphers != expectedCiphers {
						t.Errorf("tlsCiphers var = %q, want %q", ciphers, expectedCiphers)
					}
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

{{- if .tlsMinVersion}}
    ssl-default-bind-options ssl-min-ver TLSv{{.tlsMinVersion}}
    ssl-default-server-options ssl-min-ver TLSv{{.tlsMinVersion}}
{{- end}}

{{- if .tlsCiphers}}
{{- if eq .tlsMinVersion "1.3"}}
    # TLS 1.3 cipher suites
    ssl-default-bind-ciphersuites {{.tlsCiphers}}
    ssl-default-server-ciphersuites {{.tlsCiphers}}
{{- else}}
    # TLS 1.2 and below cipher lists
    ssl-default-bind-ciphers {{.tlsCiphers}}
    ssl-default-server-ciphers {{.tlsCiphers}}

    # Also configure TLS 1.3 cipher suites when TLS 1.3 is negotiated
    ssl-default-bind-ciphersuites {{.tlsCiphers}}
    ssl-default-server-ciphersuites {{.tlsCiphers}}
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
