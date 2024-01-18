package util

import (
	"reflect"
	"sort"
	"testing"
)

func TestMergeMaps(t *testing.T) {
	tests := []struct {
		name     string
		mapA     map[string]string
		mapB     map[string]string
		expected map[string]string
	}{
		{
			"Basic Merge",
			map[string]string{"key1": "value1", "key2": "value2"},
			map[string]string{"key2": "new_value2", "key3": "value3"},
			map[string]string{"key1": "value1", "key2": "new_value2", "key3": "value3"},
		},
		{
			"Empty Maps",
			map[string]string{},
			map[string]string{},
			map[string]string{},
		},
		{
			"Conflict Resolution (B Overwrites A)",
			map[string]string{"key1": "value1", "key2": "value2"},
			map[string]string{"key2": "new_value2", "key1": "new_value1"},
			map[string]string{"key1": "new_value1", "key2": "new_value2"},
		},
		{
			"Maps with Different Types",
			map[string]string{"key1": "value1", "key2": "value2"},
			map[string]string{"key3": "value3", "key4": "value4"},
			map[string]string{"key1": "value1", "key2": "value2", "key3": "value3", "key4": "value4"},
		},
		{
			"Map B Overrides Empty Map A",
			map[string]string{},
			map[string]string{"key1": "value1", "key2": "value2"},
			map[string]string{"key1": "value1", "key2": "value2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeMaps(tt.mapA, tt.mapB)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("MergeMaps() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestStringMapKeys(t *testing.T) {
	tests := []struct {
		name     string
		inputMap map[string]string
		expected []string
	}{
		{
			"Basic String Keys",
			map[string]string{"key3": "value3", "key1": "value1", "key2": "value2"},
			[]string{"key1", "key2", "key3"},
		},
		{
			"Empty Map",
			map[string]string{},
			[]string{},
		},
		{
			"Map with Single Key",
			map[string]string{"single_key": "value"},
			[]string{"single_key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StringMapKeys(tt.inputMap)

			sort.Strings(got) // Sorting for consistent comparison

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("StringMapKeys() = %v, want %v", got, tt.expected)
			}
		})
	}
}
