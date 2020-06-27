package config

import (
	"strings"
	"testing"
)

func TestSetupConfigDefaults(t *testing.T) {
	if _, err := SetupConfigDefaults(); err != nil {
		t.Errorf("Test Failed. error: %s", err.Error())
	}
}

func TestReadConfiguration(t *testing.T) {
	if err := readConfiguration(); err != nil {
		t.Errorf("Test Failed. error: %s", err.Error())
	}
}

func TestOsConfigDirectory(t *testing.T) {
	var tests = []struct {
		input    string
		expected string
	}{
		{"linux", ".config"},
		{"darwin", "Application Support"},
	}
	for _, test := range tests {
		if output := osConfigDirectory(test.input); !strings.Contains(output, test.expected) {
			t.Errorf("Test Failed. %s inputted, output: %s, expected %s", test.input, output, test.expected)
		}
	}

}
