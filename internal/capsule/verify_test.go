package capsule

import "testing"

func TestVerifyWorkspaceFile(t *testing.T) {
	original := []byte("BUILD_FOSSIL_TEST_FAILURE\n")

	valid := FileMetadata{
		Path:   "fixture.txt",
		Size:   int64(len(original)),
		SHA256: SHA256(original),
	}

	tests := []struct {
		name    string
		modify  func(*FileMetadata)
		data    []byte
		wantErr bool
	}{
		{
			name: "valid file",
			data: original,
		},
		{
			name:    "modified content",
			data:    []byte("BUILD_FOSSIL_TEST_SUCCESS\n"),
			wantErr: true,
		},
		{
			name: "incorrect size",
			modify: func(m *FileMetadata) {
				m.Size++
			},
			data:    original,
			wantErr: true,
		},
		{
			name: "unsupported path",
			modify: func(m *FileMetadata) {
				m.Path = "../secret.txt"
			},
			data:    original,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := valid

			if tt.modify != nil {
				tt.modify(&metadata)
			}

			err := VerifyWorkspaceFile(metadata, tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf(
					"VerifyWorkspaceFile() error = %v, wantErr = %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}
