package argoutil

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
