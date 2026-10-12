package capsule

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceReadSchemaFileLimits(t *testing.T) {
	root := t.TempDir()

	paths := make([]string, MaxWorkspaceV3Files+1)
	for i := range paths {
		name := fmt.Sprintf("file-%03d.go", i)
		paths[i] = name
		if err := os.WriteFile(filepath.Join(root, name), []byte("package fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct {
		name    string
		count   int
		read    func(string, []string) ([]WorkspaceFile, error)
		wantErr bool
	}{
		{"v2 accepts 64", 64, ReadWorkspaceFilesV2, false},
		{"v2 rejects 65", 65, ReadWorkspaceFilesV2, true},
		{"v3 accepts 65", 65, ReadWorkspaceFilesV3, false},
		{"v3 accepts 256", 256, ReadWorkspaceFilesV3, false},
		{"v3 accepts 257", 257, ReadWorkspaceFilesV3, false},
		{"v3 accepts 2048", 2048, ReadWorkspaceFilesV3, false},
		{"v3 rejects 2049", 2049, ReadWorkspaceFilesV3, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files, err := tc.read(root, paths[:tc.count])
			if (err != nil) != tc.wantErr {
				t.Fatalf("read error = %v, wantErr %v", err, tc.wantErr)
			}
			if err == nil && len(files) != tc.count {
				t.Fatalf("got %d files, want %d", len(files), tc.count)
			}
		})
	}
}
