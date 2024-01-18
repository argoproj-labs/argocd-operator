package util

import (
	"testing"
	"time"
)

func TestNowBytes(t *testing.T) {
	t.Run("Test NowBytes", func(t *testing.T) {
		got := NowBytes()
		nowStr := time.Now().UTC().Format(time.RFC3339)
		expected := []byte(nowStr)

		if !bytesEqual(got, expected) {
			t.Errorf("NowBytes() = %v, want %v", got, expected)
		}
	})
}

// Helper function to check equality of two byte slices
func bytesEqual(a, b []byte) bool {
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
