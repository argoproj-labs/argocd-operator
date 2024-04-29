package argoutil

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
)

func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{
			"error",
			"error",
		},
		{
			"warn",
			"warn",
		},
		{
			"info",
			"info",
		},
		{
			"debug",
			"debug",
		},
		{
			"default",
			common.ArgoCDDefaultLogLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetLogLevel(tt.name)
			if got != tt.expected {
				t.Errorf("GetLogLevel() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetLogFormat(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{
			"text",
			"text",
		},
		{
			"json",
			"json",
		},
		{
			"default",
			common.ArgoCDDefaultLogFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetLogFormat(tt.name)
			if got != tt.expected {
				t.Errorf("GetLogLevel() = %v, want %v", got, tt.expected)
			}
		})
	}
}
