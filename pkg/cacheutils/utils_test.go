package cacheutils

import (
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj-labs/argocd-operator/common"
)

// TestStripDataFromSecretOrConfigMapTransform tests the StripDataFromSecretOrConfigMapTransform function
func TestStripDataFromSecretOrConfigMapTransform(t *testing.T) {
	tests := []struct {
		name           string
		input          interface{}
		expectedResult interface{}
		expectedError  bool
		description    string
	}{
		{
			name: "tracked secret with data should remain unchanged",
			input: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "test-namespace",
					Labels: map[string]string{
						common.ArgoCDTrackedByOperatorLabel: "argocd",
					},
				},
				Data: map[string][]byte{
					"key1": []byte("value1"),
					"key2": []byte("value2"),
				},
				StringData: map[string]string{
					"key3": "value3",
				},
			},
			expectedResult: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "test-namespace",
					Labels: map[string]string{
						common.ArgoCDTrackedByOperatorLabel: "argocd",
					},
				},
				Data: map[string][]byte{
					"key1": []byte("value1"),
					"key2": []byte("value2"),
				},
				StringData: map[string]string{
					"key3": "value3",
				},
			},
			expectedError: false,
			description:   "Secret with operator tracking label should keep all data",
		},
		{
			name: "tracked secret with secret type label should remain unchanged",
			input: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "test-namespace",
					Labels: map[string]string{
						common.ArgoCDSecretTypeLabel: "repository",
					},
				},
				Data: map[string][]byte{
					"key1": []byte("value1"),
				},
			},
			expectedResult: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "test-namespace",
					Labels: map[string]string{
						common.ArgoCDSecretTypeLabel: "repository",
					},
				},
				Data: map[string][]byte{
					"key1": []byte("value1"),
				},
			},
			expectedError: false,
			description:   "Secret with secret type label should keep all data",
		},
		{
			name: "non-tracked secret should have data stripped",
			input: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"other-label": "value",
					},
				},
				Data: map[string][]byte{
					"key1": []byte("value1"),
					"key2": []byte("value2"),
				},
				StringData: map[string]string{
					"key3": "value3",
				},
			},
			expectedResult: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"other-label": "value",
					},
				},
				Data:       nil,
				StringData: nil,
			},
			expectedError: false,
			description:   "Secret without tracking labels should have data stripped",
		},
		{
			name: "secret with no labels should have data stripped",
			input: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"key1": []byte("value1"),
				},
			},
			expectedResult: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "test-namespace",
				},
				Data:       nil,
				StringData: nil,
			},
			expectedError: false,
			description:   "Secret with no labels should have data stripped",
		},
		{
			name: "secret with nil labels should have data stripped",
			input: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "test-namespace",
					Labels:    nil,
				},
				Data: map[string][]byte{
					"key1": []byte("value1"),
				},
			},
			expectedResult: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "test-namespace",
					Labels:    nil,
				},
				Data:       nil,
				StringData: nil,
			},
			expectedError: false,
			description:   "Secret with nil labels should have data stripped",
		},
		{
			name: "tracked ConfigMap with data should remain unchanged",
			input: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "test-namespace",
					Labels: map[string]string{
						common.ArgoCDTrackedByOperatorLabel: "argocd",
					},
				},
				Data: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
				BinaryData: map[string][]byte{
					"key3": []byte("value3"),
				},
			},
			expectedResult: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "test-namespace",
					Labels: map[string]string{
						common.ArgoCDTrackedByOperatorLabel: "argocd",
					},
				},
				Data: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
				BinaryData: map[string][]byte{
					"key3": []byte("value3"),
				},
			},
			expectedError: false,
			description:   "ConfigMap with operator tracking label should keep all data",
		},
		{
			name: "tracked ConfigMap with secret type label should remain unchanged",
			input: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "test-namespace",
					Labels: map[string]string{
						common.ArgoCDSecretTypeLabel: "repository",
					},
				},
				Data: map[string]string{
					"key1": "value1",
				},
			},
			expectedResult: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "test-namespace",
					Labels: map[string]string{
						common.ArgoCDSecretTypeLabel: "repository",
					},
				},
				Data: map[string]string{
					"key1": "value1",
				},
			},
			expectedError: false,
			description:   "ConfigMap with secret type label should keep all data",
		},
		{
			name: "non-tracked ConfigMap should have data stripped",
			input: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"other-label": "value",
					},
				},
				Data: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
				BinaryData: map[string][]byte{
					"key3": []byte("value3"),
				},
			},
			expectedResult: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"other-label": "value",
					},
				},
				Data:       nil,
				BinaryData: nil,
			},
			expectedError: false,
			description:   "ConfigMap without tracking labels should have data stripped",
		},
		{
			name: "ConfigMap with no labels should have data stripped",
			input: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"key1": "value1",
				},
			},
			expectedResult: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "test-namespace",
				},
				Data:       nil,
				BinaryData: nil,
			},
			expectedError: false,
			description:   "ConfigMap with no labels should have data stripped",
		},
		{
			name: "ConfigMap with nil labels should have data stripped",
			input: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "test-namespace",
					Labels:    nil,
				},
				Data: map[string]string{
					"key1": "value1",
				},
			},
			expectedResult: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm",
					Namespace: "test-namespace",
					Labels:    nil,
				},
				Data:       nil,
				BinaryData: nil,
			},
			expectedError: false,
			description:   "ConfigMap with nil labels should have data stripped",
		},
	}

	transform := StripDataFromSecretOrConfigMapTransform()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For non-Secret inputs, set expected result to the same object reference
			if tt.expectedResult == nil && tt.input != nil {
				tt.expectedResult = tt.input
			}

			result, err := transform(tt.input)

			if tt.expectedError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// For Secrets, compare the actual fields
			if secret, ok := result.(*v1.Secret); ok {
				expectedSecret, ok := tt.expectedResult.(*v1.Secret)
				if !ok {
					t.Errorf("expected result is not a Secret")
					return
				}

				if secret.Name != expectedSecret.Name {
					t.Errorf("expected name %s, got %s", expectedSecret.Name, secret.Name)
				}
				if secret.Namespace != expectedSecret.Namespace {
					t.Errorf("expected namespace %s, got %s", expectedSecret.Namespace, secret.Namespace)
				}

				// Compare labels
				if len(secret.Labels) != len(expectedSecret.Labels) {
					t.Errorf("expected %d labels, got %d", len(expectedSecret.Labels), len(secret.Labels))
				}
				for k, v := range expectedSecret.Labels {
					if secret.Labels[k] != v {
						t.Errorf("expected label %s=%s, got %s", k, v, secret.Labels[k])
					}
				}

				// Compare data fields
				if len(secret.Data) != len(expectedSecret.Data) {
					t.Errorf("expected %d data entries, got %d", len(expectedSecret.Data), len(secret.Data))
				}
				for k, v := range expectedSecret.Data {
					if string(secret.Data[k]) != string(v) {
						t.Errorf("expected data %s=%s, got %s", k, string(v), string(secret.Data[k]))
					}
				}

				// Compare string data fields
				if len(secret.StringData) != len(expectedSecret.StringData) {
					t.Errorf("expected %d string data entries, got %d", len(expectedSecret.StringData), len(secret.StringData))
				}
				for k, v := range expectedSecret.StringData {
					if secret.StringData[k] != v {
						t.Errorf("expected string data %s=%s, got %s", k, v, secret.StringData[k])
					}
				}
			} else if configMap, ok := result.(*v1.ConfigMap); ok {
				// For ConfigMaps, compare the actual fields
				expectedConfigMap, ok := tt.expectedResult.(*v1.ConfigMap)
				if !ok {
					t.Errorf("expected result is not a ConfigMap")
					return
				}

				if configMap.Name != expectedConfigMap.Name {
					t.Errorf("expected name %s, got %s", expectedConfigMap.Name, configMap.Name)
				}
				if configMap.Namespace != expectedConfigMap.Namespace {
					t.Errorf("expected namespace %s, got %s", expectedConfigMap.Namespace, configMap.Namespace)
				}

				// Compare labels
				if len(configMap.Labels) != len(expectedConfigMap.Labels) {
					t.Errorf("expected %d labels, got %d", len(expectedConfigMap.Labels), len(configMap.Labels))
				}
				for k, v := range expectedConfigMap.Labels {
					if configMap.Labels[k] != v {
						t.Errorf("expected label %s=%s, got %s", k, v, configMap.Labels[k])
					}
				}

				// Compare data fields
				if len(configMap.Data) != len(expectedConfigMap.Data) {
					t.Errorf("expected %d data entries, got %d", len(expectedConfigMap.Data), len(configMap.Data))
				}
				for k, v := range expectedConfigMap.Data {
					if configMap.Data[k] != v {
						t.Errorf("expected data %s=%s, got %s", k, v, configMap.Data[k])
					}
				}

				// Compare binary data fields
				if len(configMap.BinaryData) != len(expectedConfigMap.BinaryData) {
					t.Errorf("expected %d binary data entries, got %d", len(expectedConfigMap.BinaryData), len(configMap.BinaryData))
				}
				for k, v := range expectedConfigMap.BinaryData {
					if string(configMap.BinaryData[k]) != string(v) {
						t.Errorf("expected binary data %s=%s, got %s", k, string(v), string(configMap.BinaryData[k]))
					}
				}
			} else {
				// For other inputs, compare using reflect.DeepEqual for complex types
				if !reflect.DeepEqual(result, tt.expectedResult) {
					t.Errorf("expected %v, got %v", tt.expectedResult, result)
				}
			}
		})
	}
}

