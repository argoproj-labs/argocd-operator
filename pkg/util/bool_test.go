package util

import (
	"testing"
)

func TestBoolPtr(t *testing.T) {
	tests := []struct {
		name  string
		value bool
	}{
		{"True", true},
		{"False", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BoolPtr(tt.value)
			if *got != tt.value {
				t.Errorf("BoolPtr() = %v", got)
			}
		})
	}
}
