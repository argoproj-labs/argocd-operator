package util

// SetDiff returns A - B (elements in A not present in B)
func SetDiff(a, b map[string]string) map[string]string {
	diff := make(map[string]string)

	for k := range a {
		if _, ok := b[k]; !ok {
			diff[k] = ""
		}
	}

	return diff
}

// SetIntersection returns A intersection B
func SetIntersection(a, b map[string]string) map[string]string {
	intsec := make(map[string]string)

	for k := range a {
		if _, ok := b[k]; ok {
			intsec[k] = ""
		}
	}

	return intsec
}
