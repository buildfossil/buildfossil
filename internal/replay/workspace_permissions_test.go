package replay

import (
	"os"
	"testing"
)

func TestPrepareLegacyReplayWorkspace(t *testing.T) {
	workspace, err := os.MkdirTemp("", "buildfossil-permissions-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workspace)

	if err := prepareLegacyReplayWorkspace(workspace); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(workspace)
	if err != nil {
		t.Fatal(err)
	}

	if got := info.Mode().Perm(); got != 0755 {
		t.Fatalf("workspace mode = %04o, want 0755", got)
	}
}
