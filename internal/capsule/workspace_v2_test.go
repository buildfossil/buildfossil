package capsule

import (
	"fmt"
	"strings"
	"testing"
)

func validV2File(name string, size int64) FileMetadata {
	return FileMetadata{
		Path:   name,
		Size:   size,
		SHA256: "sha256:" + strings.Repeat("a", 64),
	}
}

func TestWorkspaceV2Validation(t *testing.T) {
	tests := []struct {
		name    string
		files   []FileMetadata
		wantErr bool
	}{
		{
			name: "two files",
			files: []FileMetadata{
				validV2File("src/main.go", 100),
				validV2File("go.mod", 50),
			},
		},
		{
			name: "duplicate paths",
			files: []FileMetadata{
				validV2File("go.mod", 50),
				validV2File("go.mod", 50),
			},
			wantErr: true,
		},
		{
			name: "parent traversal",
			files: []FileMetadata{
				validV2File("../secret.txt", 10),
			},
			wantErr: true,
		},
		{
			name: "absolute path",
			files: []FileMetadata{
				validV2File("/etc/passwd", 10),
			},
			wantErr: true,
		},
		{
			name: "file before child",
			files: []FileMetadata{
				validV2File("src", 10),
				validV2File("src/main.go", 20),
			},
			wantErr: true,
		},
		{
			name: "child before file",
			files: []FileMetadata{
				validV2File("src/main.go", 20),
				validV2File("src", 10),
			},
			wantErr: true,
		},
		{
			name: "too many files",
			files: func() []FileMetadata {
				var files []FileMetadata
				for i := 0; i < 65; i++ {
					files = append(files, validV2File(
						fmt.Sprintf("file-%d.txt", i), 1,
					))
				}
				return files
			}(),
			wantErr: true,
		},
		{
			name: "total size exceeded",
			files: []FileMetadata{
				validV2File("a.txt", 10<<20),
				validV2File("b.txt", 5<<20),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWorkspaceV2(Workspace{Files: tt.files})

			if (err != nil) != tt.wantErr {
				t.Fatalf("validation error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
