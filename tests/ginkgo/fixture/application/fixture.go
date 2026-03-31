package application

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	matcher "github.com/onsi/gomega/types"

	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
)

// AppRef is a lightweight reference to an Argo CD Application.
type AppRef struct {
	Name      string
	Namespace string
	session   *argocdFixture.Session // for get/delete operations
}

// appConfig holds configuration for creating an Application via CLI.
type appConfig struct {
	args            []string
	annotations     map[string]string
	labels          map[string]string
	managedNSLabels map[string]string
	session         *argocdFixture.Session
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

// WithSession sets the ArgoCD session for CLI authentication.
func WithSession(s *argocdFixture.Session) AppOption {
	return func(c *appConfig) { c.session = s }
}

// Create creates an Argo CD Application using the argocd CLI in login mode.
func Create(name, namespace string, opts ...AppOption) *AppRef {
	cfg := &appConfig{}
	for _, o := range opts {
		o(cfg)
	}
	Expect(cfg.session).ToNot(BeNil(), "WithSession is required for application.Create")

	args := append([]string{"app", "create", name, "--app-namespace", namespace}, cfg.args...)

	output, err := runArgoCDCLI(cfg.session, args...)
	Expect(err).ToNot(HaveOccurred(), "argocd app create failed: %s", output)

	ref := &AppRef{Name: name, Namespace: namespace, session: cfg.session}

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
		patch := map[string]any{
			"spec": map[string]any{
				"syncPolicy": map[string]any{
					"managedNamespaceMetadata": map[string]any{
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
	Expect(ref.session).ToNot(BeNil(), "session is required for application.Delete")
	output, err := runArgoCDCLI(ref.session, "app", "delete", ref.Name, "--app-namespace", ref.Namespace, "--yes")
	Expect(err).ToNot(HaveOccurred(), "argocd app delete failed: %s", output)
}

// Ref creates a reference to an existing Application without creating it.
// Session is optional — when nil, kubectl is used for get operations (matchers).
func Ref(name, namespace string, sessions ...*argocdFixture.Session) *AppRef {
	var session *argocdFixture.Session
	if len(sessions) > 0 {
		session = sessions[0]
	}
	return &AppRef{Name: name, Namespace: namespace, session: session}
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
		status, ok := data["status"].(map[string]any)
		if !ok {
			return true
		}
		conditions, ok := status["conditions"].([]any)
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
		status, ok := data["status"].(map[string]any)
		if !ok {
			return false
		}
		conditions, ok := status["conditions"].([]any)
		if !ok {
			return false
		}
		var found []string
		for _, c := range conditions {
			cond, ok := c.(map[string]any)
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

func getAppJSON(ref *AppRef) (map[string]any, error) {
	var output string
	var err error

	if ref.session != nil {
		output, err = runArgoCDCLI(ref.session, "app", "get", ref.Name, "--app-namespace", ref.Namespace, "-o", "json")
		if err != nil {
			return nil, fmt.Errorf("argocd app get failed: %v, output: %s", err, output)
		}
	} else {
		// No session — use kubectl directly (for Ref-only usage without CLI login)
		output, err = runKubectl("get", "application.argoproj.io", ref.Name, "-n", ref.Namespace, "-o", "json")
		if err != nil {
			return nil, fmt.Errorf("kubectl get application failed: %v, output: %s", err, output)
		}
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func jsonGetString(data map[string]any, keys ...string) string {
	current := any(data)
	for _, key := range keys {
		m, ok := current.(map[string]any)
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

func runArgoCDCLI(session *argocdFixture.Session, args ...string) (string, error) {
	allArgs := append([]string{"--server", session.Server, "--auth-token", session.AuthToken, "--insecure"}, args...)
	GinkgoWriter.Println("executing argocd", allArgs)
	// #nosec G204 -- test code
	cmd := exec.Command("argocd", allArgs...)
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
