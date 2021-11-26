package argoutil

import (
	corev1 "k8s.io/api/core/v1"
	"sort"
)

// EnvMerge merges two slices of EnvVar entries into a single one. If existing
// has an EnvVar with same Name attribute as one in merge, the EnvVar is not
// merged unless override is set to true. The resulting slice is not guaranteed
// to be in the same order than the existing input.
func EnvMerge(existing []corev1.EnvVar, merge []corev1.EnvVar, override bool) []corev1.EnvVar {
	ret := []corev1.EnvVar{}
	final := map[string]corev1.EnvVar{}
	for _, e := range existing {
		final[e.Name] = e
	}
	for _, m := range merge {
		if _, ok := final[m.Name]; ok {
			if override {
				final[m.Name] = m
			}
		} else {
			final[m.Name] = m
		}
	}

	envName := make([]string, len(final))
	for _, env := range final {
		envName = append(envName, env.Name)
	}
	sort.Strings(envName)
	for _, name := range envName {
		for _, v := range final {
			if v.Name == name {
				ret = append(ret, v)
			}
		}
	}
	return ret
}