// TestStripDataFromSecretOrConfigMapTransformEdgeCases tests edge cases for StripDataFromSecretOrConfigMapTransform
func TestStripDataFromSecretOrConfigMapTransformEdgeCases(t *testing.T) {
	transform := StripDataFromSecretOrConfigMapTransform()

	t.Run("secret with only Data field should be stripped", func(t *testing.T) {
		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "test-namespace",
				Labels:    map[string]string{"other": "label"},
			},
			Data: map[string][]byte{
				"key1": []byte("value1"),
			},
		}

		result, err := transform(secret)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		resultSecret, ok := result.(*v1.Secret)
		if !ok {
			t.Errorf("expected Secret, got %T", result)
			return
		}

		if resultSecret.Data != nil {
			t.Errorf("expected Data to be nil, got %v", resultSecret.Data)
		}
		if resultSecret.StringData != nil {
			t.Errorf("expected StringData to be nil, got %v", resultSecret.StringData)
		}
	})

	t.Run("secret with only StringData field should be stripped", func(t *testing.T) {
		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "test-namespace",
				Labels:    map[string]string{"other": "label"},
			},
			StringData: map[string]string{
				"key1": "value1",
			},
		}

		result, err := transform(secret)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		resultSecret, ok := result.(*v1.Secret)
		if !ok {
			t.Errorf("expected Secret, got %T", result)
			return
		}

		if resultSecret.Data != nil {
			t.Errorf("expected Data to be nil, got %v", resultSecret.Data)
		}
		if resultSecret.StringData != nil {
			t.Errorf("expected StringData to be nil, got %v", resultSecret.StringData)
		}
	})

	t.Run("configmap with empty data maps should be stripped", func(t *testing.T) {
		cm := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cm",
				Namespace: "test-namespace",
				Labels:    map[string]string{"other": "label"},
			},
			Data:       map[string]string{},
			BinaryData: map[string][]byte{},
		}

		result, err := transform(cm)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		resultCM, ok := result.(*v1.ConfigMap)
		if !ok {
			t.Errorf("expected ConfigMap, got %T", result)
			return
		}

		if resultCM.Data != nil {
			t.Errorf("expected Data to be nil, got %v", resultCM.Data)
		}
		if resultCM.BinaryData != nil {
			t.Errorf("expected BinaryData to be nil, got %v", resultCM.BinaryData)
		}
	})

	t.Run("configmap with only Data field should be stripped", func(t *testing.T) {
		cm := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cm",
				Namespace: "test-namespace",
				Labels:    map[string]string{"other": "label"},
			},
			Data: map[string]string{
				"key1": "value1",
			},
		}

		result, err := transform(cm)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		resultCM, ok := result.(*v1.ConfigMap)
		if !ok {
			t.Errorf("expected ConfigMap, got %T", result)
			return
		}

		if resultCM.Data != nil {
			t.Errorf("expected Data to be nil, got %v", resultCM.Data)
		}
		if resultCM.BinaryData != nil {
			t.Errorf("expected BinaryData to be nil, got %v", resultCM.BinaryData)
		}
	})

	t.Run("configmap with only BinaryData field should be stripped", func(t *testing.T) {
		cm := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cm",
				Namespace: "test-namespace",
				Labels:    map[string]string{"other": "label"},
			},
			BinaryData: map[string][]byte{
				"key1": []byte("value1"),
			},
		}

		result, err := transform(cm)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		resultCM, ok := result.(*v1.ConfigMap)
		if !ok {
			t.Errorf("expected ConfigMap, got %T", result)
			return
		}

		if resultCM.Data != nil {
			t.Errorf("expected Data to be nil, got %v", resultCM.Data)
		}
		if resultCM.BinaryData != nil {
			t.Errorf("expected BinaryData to be nil, got %v", resultCM.BinaryData)
		}
	})
}

