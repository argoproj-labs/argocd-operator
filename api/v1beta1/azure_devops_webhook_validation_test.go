package v1beta1

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCRDIncludesAzureDevOpsWebhookSecretsPairValidation asserts the generated Argo CD CRD embeds a CEL rule
// requiring usernameSecretRef and passwordSecretRef on spec.webhookSecrets.azureDevOps to be both set or both omitted.
func TestCRDIncludesAzureDevOpsWebhookSecretsPairValidation(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	crdPath := filepath.Join(repoRoot, "config", "crd", "bases", "argoproj.io_argocds.yaml")
	data, err := os.ReadFile(crdPath)
	if err != nil {
		t.Fatalf("read CRD %s: %v", crdPath, err)
	}
	content := string(data)

	assert.GreaterOrEqual(t, strings.Count(content, "has(self.usernameSecretRef)"), 2,
		"expected v1beta1 and v1alpha1 OpenAPI schemas to include Azure DevOps username ref CEL check")
	assert.GreaterOrEqual(t, strings.Count(content, "has(self.passwordSecretRef)"), 2,
		"expected v1beta1 and v1alpha1 OpenAPI schemas to include Azure DevOps password ref CEL check")

	assert.Contains(t, content, "usernameSecretRef and passwordSecretRef must be set")
	assert.Contains(t, content, "together")

	assert.Contains(t, content, "x-kubernetes-validations:")
}
