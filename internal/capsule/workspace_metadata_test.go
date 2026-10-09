package capsule

import (
	"strings"
	"testing"
)

func TestWorkspaceValidate(t *testing.T) {
	validFile := func() FileMetadata {
		return FileMetadata{
			Path:   "fixture.txt",
			Size:   5,
			SHA256: SHA256([]byte("hello")),
		}
	}

	tests := []struct {
		name    string
		files   []FileMetadata
		wantErr bool
	}{
		{
			name: "valid metadata",
			files: []FileMetadata{
				validFile(),
			},
		},
		{
			name: "empty workspace",
		},
		{
			name: "too many files",
			files: []FileMetadata{
				validFile(),
				validFile(),
			},
			wantErr: true,
		},
		{
			name: "unsupported path",
			files: []FileMetadata{
				{
					Path:   "../secret",
					Size:   5,
					SHA256: SHA256([]byte("hello")),
				},
			},
			wantErr: true,
		},
		{
			name: "negative size",
			files: []FileMetadata{
				{
					Path:   "fixture.txt",
					Size:   -1,
					SHA256: SHA256([]byte("hello")),
				},
			},
			wantErr: true,
		},
		{
			name: "oversized file",
			files: []FileMetadata{
				{
					Path:   "fixture.txt",
					Size:   MaxWorkspaceFileSize + 1,
					SHA256: SHA256([]byte("hello")),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid SHA256 length",
			files: []FileMetadata{
				{
					Path:   "fixture.txt",
					Size:   5,
					SHA256: "sha256:abc",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid SHA256 characters",
			files: []FileMetadata{
				{
					Path:   "fixture.txt",
					Size:   5,
					SHA256: "sha256:" + strings.Repeat("z", 64),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := Workspace{Files: tt.files}

			err := w.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf(
					"Validate() error = %v, wantErr = %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}
