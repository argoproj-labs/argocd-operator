package argoutil

import (
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
