// Copyright 2019 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package argoutil

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
)

func TestGenerateAgentPrincipalRedisProxyServiceName(t *testing.T) {
	tests := []struct {
		name     string
		crName   string
		expected string
	}{
		{
			name:     "short CR name - no truncation",
			crName:   "short-name",
			expected: "short-name-agent-principal-redisproxy",
		},
		{
			name: "long CR name - uses truncated name",
			// Input matches the test case from TestTruncateCRName
			crName: "this-is-a-very-long-argocd-instance-name-that-exceeds-37-characters",
			// Base name is truncated to 36 chars to accommodate the 27 char suffix
			expected: "this-is-a-very-long-argocd-i-657aacd-agent-principal-redisproxy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateAgentPrincipalRedisProxyServiceName(tt.crName)
			assert.Equal(t, tt.expected, result)

			// Enforce K8s 63-character service name limit
			assert.LessOrEqual(t, len(result), 63)
		})
	}
}

func TestTruncateWithHash(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		maxLength   int
		expected    string
		description string
	}{
		{
			name:        "short string - no truncation needed",
			input:       "short-name",
			maxLength:   63,
			expected:    "short-name",
			description: "Strings shorter than maxLength should be returned unchanged",
		},
		{
			name:        "exactly maxLength - no truncation needed",
			input:       "exactly-sixty-three-characters-long-string-that-is-perfect",
			maxLength:   63,
			expected:    "exactly-sixty-three-characters-long-string-that-is-perfect",
			description: "Strings exactly at maxLength should be returned unchanged",
		},
		{
			name:        "long string - needs truncation with hash",
			input:       "this-is-a-very-long-string-that-exceeds-the-maximum-length-and-needs-to-be-truncated",
			maxLength:   63,
			expected:    "this-is-a-very-long-string-that-exceeds-the-maximum-len-33d6f5f",
			description: "Long strings should be truncated and have a 7-character hash suffix",
		},
		{
			name:        "CR name truncation - maxCRNameLength",
			input:       "this-is-a-very-long-argocd-instance-name-that-exceeds-37-characters",
			maxLength:   37,
			expected:    "this-is-a-very-long-argocd-in-657aacd",
			description: "CR names should be truncated to maxCRNameLength with hash",
		},
		{
			name:        "extremely long string - minimal base with hash",
			input:       "this-is-an-extremely-long-string-that-is-so-long-it-will-need-to-be-completely-replaced-by-a-hash-because-there-is-no-room-for-any-part-of-the-original-string",
			maxLength:   63,
			expected:    "this-is-an-extremely-long-string-that-is-so-long-it-wil-7da3fa9",
			description: "Extremely long strings should be truncated to fit maxLength with hash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateWithHash(tt.input, tt.maxLength)

			// Check the exact expected result (deterministic SHA1)
			assert.Equal(t, tt.expected, result, tt.description)

			// Check length constraint
			assert.LessOrEqual(t, len(result), tt.maxLength, "Result should not exceed maxLength")

			// Check that result is deterministic
			result2 := TruncateWithHash(tt.input, tt.maxLength)
			assert.Equal(t, result, result2, "Function should be deterministic")

			// For short strings, should be unchanged
			if len(tt.input) <= tt.maxLength {
				assert.Equal(t, tt.input, result, "Short strings should not be modified")
			} else {
				// For long strings, should be different and shorter
				assert.NotEqual(t, tt.input, result, "Long strings should be modified")
				assert.Less(t, len(result), len(tt.input), "Result should be shorter than input")

				// Should contain a hash suffix (7 characters + hyphen)
				assert.Contains(t, result, "-", "Result should contain hash separator")
				assert.Len(t, result, tt.maxLength, "Result should be exactly maxLength")
			}
		})
	}
}

func TestTruncateWithHashUniqueness(t *testing.T) {
	inputs := []string{
		"namespace1",
		"namespace2",
		"very-long-namespace-name-that-will-be-truncated-1",
		"very-long-namespace-name-that-will-be-truncated-2",
		"argocd_grp-bk-time-deposit-servicing-activity-topic-streaming-12345678",
		"argocd_grp-bk-time-deposit-servicing-activity-topic-streaming-87654321",
	}

	results := make(map[string]bool)

	for _, input := range inputs {
		result := TruncateWithHash(input, GetMaxLabelLength())
		assert.False(t, results[result], "Hash should be unique for different inputs: %s", input)
		results[result] = true

		// Verify length constraint
		assert.LessOrEqual(t, len(result), GetMaxLabelLength(), "Result should not exceed maxLabelLength")
	}
}

