package util

import "reflect"

// ConvertStringsToInterfaces accepts a slice to strings and converts it into a slice to interfaces
func ConvertStringsToInterfaces(str ...string) []interface{} {
	s := make([]interface{}, len(str))
	for i, v := range str {
		s[i] = v
	}
	return s
}

func ConvertStringMapToInterfaces(val ...map[string]string) []interface{} {
	s := make([]interface{}, len(val))
	for i, v := range val {
		s[i] = v
	}
	return s
}

// IsPtr tells us if a provided interface is a pointer or not
func IsPtr(i interface{}) bool {
	if i == nil {
		return false
	}
	return reflect.ValueOf(i).Type().Kind() == reflect.Ptr
}

// IsSlice tells us if a provided interface is a slice or not
func IsSlice(i interface{}) bool {
	if i == nil {
		return false
	}
	return reflect.ValueOf(i).Type().Kind() == reflect.Slice
}
