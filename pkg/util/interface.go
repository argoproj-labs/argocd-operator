package util

// ConvertStringsToInterfaces accepts a slice to strings and converts it into a slice to interfaces
func ConvertStringsToInterfaces(str []string) []interface{} {
	s := make([]interface{}, len(str))
	for i, v := range str {
		s[i] = v
	}
	return s
}
