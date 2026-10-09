package docker

import (
	"errors"
	"strings"
	"testing"
)

func TestCombineCleanupError(t *testing.T) {
	runErr := errors.New("execution failed")
	cleanupErr := errors.New("container removal failed")

	t.Run("successful cleanup", func(t *testing.T) {
		got := combineCleanupError(runErr, nil, "test-container")

		if !errors.Is(got, runErr) {
			t.Fatalf("original execution error was lost: %v", got)
		}
	})

	t.Run("cleanup failure", func(t *testing.T) {
		got := combineCleanupError(nil, cleanupErr, "test-container")

		if !errors.Is(got, cleanupErr) {
			t.Fatalf("cleanup error was lost: %v", got)
		}

		if !strings.Contains(got.Error(), "test-container") {
			t.Fatalf("container ID missing from error: %v", got)
		}
	})

	t.Run("execution and cleanup failure", func(t *testing.T) {
		got := combineCleanupError(runErr, cleanupErr, "test-container")

		if !errors.Is(got, runErr) {
			t.Fatalf("execution error was lost: %v", got)
		}

		if !errors.Is(got, cleanupErr) {
			t.Fatalf("cleanup error was lost: %v", got)
		}
	})

	t.Run("no errors", func(t *testing.T) {
		got := combineCleanupError(nil, nil, "test-container")

		if got != nil {
			t.Fatalf("expected nil; got %v", got)
		}
	})
}
