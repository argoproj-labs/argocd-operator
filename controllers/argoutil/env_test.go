package argoutil

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func Test_EnvGet(t *testing.T) {
	envs := []corev1.EnvVar{{Name: "FOO", Value: "BAR"}, {Name: "BAZ", ValueFrom: &corev1.EnvVarSource{}}}

	e := EnvGet(envs, "FOO")
	assert.NotNil(t, e)
	assert.Equal(t, "BAR", e.Value)

	e = EnvGet(envs, "BAZ")
	assert.NotNil(t, e)
	assert.NotNil(t, e.ValueFrom)

	assert.Nil(t, EnvGet(envs, "MISSING"))
	assert.Nil(t, EnvGet(nil, "FOO"))

	// In-place update via the returned pointer
	EnvGet(envs, "FOO").Value = "UPDATED"
	assert.Equal(t, "UPDATED", envs[0].Value)
}

func Test_EnvRemove(t *testing.T) {
	envs := []corev1.EnvVar{
		{Name: "FOO", Value: "BAR"},
		{Name: "BAR", Value: "FOO"},
		{Name: "BAZ", Value: "BAZ"},
	}

	r := EnvRemove(envs, "BAR")
	assert.Equal(t, []corev1.EnvVar{{Name: "FOO", Value: "BAR"}, {Name: "BAZ", Value: "BAZ"}}, r)

	r = EnvRemove(envs, "MISSING")
	assert.Equal(t, envs, r)

	r = EnvRemove(nil, "FOO")
	assert.Empty(t, r)
}

func Test_EnvSet(t *testing.T) {
	envs := []corev1.EnvVar{
		{Name: "FOO", Value: "BAR"},
		{Name: "BAR", Value: "FOO"},
	}

	r := EnvSet(envs, corev1.EnvVar{Name: "FOO", Value: "NEW"})
	assert.Equal(t, []corev1.EnvVar{{Name: "FOO", Value: "NEW"}, {Name: "BAR", Value: "FOO"}}, r)

	r = EnvSet(envs, corev1.EnvVar{Name: "BAZ", Value: "BAZ"})
	assert.Equal(t, []corev1.EnvVar{{Name: "FOO", Value: "BAR"}, {Name: "BAR", Value: "FOO"}, {Name: "BAZ", Value: "BAZ"}}, r)

	r = EnvSet(nil, corev1.EnvVar{Name: "FOO", Value: "BAR"})
	assert.Equal(t, []corev1.EnvVar{{Name: "FOO", Value: "BAR"}}, r)
}

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
