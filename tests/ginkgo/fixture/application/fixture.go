package application

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	matcher "github.com/onsi/gomega/types"
)

// AppRef is a lightweight reference to an Argo CD Application.
type AppRef struct {
	Name      string
	Namespace string
}

// appConfig holds configuration for creating an Application via CLI.
type appConfig struct {
	args            []string
	annotations     map[string]string
	labels          map[string]string
	managedNSLabels map[string]string
}

// AppOption configures Application creation.
type AppOption func(*appConfig)

func WithRepo(repo string) AppOption {
	return func(c *appConfig) { c.args = append(c.args, "--repo", repo) }
}

func WithPath(path string) AppOption {
	return func(c *appConfig) { c.args = append(c.args, "--path", path) }
}

func WithRevision(rev string) AppOption {
	return func(c *appConfig) { c.args = append(c.args, "--revision", rev) }
}

func WithHelmChart(chart string) AppOption {
	return func(c *appConfig) { c.args = append(c.args, "--helm-chart", chart) }
}

func WithHelmValues(values string) AppOption {
	return func(c *appConfig) {
		if values != "" {
			c.args = append(c.args, "--values", values)
		}
	}
}

func WithDestServer(server string) AppOption {
	return func(c *appConfig) { c.args = append(c.args, "--dest-server", server) }
}

func WithDestNamespace(ns string) AppOption {
	return func(c *appConfig) { c.args = append(c.args, "--dest-namespace", ns) }
}

func WithDestName(name string) AppOption {
	return func(c *appConfig) { c.args = append(c.args, "--dest-name", name) }
}

func WithProject(project string) AppOption {
	return func(c *appConfig) { c.args = append(c.args, "--project", project) }
}

func WithAutoSync() AppOption {
	return func(c *appConfig) { c.args = append(c.args, "--sync-policy", "automated") }
}

func WithPrune() AppOption {
	return func(c *appConfig) { c.args = append(c.args, "--auto-prune") }
}

func WithSelfHeal() AppOption {
	return func(c *appConfig) { c.args = append(c.args, "--self-heal") }
}

func WithRetryLimit(limit int) AppOption {
	return func(c *appConfig) { c.args = append(c.args, "--sync-retry-limit", fmt.Sprintf("%d", limit)) }
}

func WithSyncOption(option string) AppOption {
	return func(c *appConfig) { c.args = append(c.args, "--sync-option", option) }
}

func WithPlugin(name string) AppOption {
	return func(c *appConfig) { c.args = append(c.args, "--config-management-plugin", name) }
}

func WithPluginEnv(key, value string) AppOption {
	return func(c *appConfig) { c.args = append(c.args, "--plugin-env", fmt.Sprintf("%s=%s", key, value)) }
}

func WithDirectoryRecurse() AppOption {
	return func(c *appConfig) { c.args = append(c.args, "--directory-recurse") }
}

func WithAnnotation(key, value string) AppOption {
	return func(c *appConfig) {
		if c.annotations == nil {
			c.annotations = make(map[string]string)
		}
		c.annotations[key] = value
	}
}

func WithLabel(key, value string) AppOption {
	return func(c *appConfig) {
		if c.labels == nil {
			c.labels = make(map[string]string)
		}
		c.labels[key] = value
	}
}

func WithManagedNSLabels(labels map[string]string) AppOption {
	return func(c *appConfig) { c.managedNSLabels = labels }
}

// Create creates an Argo CD Application using the argocd CLI.
func Create(name, namespace string, opts ...AppOption) *AppRef {
	cfg := &appConfig{}
	for _, o := range opts {
		o(cfg)
	}

	args := append([]string{"app", "create", name, "--core", "-N", namespace}, cfg.args...)

	output, err := runArgoCDCLI(args...)
	Expect(err).ToNot(HaveOccurred(), "argocd app create failed: %s", output)

	ref := &AppRef{Name: name, Namespace: namespace}

	// Post-create: annotations via kubectl
	for k, v := range cfg.annotations {
		out, err := runKubectl("annotate", "application.argoproj.io", name, "-n", namespace,
			fmt.Sprintf("%s=%s", k, v))
		Expect(err).ToNot(HaveOccurred(), "kubectl annotate failed: %s", out)
	}

	// Post-create: labels via kubectl
	for k, v := range cfg.labels {
		out, err := runKubectl("label", "application.argoproj.io", name, "-n", namespace,
			fmt.Sprintf("%s=%s", k, v))
		Expect(err).ToNot(HaveOccurred(), "kubectl label failed: %s", out)
	}

	// Post-create: managed namespace metadata labels via kubectl patch
	if len(cfg.managedNSLabels) > 0 {
		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"syncPolicy": map[string]interface{}{
					"managedNamespaceMetadata": map[string]interface{}{
						"labels": cfg.managedNSLabels,
					},
				},
			},
		}
		patchBytes, _ := json.Marshal(patch)
		out, err := runKubectl("patch", "application.argoproj.io", name, "-n", namespace,
			"--type=merge", "-p", string(patchBytes))
		Expect(err).ToNot(HaveOccurred(), "kubectl patch failed: %s", out)
	}

	return ref
}

// Delete deletes an Argo CD Application.
func Delete(ref *AppRef) {
	output, err := runArgoCDCLI("app", "delete", ref.Name, "--core", "-N", ref.Namespace, "--yes")
	Expect(err).ToNot(HaveOccurred(), "argocd app delete failed: %s", output)
}

