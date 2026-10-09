package capsule

import (
	"io/fs"
	"strings"
	"testing"
)

func TestValidateWorkspacePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		mode    fs.FileMode
		wantErr bool
	}{
		{"regular file", "fixture.txt", 0644, false},
		{"nested file", "src/main.go", 0644, false},
		{"executable", "bin/test", 0755, false},
		{"empty path", "", 0644, true},
		{"dot path", ".", 0644, true},
		{"absolute path", "/etc/passwd", 0644, true},
		{"parent traversal", "../secret", 0644, true},
		{"nested traversal", "src/../../secret", 0644, true},
		{"hidden traversal", "src/../secret", 0644, true},
		{"double slash", "src//main.go", 0644, true},
		{"dot component", "src/./main.go", 0644, true},
		{"backslash", `src\main.go`, 0644, true},
		{"NUL byte", "src/\x00file", 0644, true},
		{"symlink", "link", fs.ModeSymlink | 0777, true},
		{"directory", "src", fs.ModeDir | 0755, true},
		{"named pipe", "pipe", fs.ModeNamedPipe | 0600, true},
		{"device", "device", fs.ModeDevice | 0600, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkspacePath(tt.path, tt.mode)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkspacePath(%q, %v) = %v; wantErr=%v",
					tt.path, tt.mode, err, tt.wantErr)
			}
		})
	}
}

func TestValidateWorkspacePathRejectsDeepTraversal(t *testing.T) {
	name := strings.Repeat("nested/", 100) + "../../secret"

	if err := ValidateWorkspacePath(name, 0644); err == nil {
		t.Fatal("expected deep traversal path to be rejected")
	}
}
