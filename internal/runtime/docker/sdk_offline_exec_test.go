package docker

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestOfflineExecRejectsInvalidUser(t *testing.T) {
	tests := []string{
		"",
		"0",
		"0:0",
		"0:20",
		"root",
		"root:root",
		"501",
		"501:staff",
		"-1:20",
		"501:-1",
		"invalid:20",
	}

	for _, user := range tests {
		t.Run(strings.ReplaceAll(user, ":", "_"), func(t *testing.T) {
			_, err := RunWithSDKRuntimeOfflineExec(
				context.Background(),
				"/workspace",
				[]string{"go", "version"},
				user,
				io.Discard,
				io.Discard,
				&RuntimeSpec{
					Kind:    "go",
					Version: "1.26.6",
				},
				"/module-cache",
			)

			if err == nil {
				t.Fatalf("invalid user %q accepted", user)
			}
			if !strings.Contains(err.Error(), "UID") &&
				!strings.Contains(err.Error(), "GID") {
				t.Fatalf(
					"user %q rejected for wrong reason: %v",
					user, err,
				)
			}
		})
	}
}
