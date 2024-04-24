package util

import (
	"encoding/base64"
	"reflect"
	"testing"
)

func TestSplitList(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want []string
	}{
		{"Empty string", "", []string{}},
		{"Single element", "apple", []string{"apple"}},
		{"Comma-separated values", "apple,banana,orange", []string{"apple", "banana", "orange"}},
		{"Trim spaces", " apple , banana , orange ", []string{"apple", "banana", "orange"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitList(tt.s)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveString(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		s     string
		want  []string
	}{
		{"Remove from empty slice", []string{}, "apple", []string{}},
		{"Remove non-existing element", []string{"banana", "orange"}, "apple", []string{"banana", "orange"}},
		{"Remove existing element", []string{"apple", "banana", "orange"}, "banana", []string{"apple", "orange"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RemoveString(tt.slice, tt.s)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RemoveString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsString(t *testing.T) {
	tests := []struct {
		name string
		arr  []string
		s    string
		want bool
	}{
		{"Empty slice", []string{}, "apple", false},
		{"Element not present", []string{"banana", "orange"}, "apple", false},
		{"Element present", []string{"apple", "banana", "orange"}, "banana", true},
		{"Element with spaces", []string{" apple ", "banana", "orange "}, "apple", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsString(tt.arr, tt.s)
			if got != tt.want {
				t.Errorf("ContainsString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{"Equal slices", []string{"apple", "banana", "orange"}, []string{"apple", "banana", "orange"}, true},
		{"Different order", []string{"apple", "banana", "orange"}, []string{"banana", "orange", "apple"}, true},
		{"Different elements", []string{"apple", "banana", "orange"}, []string{"apple", "grape", "orange"}, false},
		{"Different lengths", []string{"apple", "banana", "orange"}, []string{"apple", "banana"}, false},
		{"Empty slices", []string{}, []string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Equal(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("Equal() = %v, want %v", got, tt.want)
			}
		})
	}

	// Additional test for ensuring sorting doesn't affect the result
	t.Run("Sorting doesn't affect equality", func(t *testing.T) {
		a := []string{"apple", "banana", "orange"}
		b := []string{"banana", "orange", "apple"}
		Equal(a, b)
		if !reflect.DeepEqual(a, []string{"apple", "banana", "orange"}) ||
			!reflect.DeepEqual(b, []string{"banana", "orange", "apple"}) {
			t.Errorf("Sorting should not affect the input slices")
		}
	})
}

func TestGenerateRandomString(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"Zero length", 0},
		{"Positive length", 10},
		{"Long length", 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateRandomString(tt.length)
			if err != nil {
				t.Errorf("GenerateRandomString() error = %v", err)
				return
			}

			decoded, err := base64.URLEncoding.DecodeString(got)
			if err != nil {
				t.Errorf("Error decoding base64 string: %v", err)
			}

			// Check if the decoded string has the expected length
			if len(decoded) != tt.length {
				t.Errorf("GenerateRandomString() length = %v, want %v", len(decoded), tt.length)
			}
		})
	}
}

func TestConstructString(t *testing.T) {
	tests := []struct {
		name      string
		separator string
		parts     []string
		want      string
	}{
		{"DotSep", DotSep, []string{"a", "b", "c"}, "a.b.c"},
		{"UnderscoreSep", UnderscoreSep, []string{"a", "b", "c"}, "a_b_c"},
		{"Single char", UnderscoreSep, []string{"a"}, "a"},
		{"Empty parts", DotSep, []string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConstructString(tt.separator, tt.parts...)
			if got != tt.want {
				t.Errorf("ConstructStrings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringPtr(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"Simple string", "apple"},
		{"Emty string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StringPtr(tt.value)
			if *got != tt.value {
				t.Errorf("StringPtr() = %v", got)
			}
		})
	}
}
