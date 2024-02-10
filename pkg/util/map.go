package util

import "sort"

// combines 2 maps and returns the result. In case of conflicts, values in 2nd input overwrite values in 1st input
func MergeMaps(a, b map[string]string) map[string]string {
	mergedMap := make(map[string]string, 0)

	for k, v := range a {
		mergedMap[k] = v
	}
	for k, v := range b {
		mergedMap[k] = v
	}
	return mergedMap
}

// StringMapKeys accepts a map with string keys as input and returns a sorted slice of its keys
func StringMapKeys(m map[string]string) []string {
	r := []string{}
	for k := range m {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}

func EmptyMap() map[string]string {
	return map[string]string{}
}
