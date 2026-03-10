package main

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"strings"
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

func TestKustomizeConfig_NoKubeRBACProxyReferences(t *testing.T) {
	repoRoot := findRepoRoot(t)
	filesToCheck := []string{
		"config/default/manager_auth_proxy_patch.yaml",
		"config/default/kustomization.yaml",
		"config/manager/manager.yaml",
	}

	for _, relPath := range filesToCheck {
		fullPath := filepath.Join(repoRoot, relPath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("failed to read %s: %v", relPath, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			if strings.Contains(trimmed, "kube-rbac-proxy") {
				t.Errorf("%s:%d still references kube-rbac-proxy in non-comment line: %s", relPath, i+1, trimmed)
			}
		}
	}
}

func TestKustomizeConfig_PatchConfiguresSecureMetrics(t *testing.T) {
	repoRoot := findRepoRoot(t)
	patchPath := filepath.Join(repoRoot, "config/default/manager_auth_proxy_patch.yaml")

	data, err := os.ReadFile(patchPath)
	if err != nil {
		t.Fatalf("failed to read patch file: %v", err)
	}
	content := string(data)

	checks := map[string]string{
		"--metrics-secure":             "patch must enable --metrics-secure",
		"--metrics-bind-address=:8443": "patch must bind metrics to port 8443",
		"containerPort: 8443":          "patch must expose container port 8443",
	}
	for needle, msg := range checks {
		if !strings.Contains(content, needle) {
			t.Errorf("%s (missing %q)", msg, needle)
		}
	}
}

func TestKustomizeConfig_PatchAlsoConfiguresMetricsServiceHTTPSPort(t *testing.T) {
	repoRoot := findRepoRoot(t)
	patchPath := filepath.Join(repoRoot, "config/default/manager_auth_proxy_patch.yaml")

	data, err := os.ReadFile(patchPath)
	if err != nil {
		t.Fatalf("failed to read patch file: %v", err)
	}
	content := string(data)

	checks := map[string]string{
		"kind: Service": "patch must include a Service patch",
		"name: controller-manager-metrics-service": "patch must target controller manager metrics service",
		"targetPort: https":                        "patch must switch metrics service to target the https named port",
	}
	for needle, msg := range checks {
		if !strings.Contains(content, needle) {
			t.Errorf("%s (missing %q)", msg, needle)
		}
	}
}

func TestKustomizeConfig_BaseMetricsServiceTargets8080(t *testing.T) {
	repoRoot := findRepoRoot(t)
	servicePath := filepath.Join(repoRoot, "config/rbac/auth_proxy_service.yaml")

	data, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("failed to read auth proxy service file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "targetPort: 8080") {
		t.Error("base auth proxy service must target port 8080 so manager_auth_proxy_patch.yaml can act as a consolidated toggle")
	}
}

func TestKustomizeConfig_PatchIsEnabled(t *testing.T) {
	repoRoot := findRepoRoot(t)
	kustomizePath := filepath.Join(repoRoot, "config/default/kustomization.yaml")

	data, err := os.ReadFile(kustomizePath)
	if err != nil {
		t.Fatalf("failed to read kustomization.yaml: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "manager_auth_proxy_patch.yaml") {
			if strings.HasPrefix(trimmed, "#") {
				t.Error("manager_auth_proxy_patch.yaml is commented out in kustomization.yaml; it must be enabled")
			}
			return
		}
	}
	t.Error("manager_auth_proxy_patch.yaml not found in kustomization.yaml")
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	// Start from the test file's directory (cmd/) and go up one level.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	// Walk up until we find go.mod (repo root).
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (go.mod)")
		}
		dir = parent
	}
}
