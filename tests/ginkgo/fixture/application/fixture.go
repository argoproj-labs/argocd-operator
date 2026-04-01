package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	matcher "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var applicationGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "applications",
}

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

func WithSkipValidation() AppOption {
	return func(c *appConfig) { c.args = append(c.args, "--validate=false") }
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

// Create creates an Argo CD Application using the argocd CLI.
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

	// Post-create: apply annotations, labels, and managed namespace metadata via k8s client
	if len(cfg.annotations) > 0 || len(cfg.labels) > 0 || len(cfg.managedNSLabels) > 0 {
		patchApp(name, namespace, cfg.annotations, cfg.labels, cfg.managedNSLabels)
	}

	return ref
}

// patchApp applies annotations, labels, and managed namespace metadata to an Application using the k8s client.
func patchApp(name, namespace string, annotations, labels map[string]string, managedNSLabels map[string]string) {
	k8sClient, _ := fixtureUtils.GetE2ETestKubeClient()
	ctx := context.Background()

	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(applicationGVR.GroupVersion().WithKind("Application"))
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, app)).To(Succeed(), "failed to get Application %s/%s", namespace, name)

	if len(annotations) > 0 {
		existing := app.GetAnnotations()
		if existing == nil {
			existing = make(map[string]string)
		}
		for k, v := range annotations {
			existing[k] = v
		}
		app.SetAnnotations(existing)
	}

	if len(labels) > 0 {
		existing := app.GetLabels()
		if existing == nil {
			existing = make(map[string]string)
		}
		for k, v := range labels {
			existing[k] = v
		}
		app.SetLabels(existing)
	}

	if len(managedNSLabels) > 0 {
		spec, _, _ := unstructured.NestedMap(app.Object, "spec")
		if spec == nil {
			spec = map[string]interface{}{}
		}
		labelsMap := make(map[string]interface{}, len(managedNSLabels))
		for k, v := range managedNSLabels {
			labelsMap[k] = v
		}
		Expect(unstructured.SetNestedField(app.Object, labelsMap, "spec", "syncPolicy", "managedNamespaceMetadata", "labels")).To(Succeed())
	}

	Expect(k8sClient.Update(ctx, app)).To(Succeed(), "failed to update Application %s/%s", namespace, name)
}

// Delete deletes an Argo CD Application.
func Delete(ref *AppRef) {
	Expect(ref.session).ToNot(BeNil(), "session is required for application.Delete")
	output, err := runArgoCDCLI(ref.session, "app", "delete", ref.Name, "--app-namespace", ref.Namespace, "--yes")
	Expect(err).ToNot(HaveOccurred(), "argocd app delete failed: %s", output)
}

// Ref creates a reference to an existing Application without creating it.
// Session is optional — when nil, argocd CLI get falls back to the k8s client.
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
		// No session — use k8s client directly (for Ref-only usage without CLI login)
		k8sClient, _ := fixtureUtils.GetE2ETestKubeClient()
		app := &unstructured.Unstructured{}
		app.SetGroupVersionKind(applicationGVR.GroupVersion().WithKind("Application"))
		if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}, app); err != nil {
			return nil, fmt.Errorf("k8s client get application failed: %v", err)
		}
		return app.Object, nil
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
