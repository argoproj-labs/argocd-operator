package argoutil

import (
	"reflect"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

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
