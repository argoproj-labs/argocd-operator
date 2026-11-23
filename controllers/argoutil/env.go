package argoutil

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
)

// EnvGet returns a pointer to the EnvVar with the given name, or nil if not found.
// The pointer refers into envs, so callers may update the entry in place.
func EnvGet(envs []corev1.EnvVar, name string) *corev1.EnvVar {
	for i := range envs {
		if envs[i].Name == name {
			return &envs[i]
		}
	}
	return nil
}

// EnvRemove returns a copy of envs with any EnvVar named name removed.
// Order of remaining entries is preserved.
func EnvRemove(envs []corev1.EnvVar, name string) []corev1.EnvVar {
	result := make([]corev1.EnvVar, 0, len(envs))
	for _, e := range envs {
		if e.Name != name {
			result = append(result, e)
		}
	}
	return result
}

// EnvSet returns a copy of envs with env upserted by Name: existing entry is
// replaced in place, otherwise env is appended. Order of other entries is preserved.
func EnvSet(envs []corev1.EnvVar, env corev1.EnvVar) []corev1.EnvVar {
	result := make([]corev1.EnvVar, 0, len(envs)+1)
	found := false
	for _, e := range envs {
		if e.Name == env.Name {
			result = append(result, env)
			found = true
		} else {
			result = append(result, e)
		}
	}
	if !found {
		result = append(result, env)
	}
	return result
}

// EnvMerge merges two slices of EnvVar entries into a single one. If existing
// has an EnvVar with same Name attribute as one in merge, the EnvVar is not
// merged unless override is set to true.
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

	for _, v := range final {
		ret = append(ret, v)
	}

	// sort result slice by env name
	sort.SliceStable(ret,
		func(i, j int) bool {
			return ret[i].Name < ret[j].Name
		})

	return ret
}
