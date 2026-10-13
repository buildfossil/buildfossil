package replay

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestSonicRealWorldOfflineReplayV3 validates a captured failure
// from the real github.com/bytedance/sonic v1.14.1 project.
//
// Requires an explicitly supplied, trusted local capsule.
func TestSonicRealWorldOfflineReplayV3(t *testing.T) {
	if os.Getenv("REW_DOCKER_TEST") != "1" {
		t.Skip("Docker integration tests disabled")
	}

	filename := os.Getenv("REW_SONIC_CAPSULE")
	if filename == "" {
		t.Skip("Sonic capsule not configured")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		4*time.Minute,
	)
	defer cancel()

	result, err := RunV3WithOptions(
		ctx,
		filename,
		Options{
			AllowArbitraryCommand: true,
			MemoryBytes:           1 << 30,
		},
	)
	if err != nil {
		t.Fatalf("real-world Sonic replay: %v", err)
	}

	if result.Outcome != OutcomeReproduced {
		t.Fatalf(
			"unexpected outcome: %q, want %q",
			result.Outcome,
			OutcomeReproduced,
		)
	}

	if result.OriginalExitCode != 1 || result.ReplayExitCode != 1 {
		t.Fatalf(
			"exit code mismatch: original=%d replay=%d",
			result.OriginalExitCode,
			result.ReplayExitCode,
		)
	}

	t.Logf(
		"SONIC REAL-WORLD OFFLINE REPLAY: %s (original=%d replay=%d)",
		result.Outcome,
		result.OriginalExitCode,
		result.ReplayExitCode,
	)
}
