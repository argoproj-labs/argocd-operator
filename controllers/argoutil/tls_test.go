package argoutil

import (
	"crypto/tls"
	"reflect"
	"testing"

	configv1 "github.com/openshift/api/config/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

func TestBuildCipherSuiteMap(t *testing.T) {
	m := buildCipherSuiteMap()

	if m == nil {
		t.Fatal("expected non-nil map")
	}

	if len(m) == 0 {
		t.Fatal("expected non-empty map")
	}

	if _, ok := m["TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"]; !ok {
		t.Fatal("expected known cipher suite to exist")
	}
}

func TestValidateTLSConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *argoproj.ArgoCDTlsConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: false,
		},
		{
			name: "empty ciphers",
			cfg: &argoproj.ArgoCDTlsConfig{
				MinVersion: "1.2",
				MaxVersion: "1.3",
			},
			wantErr: false,
		},
		{
			name: "valid cipher",
			cfg: &argoproj.ArgoCDTlsConfig{
				MinVersion: "1.2",
				MaxVersion: "1.3",
				CipherSuites: []string{
					"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid cipher",
			cfg: &argoproj.ArgoCDTlsConfig{
				CipherSuites: []string{
					"INVALID_CIPHER",
				},
			},
			wantErr: true,
		},
		{
			name: "ignore empty cipher",
			cfg: &argoproj.ArgoCDTlsConfig{
				CipherSuites: []string{
					"",
					"   ",
				},
			},
			wantErr: false,
		},
		{
			name: "tls 1.3 skips compatibility check",
			cfg: &argoproj.ArgoCDTlsConfig{
				MinVersion: "1.3",
				MaxVersion: "1.3",
				CipherSuites: []string{
					"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTLSConfig(tt.cfg)

			if tt.wantErr && err == nil {
				t.Fatal("expected error but got nil")
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestIsCipherCompatible(t *testing.T) {
	tests := []struct {
		name       string
		cs         *tls.CipherSuite
		minVersion string
		maxVersion string
		expected   bool
	}{
		{
			name: "compatible cipher",
			cs: &tls.CipherSuite{
				SupportedVersions: []uint16{
					tls.VersionTLS12,
					tls.VersionTLS13,
				},
			},
			minVersion: "1.2",
			maxVersion: "1.3",
			expected:   true,
		},
		{
			name: "incompatible cipher",
			cs: &tls.CipherSuite{
				SupportedVersions: []uint16{
					tls.VersionTLS10,
				},
			},
			minVersion: "1.2",
			maxVersion: "1.3",
			expected:   false,
		},
		{
			name: "empty min max",
			cs: &tls.CipherSuite{
				SupportedVersions: []uint16{
					tls.VersionTLS12,
				},
			},
			minVersion: "",
			maxVersion: "",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCipherCompatible(tt.cs, tt.minVersion, tt.maxVersion)

			if got != tt.expected {
				t.Fatalf("expected %v got %v", tt.expected, got)
			}
		})
	}
}

func TestTLSVersionString(t *testing.T) {
	tests := []struct {
		version  uint16
		expected string
	}{
		{tls.VersionTLS10, "1.0"},
		{tls.VersionTLS11, "1.1"},
		{tls.VersionTLS12, "1.2"},
		{tls.VersionTLS13, "1.3"},
		{999, ""},
	}

	for _, tt := range tests {
		got := tlsVersionString(tt.version)

		if got != tt.expected {
			t.Fatalf("expected %s got %s", tt.expected, got)
		}
	}
}

func TestJoinCiphers(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "join values",
			input:    []string{"A", "B", "C"},
			expected: "A:B:C",
		},
		{
			name:     "empty",
			input:    []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JoinCiphers(tt.input)

			if got != tt.expected {
				t.Fatalf("expected %s got %s", tt.expected, got)
			}
		})
	}
}

func TestAgentJoinCiphers(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "join values",
			input:    []string{"A", "B", "C"},
			expected: "A,B,C",
		},
		{
			name:     "empty",
			input:    []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AgentJoinCiphers(tt.input)

			if got != tt.expected {
				t.Fatalf("expected %s got %s", tt.expected, got)
			}
		})
	}
}

func TestRedisTLSVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1.0", "TLSv1"},
		{"1.1", "TLSv1.1"},
		{"1.2", "TLSv1.2"},
		{"1.3", "TLSv1.3"},
	}

	for _, tt := range tests {
		got := RedisTLSVersion(tt.input)

		if got != tt.expected {
			t.Fatalf("expected %s got %s", tt.expected, got)
		}
	}
}

func TestAgentTLSVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"1.0", ""},
		{"1.1", "tls1.1"},
		{"1.2", "tls1.2"},
		{"1.3", "tls1.3"},
	}

	for _, tt := range tests {
		got := AgentTLSVersion(tt.input)

		if got != tt.expected {
			t.Fatalf("expected %s got %s", tt.expected, got)
		}
	}
}

