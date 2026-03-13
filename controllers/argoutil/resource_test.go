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
)

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
