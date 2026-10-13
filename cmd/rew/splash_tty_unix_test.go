//go:build darwin || linux

package main

import (
	"os"
	"testing"
)

func TestSplashRejectsRegularFile(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "splash-*")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if splashIsTerminal(file) {
		t.Fatal("regular file incorrectly detected as terminal")
	}
}

func TestSplashRejectsNilFile(t *testing.T) {
	if splashIsTerminal(nil) {
		t.Fatal("nil file incorrectly detected as terminal")
	}
}

func TestSplashColorEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		variable string
		value    string
	}{
		{"no color", "NO_COLOR", "1"},
		{"clicolor disabled", "CLICOLOR", "0"},
		{"dumb terminal", "TERM", "dumb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", "")
			t.Setenv("CLICOLOR", "1")
			t.Setenv("TERM", "xterm-256color")
			t.Setenv(tt.variable, tt.value)

			if splashSupportsColor(os.Stdout) {
				t.Fatal("color should be disabled")
			}
		})
	}
}
