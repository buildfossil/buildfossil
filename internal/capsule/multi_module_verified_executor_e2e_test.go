package capsule

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	dockerruntime "github.com/buildfossil/buildfossil/internal/runtime/docker"
)

func TestMultiModuleVerifiedExecutorRejectsGraphMismatch(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	verified := makeVerifiedMultiModuleSnapshotV3(t)

	workspaceTAR, err := BuildWorkspaceSnapshotV3(verified)
	if err != nil {
		t.Fatal(err)
	}
	moduleTAR, err := BuildGoModuleSnapshotV3(verified)
	if err != nil {
		t.Fatal(err)
	}

	expected := make([]dockerruntime.ExpectedGoModule, 0, len(verified.Manifest.GoModules))
	for _, mod := range verified.Manifest.GoModules {
		expected = append(expected, dockerruntime.ExpectedGoModule{
			Path: mod.Path, Version: mod.Version,
		})
	}

	const marker = "BUILDFOSSIL_COMMAND_WAS_EXECUTED"

	tests := []struct {
		name     string
		expected []dockerruntime.ExpectedGoModule
		reject   bool
	}{
		{
			name:     "matching graph permits command",
			expected: expected,
		},
		{
			name: "wrong version blocks command",
			expected: []dockerruntime.ExpectedGoModule{
				{Path: "example.com/a", Version: "v1.0.0"},
				{Path: "example.com/b", Version: "v9.0.0"},
			},
			reject: true,
		},
		{
			name: "missing module blocks command",
			expected: []dockerruntime.ExpectedGoModule{
				{Path: "example.com/a", Version: "v1.0.0"},
			},
			reject: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(
				context.Background(), 120*time.Second,
			)
			defer cancel()

			var stdout, stderr bytes.Buffer

			code, err := dockerruntime.RunWithSDKRuntimeSnapshotExecVerified(
				ctx,
				workspaceTAR,
				moduleTAR,
				[]string{
					"/bin/sh", "-c",
					"printf 'BUILDFOSSIL_COMMAND_WAS_EXECUTED\n' >&2; exit 23",
				},
				tc.expected,
				"10001:10001",
				&stdout,
				&stderr,
				&dockerruntime.RuntimeSpec{
					Kind: "go", Version: "1.26.6",
				},
			)

			if tc.reject {
				if err == nil {
					t.Fatal("graph mismatch was accepted")
				}
				if !strings.Contains(err.Error(), "graph") {
					t.Fatalf("unexpected rejection: %v", err)
				}
				if stdout.Len() != 0 || stderr.Len() != 0 {
					t.Fatalf(
						"verification leaked command output: stdout=%q stderr=%q",
						stdout.String(), stderr.String(),
					)
				}
				return
			}

			if err != nil {
				t.Fatalf("matching graph rejected: %v", err)
			}
			if code != 23 || stderr.String() != marker+"\n" {
				t.Fatalf("command output mismatch: exit=%d stderr=%q", code, stderr.String())
			}
		})
	}
}
