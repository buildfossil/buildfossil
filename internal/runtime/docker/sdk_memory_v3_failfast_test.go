package docker

import (
	"context"
	"strings"
	"testing"
)

func TestVerifiedExecutorRejectsInvalidMemoryBeforeDocker(t *testing.T) {
	_, err := RunWithSDKRuntimeSnapshotExecVerifiedWithMemory(
		context.Background(),
		[]byte("workspace"),
		[]byte("modules"),
		[]string{"go", "build", "./..."},
		nil,
		3<<30,
		"10001:10001",
		nil,
		nil,
		&RuntimeSpec{Kind: "go", Version: "1.26.6"},
	)

	if err == nil {
		t.Fatal("invalid memory accepted")
	}
	if !strings.Contains(err.Error(), "unsupported v3 replay memory limit") {
		t.Fatalf("unexpected error: %v", err)
	}
}