func TestNameWithSuffixForStatefulSet(t *testing.T) {
	tests := []struct {
		name           string
		crName         string
		suffix         string
		expectContains string
		expectExact    string // For backward compatibility tests where we expect exact name
	}{
		{
			name:           "preserves full suffix for short CR names (backward compatibility)",
			crName:         "argocd",
			suffix:         "application-controller",
			expectExact:    "argocd-application-controller",
			expectContains: "application-controller",
		},
		{
			name:           "preserves full suffix for medium CR names (backward compatibility)",
			crName:         "example-argocd",
			suffix:         "application-controller",
			expectExact:    "example-argocd-application-controller",
			expectContains: "application-controller",
		},
		{
			name:           "abbreviates suffix for long CR names to stay within 52 char limit",
			crName:         "this-name-will-push-the-char-limit",
			suffix:         "application-controller",
			expectContains: "app-controller",
		},
		{
			name:           "abbreviates suffix for very long CR names",
			crName:         "very-long-argocd-instance-name-that-needs-truncation-and-more",
			suffix:         "application-controller",
			expectContains: "app-controller", // CR name truncated, suffix abbreviated to fit
		},
		{
			name:           "truncates with hash for very long CR names with non-abbreviated suffix",
			crName:         "very-long-argocd-instance-name-that-needs-truncation-and-more",
			suffix:         "redis-ha-server",
			expectContains: "", // No abbreviation for redis-ha-server, falls through to hash truncation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NameWithSuffixForStatefulSet(metav1.ObjectMeta{Name: tt.crName}, tt.suffix)

			// Verify StatefulSet name stays within limit
			assert.LessOrEqual(t, len(result), GetMaxStatefulSetNameLength(),
				"StatefulSet name must be <= %d chars", GetMaxStatefulSetNameLength())

			// Verify that after Kubernetes adds controller revision (11 chars), total stays within 63
			assert.LessOrEqual(t, len(result)+11, GetMaxLabelLength(),
				"StatefulSet name + controller revision must be <= %d chars", GetMaxLabelLength())

			// Verify exact name for backward compatibility cases
			if tt.expectExact != "" {
				assert.Equal(t, tt.expectExact, result,
					"Should preserve original naming for backward compatibility")
			}

			// Verify suffix is present when expected
			if tt.expectContains != "" {
				assert.Contains(t, result, tt.expectContains,
					"Should contain expected suffix")
			}

			// Verify function is deterministic
			result2 := NameWithSuffixForStatefulSet(metav1.ObjectMeta{Name: tt.crName}, tt.suffix)
			assert.Equal(t, result, result2, "Function should be deterministic")
		})
	}
}

func TestNameWithSuffix(t *testing.T) {
	tests := []struct {
		name   string
		crName string
		suffix string
	}{
		{
			name:   "returns concatenated name when both parts fit within limit",
			crName: "argocd",
			suffix: "redis",
		},
		{
			name:   "truncates CR name when combined length exceeds limit",
			crName: "argocd",
			suffix: "argocd-application-controller",
		},
		{
			name:   "handles long CR name with short suffix",
			crName: "long-argocd-cr-name-exceeding-limit",
			suffix: "server",
		},
		{
			name:   "applies double truncation for service account names exceeding limit",
			crName: "long-argocd-cr-name-exceeding-limit",
			suffix: "argocd-application-controller",
		},
		{
			name:   "truncates both CR name and suffix when extremely long",
			crName: "extremely-long-argocd-instance-name-that-exceeds-maximum-limits",
			suffix: "very-long-suffix-for-service-account-resource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NameWithSuffix(metav1.ObjectMeta{Name: tt.crName}, tt.suffix)

			// Verify result stays within Kubernetes label limit
			assert.LessOrEqual(t, len(result), GetMaxLabelLength(),
				"Result must be <= %d chars", GetMaxLabelLength())

			// Verify function is deterministic
			result2 := NameWithSuffix(metav1.ObjectMeta{Name: tt.crName}, tt.suffix)
			assert.Equal(t, result, result2, "Function should be deterministic")

			// Verify result is not empty
			assert.NotEmpty(t, result, "Result should not be empty")
		})
	}
}

func TestTruncateCRName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "short CR name - no truncation",
			input:    "short-argocd",
			expected: "short-argocd",
		},
		{
			name:     "long CR name - needs truncation",
			input:    "this-is-a-very-long-argocd-instance-name-that-exceeds-37-characters",
			expected: "this-is-a-very-long-argocd-in-657aacd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateCRName(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.LessOrEqual(t, len(result), GetMaxCRNameLength(), "Result should not exceed maxCRNameLength")
		})
	}
}

