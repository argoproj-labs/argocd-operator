package argoutil

import (
	"reflect"
	"testing"

	configv1 "github.com/openshift/api/config/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

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
		cfg      *argoproj.ArgoCDTLSConfig
		args     map[string]string
		expected map[string]string
		wantErr  bool
	}{
		{
			name: "valid config",
			cfg: &argoproj.ArgoCDTLSConfig{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildArgoCDAgentTLSArgs(tt.cfg, tt.args)
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
		cfg      *argoproj.ArgoCDTLSConfig
		expected []string
	}{
		{
			name:     "nil config",
			cfg:      nil,
			expected: nil,
		},
		{
			name: "empty min max",
			cfg:  &argoproj.ArgoCDTLSConfig{},
		},
		{
			name: "min and max",
			cfg: &argoproj.ArgoCDTLSConfig{
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
			cfg: &argoproj.ArgoCDTLSConfig{
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