// TestIsTrackedByOperator tests the IsTrackedByOperator function
func TestIsTrackedByOperator(t *testing.T) {
	tests := []struct {
		name        string
		obj         interface{}
		expected    bool
		description string
	}{
		{
			name:        "nil object should return false",
			obj:         nil,
			expected:    false,
			description: "Nil object should not be considered tracked",
		},
		{
			name: "object with nil labels should return false",
			obj: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
					Labels:    nil,
				},
			},
			expected:    false,
			description: "Object with nil labels should not be considered tracked",
		},
		{
			name: "object with empty labels should return false",
			obj: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
					Labels:    map[string]string{},
				},
			},
			expected:    false,
			description: "Object with empty labels should not be considered tracked",
		},
		{
			name: "object with operator tracking label should return true",
			obj: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
					Labels: map[string]string{
						common.ArgoCDTrackedByOperatorLabel: "argocd",
					},
				},
			},
			expected:    true,
			description: "Object with operator tracking label should be considered tracked",
		},
		{
			name: "object with secret type label should return true",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-configmap",
					Namespace: "default",
					Labels: map[string]string{
						common.ArgoCDSecretTypeLabel: "repository",
					},
				},
			},
			expected:    true,
			description: "Object with secret type label should be considered tracked",
		},
		{
			name: "object with both tracking labels should return true",
			obj: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
					Labels: map[string]string{
						common.ArgoCDTrackedByOperatorLabel: "argocd",
						common.ArgoCDSecretTypeLabel:        "repository",
					},
				},
			},
			expected:    true,
			description: "Object with both tracking labels should be considered tracked",
		},
		{
			name: "object with other labels should return false",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-configmap",
					Namespace: "default",
					Labels: map[string]string{
						"other-label": "value",
						"app":         "test",
					},
				},
			},
			expected:    false,
			description: "Object without tracking labels should not be considered tracked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var obj runtime.Object
			if tt.obj != nil {
				if runtimeObj, ok := tt.obj.(runtime.Object); ok {
					obj = runtimeObj
				} else {
					// For objects that don't implement runtime.Object, pass nil
					obj = nil
				}
			}
			result := IsTrackedByOperator(obj)
			if result != tt.expected {
				t.Errorf("expected %v, got %v. %s", tt.expected, result, tt.description)
			}
		})
	}
}
