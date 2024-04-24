package util

import (
	"reflect"
	"testing"
)

func TestSetDiff(t *testing.T) {
	tests := []struct {
		name     string
		mapA     map[string]string
		mapB     map[string]string
		expected map[string]string
	}{
		{
			"Basic Diff",
			map[string]string{"key1": "value1", "key2": "value2"},
			map[string]string{"key2": "value2", "key3": "value3"},
			map[string]string{"key1": ""},
		},
		{
			"Empty Maps",
			map[string]string{},
			map[string]string{},
			map[string]string{},
		},
		{
			"Extracting All Elements (B Contains A)",
			map[string]string{"key1": "value1", "key2": "value2"},
			map[string]string{"key1": "value3", "key2": "value4", "key3": "value4"},
			map[string]string{},
		},
		{
			"Maps with Different Types",
			map[string]string{"key1": "value1", "key2": "value2"},
			map[string]string{"key3": "value3", "key4": "value4"},
			map[string]string{"key1": "", "key2": ""},
		},
		{
			"Substracting from Empty Map A",
			map[string]string{},
			map[string]string{"key1": "value1", "key2": "value2"},
			map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SetDiff(tt.mapA, tt.mapB)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("SetDiff() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSetIntersection(t *testing.T) {
	tests := []struct {
		name     string
		mapA     map[string]string
		mapB     map[string]string
		expected map[string]string
	}{
		{
			"Basic Intersection",
			map[string]string{"key1": "value1", "key2": "value2"},
			map[string]string{"key2": "value2", "key3": "value3"},
			map[string]string{"key2": ""},
		},
		{
			"Empty Maps",
			map[string]string{},
			map[string]string{},
			map[string]string{},
		},
		{
			"Extracting All Elements (B Contains A)",
			map[string]string{"key1": "value1", "key2": "value2"},
			map[string]string{"key1": "value3", "key2": "value4", "key3": "value4"},
			map[string]string{"key1": "", "key2": ""},
		},
		{
			"No Match",
			map[string]string{"key1": "value1", "key2": "value2"},
			map[string]string{"key3": "value3", "key4": "value4"},
			map[string]string{},
		},
		{
			"Intersection with Empty Map A",
			map[string]string{},
			map[string]string{"key1": "value1", "key2": "value2"},
			map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SetIntersection(tt.mapA, tt.mapB)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("SetIntersection() = %v, want %v", got, tt.expected)
			}
		})
	}
}
