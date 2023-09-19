package util

import (
	"encoding/base64"
	"sort"
	"strings"
)

func SplitList(s string) []string {
	elems := strings.Split(s, ",")
	for i := range elems {
		elems[i] = strings.TrimSpace(elems[i])
	}
	return elems
}

func RemoveString(slice []string, s string) []string {
	var result []string
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return result
}

// // ContainsString returns true if a string is part of the given slice.
//
//	func ContainsString(arr []string, s string) bool {
//		for _, val := range arr {
//			if strings.TrimSpace(val) == s {
//				return true
//			}
//		}
//		return false
//	}
func ContainsString(arr []string, s string, ifTrimSpace bool) bool {
	for _, val := range arr {
		if ifTrimSpace {
			val = strings.TrimSpace(val)
		}
		if val == s {
			return true
		}
	}
	return false
}

func Equal(a, b []string) bool {
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
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
