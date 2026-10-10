package capsule

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyStagedWorkspaceV3(t *testing.T) {
	manifest, files, artifacts := makeWorkspaceV3Fixture(t)

	var archive bytes.Buffer

	if err := WriteWorkspaceV3(
		&archive,
		manifest,
		files,
		artifacts,
	); err != nil {
		t.Fatalf("create v3 capsule: %v", err)
	}

	capsulePath := filepath.Join(t.TempDir(), "fixture.bfc")

	if err := os.WriteFile(
		capsulePath,
		archive.Bytes(),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	verified, err := ReadVerifiedV3(capsulePath)
	if err != nil {
		t.Fatalf("verify capsule: %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(t *testing.T, root string)
		wantErr bool
	}{
		{
			name: "valid workspace",
		},
		{
			name: "modified file",
			mutate: func(t *testing.T, root string) {
				t.Helper()
				if err := os.WriteFile(
					filepath.Join(root, "main.go"),
					[]byte("tampered\n"),
					0600,
				); err != nil {
					t.Fatal(err)
				}
			},
			wantErr: true,
		},
		{
			name: "extra file",
			mutate: func(t *testing.T, root string) {
				t.Helper()
				if err := os.WriteFile(
					filepath.Join(root, "unexpected.txt"),
					[]byte("extra"),
					0600,
				); err != nil {
					t.Fatal(err)
				}
			},
			wantErr: true,
		},
		{
			name: "missing file",
			mutate: func(t *testing.T, root string) {
				t.Helper()
				if err := os.Remove(
					filepath.Join(root, "main.go"),
				); err != nil {
					t.Fatal(err)
				}
			},
			wantErr: true,
		},
		{
			name: "symlink",
			mutate: func(t *testing.T, root string) {
				t.Helper()

				linkPath := filepath.Join(root, "main.go")

				if err := os.Remove(linkPath); err != nil {
					t.Fatal(err)
				}

				if err := os.Symlink(
					filepath.Join(root, "go.mod"),
					linkPath,
				); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			},
			wantErr: true,
		},
		{
			name: "extra directory",
			mutate: func(t *testing.T, root string) {
				t.Helper()
				if err := os.Mkdir(
					filepath.Join(root, "unexpected"),
					0700,
				); err != nil {
					t.Fatal(err)
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace := filepath.Join(t.TempDir(), "workspace")
			artifactsRoot := filepath.Join(t.TempDir(), "artifacts")

			if err := os.Mkdir(workspace, 0700); err != nil {
				t.Fatal(err)
			}

			if err := os.Mkdir(artifactsRoot, 0700); err != nil {
				t.Fatal(err)
			}

			if err := RestoreWorkspaceV3(
				workspace,
				artifactsRoot,
				verified,
			); err != nil {
				t.Fatalf("restore v3 workspace: %v", err)
			}

			if tt.mutate != nil {
				tt.mutate(t, workspace)
			}

			err := VerifyStagedWorkspaceV3(
				workspace,
				verified.Manifest,
			)

			if (err != nil) != tt.wantErr {
				t.Fatalf(
					"verification error=%v; wantErr=%t",
					err,
					tt.wantErr,
				)
			}
		})
	}
}
