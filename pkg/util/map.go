package util

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

// TO DO: AppendStringMap and MergeMaps do the same thing, get rid of AppendStringMap!

// AppendStringMap will append the map `add` to the given map `src` and return the result.
func AppendStringMap(src map[string]string, add map[string]string) map[string]string {
	res := src
	if len(src) <= 0 {
		res = make(map[string]string, len(add))
	}
	for key, val := range add {
		res[key] = val
	}
	return res
}
