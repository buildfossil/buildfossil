package replay

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotV3ReadersRemainBoundAfterPathReplacement(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(*testing.T) *snapshotFilesV3
	}{
		{
			name: "legacy",
			prepare: func(t *testing.T) *snapshotFilesV3 {
				t.Helper()
				prepared, err := prepareSnapshotFilesV3(
					verifiedSnapshotFilesFixtureV3(t),
				)
				if err != nil {
					t.Fatal(err)
				}
				return prepared
			},
		},
		{
			name: "disk-backed-store",
			prepare: func(t *testing.T) *snapshotFilesV3 {
				t.Helper()
				_, stored := storedSnapshotFixtureV3(t)
				prepared, err := prepareSnapshotFilesFromStoreV3(stored)
				if err != nil {
					t.Fatal(err)
				}
				return prepared
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prepared := tc.prepare(t)
			t.Cleanup(func() {
				if err := prepared.Close(); err != nil {
					t.Errorf("cleanup: %v", err)
				}
			})

			files := []struct {
				name   string
				reader *os.File
			}{
				{"workspace.tar", prepared.workspace},
				{"modules.tar", prepared.modules},
			}

			for _, item := range files {
				t.Run(item.name, func(t *testing.T) {
					original, err := io.ReadAll(item.reader)
					if err != nil {
						t.Fatal(err)
					}
					if len(original) == 0 {
						t.Fatal("empty original snapshot")
					}

					if _, err := item.reader.Seek(0, io.SeekStart); err != nil {
						t.Fatal(err)
					}

					pathname := filepath.Join(prepared.directory, item.name)

					// Rename the original pathname, leaving its inode
					// available through the already-open file descriptor.
					moved := pathname + ".original"
					if err := os.Rename(pathname, moved); err != nil {
						t.Fatal(err)
					}

					replacement := []byte("attacker-controlled-replacement")
					if err := os.WriteFile(pathname, replacement, 0600); err != nil {
						t.Fatal(err)
					}

					current, err := os.ReadFile(pathname)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(current, replacement) {
						t.Fatal("replacement setup failed")
					}

					actual, err := io.ReadAll(item.reader)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(actual, original) {
						t.Fatal("reader followed replaced pathname")
					}
					if bytes.Equal(actual, replacement) {
						t.Fatal("reader consumed attacker replacement")
					}
				})
			}
		})
	}
}
