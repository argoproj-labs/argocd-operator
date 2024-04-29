package util

import (
	"os"
	"testing"
)

func TestLoadTemplateFile(t *testing.T) {
	t.Run("Simple Template", func(t *testing.T) {
		testfile, err := os.CreateTemp("", "testing")
		if err != nil {
			t.Errorf("Error creating temporary file: %v", err)
		}
		defer func(name string) {
			err := os.Remove(name)
			if err != nil {
				t.Errorf("Error removing temporary file: %v", err)
			}
		}(testfile.Name())

		_, err = testfile.Write([]byte("Day and time entered: {{.Day}}, {{.Time}}."))

		if err != nil {
			t.Errorf("Error wriing to temporary file: %v", err)
		}

		err = testfile.Close()
		if err != nil {
			t.Errorf("Error closing temporary file: %v", err)
		}

		params := map[string]string{
			"Day":  "Monday",
			"Time": "12.00",
		}

		result, err := LoadTemplateFile(testfile.Name(), params)

		if err != nil {
			t.Errorf("LoadTemplateFile() error = %v", err)
		}

		expected := "Day and time entered: Monday, 12.00."

		if result != expected {
			t.Errorf("LoadTemplateFile() result = %v, want %v", err, expected)
		}
	})
	t.Run("Non-existent File", func(t *testing.T) {
		params := map[string]string{
			"Day":  "Monday",
			"Time": "12.00",
		}

		result, err := LoadTemplateFile("some_path", params)

		if err == nil {
			t.Errorf("LoadTemplateFile() should throw error because of non-existent template file")
		}

		expected := ""

		if result != expected {
			t.Errorf("LoadTemplateFile() result = %v, want %v", err, expected)
		}
	})
}
