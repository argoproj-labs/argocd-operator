package util

import "testing"

func TestCombineImageTag(t *testing.T) {
	tests := []struct {
		name     string
		img      string
		tag      string
		expected string
	}{
		{
			"CombineImageTag Tag",
			"my-image",
			"latest",
			"my-image:latest",
		},
		{
			"CombineImageTag Digest",
			"my-image",
			"sha256:abc123",
			"my-image@sha256:abc123",
		},
		{
			"CombineImageTag NoTag",
			"my-image",
			"",
			"my-image",
		},
		{
			"CombineImageTag Tag With Colon",
			"my-image",
			"v1.0:20220101",
			"my-image@v1.0:20220101",
		},
		{
			"CombineImageTag Empty Image",
			"",
			"latest",
			"latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CombineImageTag(tt.img, tt.tag)

			if got != tt.expected {
				t.Errorf("CombineImageTag() = %v, want %v", got, tt.expected)
			}
		})
	}
}
