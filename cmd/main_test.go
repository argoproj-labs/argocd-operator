package main

import (
	"crypto/tls"
	"testing"
)

func TestBuildMetricsServerOptions_SecureDisabled(t *testing.T) {
	opts := buildMetricsServerOptions(":8080", false, nil)

	if opts.SecureServing {
		t.Error("expected SecureServing to be false")
	}
	if opts.BindAddress != ":8080" {
		t.Errorf("expected BindAddress :8080, got %s", opts.BindAddress)
	}
	if opts.FilterProvider != nil {
		t.Error("expected FilterProvider to be nil when secureMetrics is false")
	}
}

func TestBuildMetricsServerOptions_SecureEnabled(t *testing.T) {
	opts := buildMetricsServerOptions(":8443", true, nil)

	if !opts.SecureServing {
		t.Error("expected SecureServing to be true")
	}
	if opts.BindAddress != ":8443" {
		t.Errorf("expected BindAddress :8443, got %s", opts.BindAddress)
	}
	if opts.FilterProvider == nil {
		t.Error("expected FilterProvider to be set when secureMetrics is true")
	}
}

func TestBuildMetricsServerOptions_TLSOptsPassedThrough(t *testing.T) {
	called := false
	tlsOpt := func(c *tls.Config) { called = true }

	opts := buildMetricsServerOptions(":8080", false, []func(*tls.Config){tlsOpt})

	if len(opts.TLSOpts) != 1 {
		t.Fatalf("expected 1 TLS option, got %d", len(opts.TLSOpts))
	}
	opts.TLSOpts[0](&tls.Config{})
	if !called {
		t.Error("expected TLS option to be called")
	}
}
