package util

import (
	"encoding/base64"
	"sort"
	"strings"
)

const (
	DotSep        = "."
	UnderscoreSep = "_"
)

// SplitList accepts a string input containing a list of comma separated values, and returns a slice containing those values as separate elements
func SplitList(s string) []string {
	if s == "" {
		return []string{}
	}
	elems := strings.Split(s, ",")
	for i := range elems {
		elems[i] = strings.TrimSpace(elems[i])
	}
	return elems
}

// RemoveString removes the given string from the given slice
func RemoveString(slice []string, s string) []string {
	var result []string
	if len(slice) == 0 {
		return []string{}
	}

	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return result
}

// ContainsString returns true if the given string is part of the given slice.
func ContainsString(arr []string, s string) bool {
	for _, val := range arr {
		if strings.TrimSpace(val) == s {
			return true
		}
	}
	return false
}

func Equal(a, b []string) bool {
	s1 := append([]string{}, a...)
	s2 := append([]string{}, b...)
	sort.Strings(s1)
	sort.Strings(s2)
	if len(s1) != len(s2) {
		return false
	}
	for i := range s1 {
		if s1[i] != s2[i] {
			return false
		}
	}
	return true
}

// GenerateRandomString returns a securely generated random string.
func GenerateRandomString(s int) (string, error) {
	b, err := GenerateRandomBytes(s)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// StringPtr returns a pointer to provided string value
func StringPtr(val string) *string {
	return &val
}

// ConstructString concatenates the supplied parts by using the provided separator. Any empty strings are skipped
func ConstructString(separtor string, parts ...string) string {
	return strings.Join(RemoveString(parts, ""), separtor)
}
