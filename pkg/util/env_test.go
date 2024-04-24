package util

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func Test_EnvMerge(t *testing.T) {
	t.Run("Merge non-existing env", func(t *testing.T) {
		e := []corev1.EnvVar{
			{
				Name:  "FOO",
				Value: "BAR",
			},
			{
				Name:  "BAR",
				Value: "FOO",
			},
		}
		r := EnvMerge(e, []corev1.EnvVar{{Name: "BAZ", Value: "BAZ"}}, false)
		// New element
		assert.Len(t, r, 3)
		assert.Contains(t, r, corev1.EnvVar{Name: "BAZ", Value: "BAZ"})
	})
	t.Run("Merge multiple non-existing and existing env", func(t *testing.T) {
		e := []corev1.EnvVar{
			{
				Name:  "FOO",
				Value: "BAR",
			},
			{
				Name:  "BAR",
				Value: "FOO",
			},
		}
		r := EnvMerge(e, []corev1.EnvVar{{Name: "BAZ", Value: "BAZ"}, {Name: "FOO", Value: "FOO"}}, false)
		// New element
		assert.Equal(t, len(r), 3)
		// New variable should be the one we added
		assert.Contains(t, r, corev1.EnvVar{Name: "BAR", Value: "FOO"})
		assert.Contains(t, r, corev1.EnvVar{Name: "FOO", Value: "BAR"})
		assert.Contains(t, r, corev1.EnvVar{Name: "BAZ", Value: "BAZ"})
		assert.NotContains(t, r, corev1.EnvVar{Name: "FOO", Value: "FOO"})
	})
	t.Run("Merge existing env with override", func(t *testing.T) {
		e := []corev1.EnvVar{
			{
				Name:  "FOO",
				Value: "BAR",
			},
			{
				Name:  "BAR",
				Value: "FOO",
			},
		}
		r := EnvMerge(e, []corev1.EnvVar{{Name: "FOO", Value: "FOO"}}, true)
		// No new element
		assert.Equal(t, len(r), 2)
		// Variable has been overwritten
		assert.Contains(t, r, corev1.EnvVar{Name: "FOO", Value: "FOO"})
		assert.Contains(t, r, corev1.EnvVar{Name: "BAR", Value: "FOO"})
	})
	t.Run("Merge existing env without override", func(t *testing.T) {
		e := []corev1.EnvVar{
			{
				Name:  "FOO",
				Value: "BAR",
			},
			{
				Name:  "BAR",
				Value: "FOO",
			},
		}
		r := EnvMerge(e, []corev1.EnvVar{{Name: "FOO", Value: "FOO"}}, false)
		// No new element
		assert.Equal(t, len(r), 2)
		// Variable has not been changed
		assert.Contains(t, r, corev1.EnvVar{Name: "FOO", Value: "BAR"})
		assert.Contains(t, r, corev1.EnvVar{Name: "BAR", Value: "FOO"})
	})
}

func Test_EnvMerge_testSorted(t *testing.T) {
	t.Run("Merge non-existing env", func(t *testing.T) {
		e := []corev1.EnvVar{
			{
				Name:  "FOO",
				Value: "BAR",
			},
			{
				Name:  "BAR",
				Value: "FOO",
			},
		}
		r := EnvMerge(e, []corev1.EnvVar{{Name: "BAZ", Value: "BAZ"}}, false)

		// verify if the Env Vars are sorted by names
		s := []corev1.EnvVar{
			{
				Name:  "BAR",
				Value: "FOO",
			},
			{
				Name:  "BAZ",
				Value: "BAZ",
			},
			{
				Name:  "FOO",
				Value: "BAR",
			},
		}
		if !reflect.DeepEqual(r, s) {
			assert.Fail(t, "environmental variables are not sorted")
		}
	})
	t.Run("Merge multiple non-existing and existing env", func(t *testing.T) {
		e := []corev1.EnvVar{
			{
				Name:  "FOO",
				Value: "BAR",
			},
			{
				Name:  "BAR",
				Value: "FOO",
			},
		}
		r := EnvMerge(e, []corev1.EnvVar{{Name: "BAZ", Value: "BAZ"}, {Name: "FOO", Value: "FOO"}}, true)

		// verify if the Env Vars are sorted by names
		s := []corev1.EnvVar{
			{
				Name:  "BAR",
				Value: "FOO",
			},
			{
				Name:  "BAZ",
				Value: "BAZ",
			},
			{
				Name:  "FOO",
				Value: "FOO",
			},
		}
		// New variable should be the one we added
		if !reflect.DeepEqual(r, s) {
			assert.Fail(t, "environmental variables are not sorted")
		}
	})
}

func TestGetEnv(t *testing.T) {
	t.Run("Setting Test env", func(t *testing.T) {
		key := "TEST_ENV_VAR"
		value := "test_value"
		err := os.Setenv(key, value)
		if err != nil {
			t.Errorf("error setting env %v", err)
		}
		defer func(key string) {
			err := os.Unsetenv(key)
			if err != nil {
				t.Errorf("Error unsetting env %v", err)
			}
		}(key)

		result := GetEnv(key)

		if result != value {
			t.Errorf("GetEnv() result = %v, want %v", result, value)
		}
	})
}

func TestCaseInsensitiveGetenv(t *testing.T) {
	t.Run("Merge non-existing env", func(t *testing.T) {
		key := "TEST_env_VAR_Case_Insensitive"
		value := "tEsT_vAlUE1"
		key_lowercase := "test_env_var_case_insensitive"
		err := os.Setenv(key_lowercase, value)
		if err != nil {
			t.Errorf("error setting env %v", err)
		}
		defer func(key string) {
			err := os.Unsetenv(key)
			if err != nil {
				t.Errorf("Error unsetting env %v", err)
			}
		}(key)

		result_caseinsensitive, result_value := CaseInsensitiveGetenv(key)

		if result_value != value {
			t.Errorf("Wrong value: result = %v, want %v", result_value, value)
		}

		if result_caseinsensitive != key_lowercase {
			t.Errorf("GetEnv() result = %v, want %v", result_caseinsensitive, key_lowercase)
		}
	})
}

func TestProxyEnvVars(t *testing.T) {
	t.Run("Merge non-existing env", func(t *testing.T) {
		proxyKeys := []string{HttpProxy, HttpsProxy, NoProxy}
		values := []string{"http://proxy.example.com", "https://proxy.example.com", "localhost,127.0.0.1"}
		for i := 0; i < 3; i++ {
			err := os.Setenv(proxyKeys[i], values[i])
			if err != nil {
				t.Errorf("error setting env %v", err)
			}
			defer func(key string) {
				err := os.Unsetenv(key)
				if err != nil {
					t.Errorf("Error unsetting env %v", err)
				}
			}(proxyKeys[i])
		}

		envVars := ProxyEnvVars()

		expectedEnvVars := []corev1.EnvVar{
			{Name: proxyKeys[0], Value: values[0]},
			{Name: proxyKeys[1], Value: values[1]},
			{Name: proxyKeys[2], Value: values[2]},
		}

		if len(envVars) != len(expectedEnvVars) {
			t.Errorf("ProxyEnvVars() result length  = %v, want %v", len(envVars), len(expectedEnvVars))
		}

		if !reflect.DeepEqual(envVars, expectedEnvVars) {
			t.Errorf("ProxyEnvVars() result = %v, want %v", envVars, expectedEnvVars)

		}
	})
}
