package argoutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

// TestIsHostRunningInFipsMode validates the FIPS check function that checks if the host is running in FIPS mode.
func TestIsHostRunningInFipsMode(t *testing.T) {
	// --- Setup ---
	// Create a temporary directory for our test files.
	tmpDir, err := os.MkdirTemp("", "fips-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir for FIPS test: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	// Case 1: FIPS enabled file ("1")
	fipsEnabledFile := filepath.Join(tmpDir, "fips_enabled_on")
	if err := os.WriteFile(fipsEnabledFile, []byte("1"), 0644); err != nil {
		t.Fatalf("Failed to write FIPS enabled file: %v", err)
	}

	// Case 2: FIPS disabled file ("0")
	fipsDisabledFile := filepath.Join(tmpDir, "fips_enabled_off")
	if err := os.WriteFile(fipsDisabledFile, []byte("0"), 0644); err != nil {
		t.Fatalf("Failed to write FIPS disabled file: %v", err)
	}

	// Case 3: FIPS file does not exist (we'll use a non-existent path)
	nonExistentFile := filepath.Join(tmpDir, "non_existent_fips_file")

	// Case 4: Cannot read FIPS file (permission error)
	unreadableFile := filepath.Join(tmpDir, "unreadable_fips_file")
	if err := os.WriteFile(unreadableFile, []byte("1"), 0000); err != nil {
		t.Fatalf("Failed to write unreadable file: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(unreadableFile, 0644); err != nil {
			t.Logf("Warning: could not restore permissions for cleanup: %v", err)
		}
	}) // Cleanup permissions

	// --- Test Cases Table ---
	testCases := []struct {
		name         string
		filePath     string // The path we will point our global var to.
		wantFipsMode bool
		wantErr      bool
	}{
		{
			name:         "FIPS is Enabled",
			filePath:     fipsEnabledFile,
			wantFipsMode: true,
			wantErr:      false,
		},
		{
			name:         "FIPS is Disabled",
			filePath:     fipsDisabledFile,
			wantFipsMode: false,
			wantErr:      false,
		},
		{
			name:         "FIPS file does not exist",
			filePath:     nonExistentFile,
			wantFipsMode: false,
			wantErr:      false,
		},
		{
			name:         "Cannot read FIPS file due to permissions",
			filePath:     unreadableFile,
			wantFipsMode: false,
			wantErr:      true,
		},
	}

	// --- Run Tests ---
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Point the global variable to our test file.
			checker := &LinuxFipsConfigChecker{ConfigFilePath: tc.filePath}

			gotFipsMode, gotErr := checker.IsFipsEnabled()

			if (gotErr != nil) != tc.wantErr {
				t.Errorf("IsHostRunningInFipsMode() error = %v, wantErr %v", gotErr, tc.wantErr)
				return
			}

			if gotFipsMode != tc.wantFipsMode {
				t.Errorf("IsHostRunningInFipsMode() gotFipsMode = %v, want %v", gotFipsMode, tc.wantFipsMode)
			}
		})
	}
}

func TestDecorateWithFIPSEnv(t *testing.T) {
	t.Run("Empty input adds GODEBUG and GOLANG_FIPS", func(t *testing.T) {
		result := DecorateWithFIPSEnv(nil)
		assert.Len(t, result, 2)
		assert.Contains(t, result, corev1.EnvVar{Name: "GODEBUG", Value: "fips140=on"})
		assert.Contains(t, result, corev1.EnvVar{Name: "GOLANG_FIPS", Value: "0"})
	})

	t.Run("No GODEBUG in input adds both vars and preserves existing", func(t *testing.T) {
		input := []corev1.EnvVar{
			{Name: "HOME", Value: "/home/argocd"},
		}
		result := DecorateWithFIPSEnv(input)
		assert.Len(t, result, 3)
		assert.Contains(t, result, corev1.EnvVar{Name: "GODEBUG", Value: "fips140=on"})
		assert.Contains(t, result, corev1.EnvVar{Name: "GOLANG_FIPS", Value: "0"})
		assert.Contains(t, result, corev1.EnvVar{Name: "HOME", Value: "/home/argocd"})
	})

	t.Run("Custom GODEBUG without fips140=on is preserved and GOLANG_FIPS not added", func(t *testing.T) {
		input := []corev1.EnvVar{
			{Name: "GODEBUG", Value: "http2debug=1"},
		}
		result := DecorateWithFIPSEnv(input)
		assert.Len(t, result, 1)
		assert.Contains(t, result, corev1.EnvVar{Name: "GODEBUG", Value: "http2debug=1"})
		assert.NotContains(t, result, corev1.EnvVar{Name: "GOLANG_FIPS", Value: "0"})
	})

	t.Run("Custom GODEBUG with fips140=off is preserved and GOLANG_FIPS not added", func(t *testing.T) {
		input := []corev1.EnvVar{
			{Name: "GODEBUG", Value: "fips140=off"},
		}
		result := DecorateWithFIPSEnv(input)
		assert.Len(t, result, 1)
		assert.Contains(t, result, corev1.EnvVar{Name: "GODEBUG", Value: "fips140=off"})
		assert.NotContains(t, result, corev1.EnvVar{Name: "GOLANG_FIPS", Value: "0"})
	})

	t.Run("Custom GODEBUG with fips140=on adds GOLANG_FIPS", func(t *testing.T) {
		input := []corev1.EnvVar{
			{Name: "GODEBUG", Value: "fips140=on"},
		}
		result := DecorateWithFIPSEnv(input)
		assert.Len(t, result, 2)
		assert.Contains(t, result, corev1.EnvVar{Name: "GODEBUG", Value: "fips140=on"})
		assert.Contains(t, result, corev1.EnvVar{Name: "GOLANG_FIPS", Value: "0"})
	})

	t.Run("Custom GODEBUG with fips140=on among other settings adds GOLANG_FIPS", func(t *testing.T) {
		input := []corev1.EnvVar{
			{Name: "GODEBUG", Value: "http2debug=1,fips140=on,tls13=1"},
		}
		result := DecorateWithFIPSEnv(input)
		assert.Len(t, result, 2)
		assert.Contains(t, result, corev1.EnvVar{Name: "GODEBUG", Value: "http2debug=1,fips140=on,tls13=1"})
		assert.Contains(t, result, corev1.EnvVar{Name: "GOLANG_FIPS", Value: "0"})
	})

	t.Run("Existing GOLANG_FIPS is not overridden", func(t *testing.T) {
		input := []corev1.EnvVar{
			{Name: "GOLANG_FIPS", Value: "1"},
		}
		result := DecorateWithFIPSEnv(input)
		assert.Len(t, result, 2)
		assert.Contains(t, result, corev1.EnvVar{Name: "GODEBUG", Value: "fips140=on"})
		assert.Contains(t, result, corev1.EnvVar{Name: "GOLANG_FIPS", Value: "1"})
	})

	t.Run("Custom GODEBUG without fips140 and existing GOLANG_FIPS both preserved", func(t *testing.T) {
		input := []corev1.EnvVar{
			{Name: "GODEBUG", Value: "http2debug=1"},
			{Name: "GOLANG_FIPS", Value: "1"},
		}
		result := DecorateWithFIPSEnv(input)
		assert.Len(t, result, 2)
		assert.Contains(t, result, corev1.EnvVar{Name: "GODEBUG", Value: "http2debug=1"})
		assert.Contains(t, result, corev1.EnvVar{Name: "GOLANG_FIPS", Value: "1"})
	})

	t.Run("Substring fips140=onward does not trigger GOLANG_FIPS", func(t *testing.T) {
		input := []corev1.EnvVar{
			{Name: "GODEBUG", Value: "fips140=onward"},
		}
		result := DecorateWithFIPSEnv(input)
		assert.Len(t, result, 1)
		assert.Contains(t, result, corev1.EnvVar{Name: "GODEBUG", Value: "fips140=onward"})
		assert.NotContains(t, result, corev1.EnvVar{Name: "GOLANG_FIPS", Value: "0"})
	})
}