// Ref creates a reference to an existing Application without creating it.
func Ref(name, namespace string) *AppRef {
	return &AppRef{Name: name, Namespace: namespace}
}

// --- Matchers ---

// HaveHealthStatus checks that the Application has the expected health status.
func HaveHealthStatus(expected string) matcher.GomegaMatcher {
	return WithTransform(func(ref *AppRef) bool {
		data, err := getAppJSON(ref)
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}
		current := jsonGetString(data, "status", "health", "status")
		GinkgoWriter.Printf("HaveHealthStatus - current: %s / expected: %s\n", current, expected)
		return current == expected
	}, BeTrue())
}

// HaveSyncStatus checks that the Application has the expected sync status.
func HaveSyncStatus(expected string) matcher.GomegaMatcher {
	return WithTransform(func(ref *AppRef) bool {
		data, err := getAppJSON(ref)
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}
		current := jsonGetString(data, "status", "sync", "status")
		GinkgoWriter.Printf("HaveSyncStatus - current: %s / expected: %s\n", current, expected)
		return current == expected
	}, BeTrue())
}

// HaveOperationPhase checks that the Application has the expected operation phase.
func HaveOperationPhase(expected string) matcher.GomegaMatcher {
	return WithTransform(func(ref *AppRef) bool {
		data, err := getAppJSON(ref)
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}
		current := jsonGetString(data, "status", "operationState", "phase")
		GinkgoWriter.Printf("HaveOperationPhase - current: %s / expected: %s\n", current, expected)
		return current == expected
	}, BeTrue())
}

// HaveNoConditions checks that the Application has no conditions.
func HaveNoConditions() matcher.GomegaMatcher {
	return WithTransform(func(ref *AppRef) bool {
		data, err := getAppJSON(ref)
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}
		status, ok := data["status"].(map[string]interface{})
		if !ok {
			return true
		}
		conditions, ok := status["conditions"].([]interface{})
		if !ok || len(conditions) == 0 {
			return true
		}
		GinkgoWriter.Printf("HaveNoConditions - have: %+v\n", conditions)
		return false
	}, BeTrue())
}

// HaveConditionMatching checks that the Application has a condition matching the type and message pattern.
func HaveConditionMatching(conditionType string, messagePattern string) matcher.GomegaMatcher {
	pattern := regexp.MustCompile(messagePattern)
	return WithTransform(func(ref *AppRef) bool {
		data, err := getAppJSON(ref)
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}
		status, ok := data["status"].(map[string]interface{})
		if !ok {
			return false
		}
		conditions, ok := status["conditions"].([]interface{})
		if !ok {
			return false
		}
		var found []string
		for _, c := range conditions {
			cond, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			ct, _ := cond["type"].(string)
			msg, _ := cond["message"].(string)
			found = append(found, fmt.Sprintf("  -  %s/%s", ct, msg))
			if ct == conditionType && pattern.MatchString(msg) {
				return true
			}
		}
		GinkgoWriter.Printf("HaveConditionMatching - expected: %s/%s; current(%d):\n", conditionType, messagePattern, len(conditions))
		for _, f := range found {
			GinkgoWriter.Println(f)
		}
		return false
	}, BeTrue())
}

// GetOperationMessage retrieves the operation state message for an Application.
func GetOperationMessage(ref *AppRef) (string, error) {
	data, err := getAppJSON(ref)
	if err != nil {
		return "", err
	}
	return jsonGetString(data, "status", "operationState", "message"), nil
}

// --- Internal helpers ---

func getAppJSON(ref *AppRef) (map[string]interface{}, error) {
	output, err := runArgoCDCLI("app", "get", ref.Name, "--core", "-N", ref.Namespace, "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("argocd app get failed: %v, output: %s", err, output)
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func jsonGetString(data map[string]interface{}, keys ...string) string {
	current := interface{}(data)
	for _, key := range keys {
		m, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current, ok = m[key]
		if !ok {
			return ""
		}
	}
	s, ok := current.(string)
	if !ok {
		return ""
	}
	return s
}

func runArgoCDCLI(args ...string) (string, error) {
	// In core mode, the CLI needs --server-namespace to know where the ArgoCD
	// installation lives (e.g. argocd-cm ConfigMap). Extract it from the -N flag
	// since tests install ArgoCD in the same namespace as the applications.
	for i, arg := range args {
		if arg == "-N" && i+1 < len(args) {
			args = append(args, "--server-namespace", args[i+1])
			break
		}
	}

	GinkgoWriter.Println("executing argocd", args)
	// #nosec G204 -- test code
	cmd := exec.Command("argocd", args...)
	// Also set ARGOCD_NAMESPACE as a fallback in case --server-namespace is not supported.
	for i, arg := range args {
		if arg == "-N" && i+1 < len(args) {
			cmd.Env = append(cmd.Environ(), "ARGOCD_NAMESPACE="+args[i+1])
			break
		}
	}
	output, err := cmd.CombinedOutput()
	GinkgoWriter.Println(string(output))
	return string(output), err
}

func runKubectl(args ...string) (string, error) {
	GinkgoWriter.Println("executing kubectl", args)
	// #nosec G204 -- test code
	cmd := exec.Command("kubectl", args...)
	output, err := cmd.CombinedOutput()
	GinkgoWriter.Println(string(output))
	return string(output), err
}
