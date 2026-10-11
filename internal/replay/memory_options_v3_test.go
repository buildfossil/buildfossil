package replay

import (
	"context"
	"strings"
	"testing"
)

func TestRunV3RejectsInvalidMemoryBeforeCapsuleRead(t *testing.T) {
	tests := []int64{
		-1,
		128 << 20,
		3 << 30,
	}

	for _, memory := range tests {
		_, err := RunV3WithOptions(
			context.Background(),
			"/nonexistent/buildfossil-test.bfc",
			Options{
				AllowArbitraryCommand: true,
				MemoryBytes:           memory,
			},
		)

		if err == nil {
			t.Fatalf("memory %d accepted", memory)
		}
		if !strings.Contains(err.Error(), "invalid v3 memory limit") {
			t.Fatalf("unexpected error for %d: %v", memory, err)
		}
	}
}
