package util

import (
	"reflect"
	"testing"
)

func TestConvertStringsToInterfaces(t *testing.T) {
	tests := []struct {
		name     string
		str      []string
		expected []interface{}
	}{
		{
			"ConvertStringsToInterfaces Basic",
			[]string{"apple", "banana", "orange"},
			[]interface{}{"apple", "banana", "orange"},
		},
		{
			"ConvertStringsToInterfaces Empty",
			[]string{},
			[]interface{}{},
		},
		{
			"ConvertStringsToInterfaces With Spaces",
			[]string{"apple", "banana ", " orange"},
			[]interface{}{"apple", "banana ", " orange"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertStringsToInterfaces(tt.str...)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ConvertStringsToInterfaces() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConvertStringMapToInterfaces(t *testing.T) {
	tests := []struct {
		name     string
		val      []map[string]string
		expected []interface{}
	}{
		{
			"ConvertStringMapToInterfaces Basic",
			[]map[string]string{{"key1": "value1"}, {"key2": "value2"}},
			[]interface{}{map[string]string{"key1": "value1"}, map[string]string{"key2": "value2"}},
		},
		{
			"ConvertStringMapToInterfaces Empty",
			[]map[string]string{},
			[]interface{}{},
		},
		{
			"ConvertStringMapToInterfaces With Empty Map",
			[]map[string]string{{}, {"key": "value"}},
			[]interface{}{map[string]string{}, map[string]string{"key": "value"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertStringMapToInterfaces(tt.val...)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ConvertStringMapToInterfaces() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsPtr(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{
			"IsPtr True",
			&struct{}{},
			true,
		},
		{
			"IsPtr False",
			struct{}{},
			false,
		},
		{
			"IsPtr Nil",
			nil,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPtr(tt.input)

			if got != tt.expected {
				t.Errorf("IsPtr() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{
			"IsSlice True",
			[]int{1, 2, 3},
			true,
		},
		{
			"IsSlice False",
			42,
			false,
		},
		{
			"IsSlice Nil",
			nil,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSlice(tt.input)

			if got != tt.expected {
				t.Errorf("IsSlice() = %v, want %v", got, tt.expected)
			}
		})
	}
}
