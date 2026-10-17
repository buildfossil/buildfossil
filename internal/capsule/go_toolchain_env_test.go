package capsule

import (
	"strings"
	"testing"
)

func TestLocalGoToolchainEnv(t *testing.T) {
	t.Setenv("GOTOOLCHAIN", "auto")
	t.Setenv("REW_TEST_ENV", "preserved")

	env := LocalGoToolchainEnv()
	count := 0
	foundOther := false

	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			t.Fatalf("malformed environment entry: %q", entry)
		}

		if key == "GOTOOLCHAIN" {
			count++
			if value != "local" {
				t.Fatalf("GOTOOLCHAIN = %q, want local", value)
			}
		}

		if key == "REW_TEST_ENV" && value == "preserved" {
			foundOther = true
		}
	}

	if count != 1 {
		t.Fatalf("GOTOOLCHAIN entries = %d, want 1", count)
	}
	if !foundOther {
		t.Fatal("unrelated environment variable was lost")
	}
}