func TestGetImagePullPolicy(t *testing.T) {
	// Save and restore original environment variable
	originalEnv := os.Getenv("IMAGE_PULL_POLICY")
	defer func() {
		if originalEnv != "" {
			os.Setenv("IMAGE_PULL_POLICY", originalEnv)
		} else {
			os.Unsetenv("IMAGE_PULL_POLICY")
		}
	}()

	tests := []struct {
		name               string
		policy             corev1.PullPolicy
		envValue           string
		setEnv             bool
		expectedPullPolicy corev1.PullPolicy
		description        string
	}{
		{
			name:               "instance specific policy - Always",
			policy:             []corev1.PullPolicy{corev1.PullAlways}[0],
			setEnv:             false,
			expectedPullPolicy: corev1.PullAlways,
			description:        "When instance policy is set to Always, it should take precedence",
		},
		{
			name:               "instance specific policy - Never",
			policy:             []corev1.PullPolicy{corev1.PullNever}[0],
			setEnv:             false,
			expectedPullPolicy: corev1.PullNever,
			description:        "When instance policy is set to Never, it should take precedence",
		},
		{
			name:               "instance specific policy - IfNotPresent",
			policy:             []corev1.PullPolicy{corev1.PullIfNotPresent}[0],
			setEnv:             false,
			expectedPullPolicy: corev1.PullIfNotPresent,
			description:        "When instance policy is set to IfNotPresent, it should take precedence",
		},
		{
			name:               "instance policy overrides environment variable",
			policy:             []corev1.PullPolicy{corev1.PullNever}[0],
			envValue:           "Always",
			setEnv:             true,
			expectedPullPolicy: corev1.PullNever,
			description:        "Instance policy should take precedence over environment variable",
		},
		{
			name:               "environment variable - Always",
			policy:             "",
			envValue:           "Always",
			setEnv:             true,
			expectedPullPolicy: corev1.PullAlways,
			description:        "When instance policy is nil, environment variable should be used",
		},
		{
			name:               "environment variable - Never",
			policy:             "",
			envValue:           "Never",
			setEnv:             true,
			expectedPullPolicy: corev1.PullNever,
			description:        "When instance policy is nil, environment variable should be used",
		},
		{
			name:               "environment variable - IfNotPresent",
			policy:             "",
			envValue:           "IfNotPresent",
			setEnv:             true,
			expectedPullPolicy: corev1.PullIfNotPresent,
			description:        "When instance policy is nil, environment variable should be used",
		},
		{
			name:               "default policy when nothing set",
			setEnv:             false,
			expectedPullPolicy: corev1.PullIfNotPresent,
			description:        "When neither instance policy nor env var is set, default to IfNotPresent",
		},
		{
			name:               "empty environment variable falls back to default",
			policy:             "",
			envValue:           "",
			setEnv:             true,
			expectedPullPolicy: corev1.PullIfNotPresent,
			description:        "When env var is empty string, should fall back to default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			if tt.setEnv {
				os.Setenv("IMAGE_PULL_POLICY", tt.envValue)
			} else {
				os.Unsetenv("IMAGE_PULL_POLICY")
			}

			// Execute
			result := GetImagePullPolicy(tt.policy)

			// Assert
			assert.Equal(t, tt.expectedPullPolicy, result, tt.description)
		})
	}
}

// Argo CD tracking annotations applied by a parent (central) Argo CD instance to the ArgoCD CR.
const (
	reproduceArgoCDTrackingIDAnnotation     = "argocd.argoproj.io/tracking-id"
	reproduceArgoCDInstallationIDAnnotation = "argocd.argoproj.io/installation-id"
)

// TestAnnotationsForCluster_DoesNotPropagateCRAnnotations verifies AnnotationsForCluster returns
// only the operator's default annotations.
func TestAnnotationsForCluster_DoesNotPropagateCRAnnotations(t *testing.T) {
	cr := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "appteam",
			Namespace: "appteam-argocd",
			Annotations: map[string]string{
				reproduceArgoCDTrackingIDAnnotation:     "central-gitops:argoproj.io/ArgoCD:gitops/appteam",
				reproduceArgoCDInstallationIDAnnotation: "central-argocd",
				"example.com/team":                      "platform",
			},
		},
	}

	annotations := AnnotationsForCluster(cr)

	// Only operator defaults are present.
	assert.Equal(t, "appteam", annotations[common.AnnotationName])
	assert.Equal(t, "appteam-argocd", annotations[common.AnnotationNamespace])

	// No annotations are inherited from the CR.
	assert.NotContains(t, annotations, reproduceArgoCDTrackingIDAnnotation,
		"central Argo CD tracking-id must not be copied to operator-managed resources")
	assert.NotContains(t, annotations, reproduceArgoCDInstallationIDAnnotation,
		"central Argo CD installation-id must not be copied to operator-managed resources")
	assert.NotContains(t, annotations, "example.com/team",
		"CR annotations must not be propagated to operator-managed resources")

	assert.Len(t, annotations, 2, "only the two default annotations should be present")
}