// TestFileExists runs a series of table-driven tests to validate the fileExists function.
func TestFileExists(t *testing.T) {
	// --- Test Case 1: A file that actually exists ---
	// Create a temporary file that will be cleaned up after the test.
	tmpFile, err := os.CreateTemp("", "testfile-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	// Use t.Cleanup to ensure the file is removed even if the test panics.
	t.Cleanup(func() {
		os.Remove(tmpFile.Name())
	})
	tmpFile.Close() // Close the file handle

	// --- Test Case 2: A path that is a directory ---
	// Create a temporary directory that will be cleaned up after the test.
	tmpDir, err := os.MkdirTemp("", "testdir-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	// --- Test Case 4: A path where we lack permission ---
	// Create a directory, and a file inside it. Then remove permissions from the parent directory
	// so we can't 'stat' the file within it, triggering a permission error.
	permDir, err := os.MkdirTemp("", "perm-dir-*")
	if err != nil {
		t.Fatalf("Failed to create permission test dir: %v", err)
	}
	t.Cleanup(func() {
		// We need to re-add permissions to be able to delete it.
		if err := os.Chmod(permDir, 0755); err != nil {
			t.Logf("Warning: could not restore permissions for cleanup: %v", err)
		}
		if err := os.RemoveAll(permDir); err != nil {
			t.Logf("Warning: could not remove permission test dir: %v", err)
		}
	})

	permFile := filepath.Join(permDir, "unreachable.txt")
	if _, err := os.Create(permFile); err != nil {
		t.Fatalf("Failed to create permission test file: %v", err)
	}
	// On Unix-like systems, removing execute permission from a directory
	// prevents accessing its contents.
	if err := os.Chmod(permDir, 0400); err != nil {
		t.Logf("Warning: could not change directory permissions for permission test: %v", err)
	}

	// Define the test cases in a table for clarity.
	testCases := []struct {
		name       string // A descriptive name for the test case.
		path       string // The input path for the fileExists function.
		wantExists bool   // The expected boolean result.
		wantErr    bool   // Whether we expect an error.
	}{
		{
			name:       "File Exists",
			path:       tmpFile.Name(),
			wantExists: true,
			wantErr:    false,
		},
		{
			name:       "File Does Not Exist",
			path:       "this-file-definitely-does-not-exist.tmp",
			wantExists: false,
			wantErr:    false,
		},
		{
			name:       "Path is a Directory",
			path:       tmpDir,
			wantExists: false,
			wantErr:    false,
		},
		{
			name:       "Permission Error",
			path:       permFile,
			wantExists: false,
			wantErr:    true,
		},
	}

	// Iterate over the test cases.
	for _, tc := range testCases {
		// t.Run allows running subtests, which gives clearer output.
		t.Run(tc.name, func(t *testing.T) {
			// Execute the function under test.
			gotExists, gotErr := fileExists(tc.path)

			// Check if the error status matches our expectation.
			if (gotErr != nil) != tc.wantErr {
				// If we got an error but didn't expect one, or vice versa.
				t.Errorf("fileExists() error = %v, wantErr %v", gotErr, tc.wantErr)
				return // Stop this subtest if the error status is wrong.
			}

			// Check if the 'exists' boolean matches our expectation.
			if gotExists != tc.wantExists {
				t.Errorf("fileExists() gotExists = %v, want %v", gotExists, tc.wantExists)
			}
		})
	}
}