func TestBuildArgoCDAgentTLSArgs(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *argoproj.ArgoCDTlsConfig
		args     map[string]string
		expected map[string]string
		wantErr  bool
	}{
		{
			name: "nil config",
			cfg:  nil,
			args: map[string]string{
				"existing": "value",
			},
			expected: map[string]string{
				"existing": "value",
			},
			wantErr: false,
		},
		{
			name: "valid config",
			cfg: &argoproj.ArgoCDTlsConfig{
				MinVersion: "1.2",
				MaxVersion: "1.3",
				CipherSuites: []string{
					"TLS_AES_128_GCM_SHA256",
				},
			},
			args: map[string]string{},
			expected: map[string]string{
				"--tlsminversion": "tls1.2",
				"--tlsmaxversion": "tls1.3",
				"--tlsciphers":    "TLS_AES_128_GCM_SHA256",
			},
			wantErr: false,
		},
		{
			name: "invalid cipher",
			cfg: &argoproj.ArgoCDTlsConfig{
				CipherSuites: []string{
					"INVALID",
				},
			},
			args:     map[string]string{},
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildArgoCDAgentTLSArgs(tt.cfg, tt.args)

			if tt.wantErr && err == nil {
				t.Fatal("expected error but got nil")
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !tt.wantErr && !reflect.DeepEqual(got, tt.expected) {
				t.Fatalf("expected %+v got %+v", tt.expected, got)
			}
		})
	}
}

func TestTLSProtocolVersionString(t *testing.T) {
	tests := []struct {
		input    configv1.TLSProtocolVersion
		expected string
	}{
		{configv1.VersionTLS10, "1.0"},
		{configv1.VersionTLS11, "1.1"},
		{configv1.VersionTLS12, "1.2"},
		{configv1.VersionTLS13, "1.3"},
		{configv1.TLSProtocolVersion("invalid"), ""},
	}

	for _, tt := range tests {
		got := TLSProtocolVersionString(tt.input)

		if got != tt.expected {
			t.Fatalf("expected %s got %s", tt.expected, got)
		}
	}
}

func TestRedisTLSProtocolVersionString(t *testing.T) {
	tests := []struct {
		input    configv1.TLSProtocolVersion
		expected string
	}{
		{configv1.VersionTLS10, "TLSv1"},
		{configv1.VersionTLS11, "TLSv1.1"},
		{configv1.VersionTLS12, "TLSv1.2"},
		{configv1.VersionTLS13, "TLSv1.3"},
		{configv1.TLSProtocolVersion("invalid"), ""},
	}

	for _, tt := range tests {
		got := RedisTLSProtocolVersionString(tt.input)

		if got != tt.expected {
			t.Fatalf("expected %s got %s", tt.expected, got)
		}
	}
}

func TestAgentTLSProtocolVersionString(t *testing.T) {
	tests := []struct {
		input    configv1.TLSProtocolVersion
		expected string
	}{
		{configv1.VersionTLS10, ""},
		{configv1.VersionTLS11, "tls1.1"},
		{configv1.VersionTLS12, "tls1.2"},
		{configv1.VersionTLS13, "tls1.3"},
	}

	for _, tt := range tests {
		got := AgentTLSProtocolVersionString(tt.input)

		if got != tt.expected {
			t.Fatalf("expected %s got %s", tt.expected, got)
		}
	}
}

func TestBuildRedisProtocols(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *argoproj.ArgoCDTlsConfig
		expected []string
	}{
		{
			name:     "nil config",
			cfg:      nil,
			expected: nil,
		},
		{
			name: "empty min max",
			cfg:  &argoproj.ArgoCDTlsConfig{},
		},
		{
			name: "min and max",
			cfg: &argoproj.ArgoCDTlsConfig{
				MinVersion: "1.1",
				MaxVersion: "1.3",
			},
			expected: []string{
				"TLSv1.1",
				"TLSv1.2",
				"TLSv1.3",
			},
		},
		{
			name: "only min",
			cfg: &argoproj.ArgoCDTlsConfig{
				MinVersion: "1.2",
			},
			expected: []string{
				"TLSv1.2",
				"TLSv1.3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildRedisProtocols(tt.cfg)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Fatalf("expected %+v got %+v", tt.expected, got)
			}
		})
	}
}

func TestMapCipherSuites(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name: "valid mappings",
			input: []string{
				"ECDHE-RSA-AES128-GCM-SHA256",
				"TLS_AES_128_GCM_SHA256",
			},
			expected: []string{
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				"TLS_AES_128_GCM_SHA256",
			},
		},
		{
			name: "invalid cipher",
			input: []string{
				"INVALID",
			},
			expected: []string{},
		},
		{
			name: "partial valid",
			input: []string{
				"INVALID",
				"ECDHE-RSA-AES256-GCM-SHA384",
			},
			expected: []string{
				"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapCipherSuites(tt.input)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Fatalf("expected %+v got %+v", tt.expected, got)
			}
		})
	}
}
