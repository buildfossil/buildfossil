package capsule

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestVerifiedV3ReadersRejectCorruptedTarTermination verifies
// rejection of damaged or truncated schema-v3 TAR archives in
// both the legacy and disk-backed capsule verification paths.
func TestVerifiedV3ReadersRejectCorruptedTarTermination(t *testing.T) {
	originalPath := storedReaderFixtureV3(t)

	original, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(original) < 1024 {
		t.Fatal("fixture is too small for TAR termination blocks")
	}

	// Confirm the fixture contains a conventional zero trailer.
	if !bytes.Equal(original[len(original)-1024:], make([]byte, 1024)) {
		t.Fatal("fixture does not end in two zero TAR blocks")
	}

	tests := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{
			name: "corrupted final byte",
			mutate: func(data []byte) []byte {
				data[len(data)-1] = 0xff
				return data
			},
		},
		{
			name: "corrupted first termination block",
			mutate: func(data []byte) []byte {
				data[len(data)-1024] = 0xff
				return data
			},
		},
		{
			name: "truncated final block",
			mutate: func(data []byte) []byte {
				return data[:len(data)-512]
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			corrupted := tc.mutate(bytes.Clone(original))

			path := filepath.Join(t.TempDir(), "invalid.bfc")
			if err := os.WriteFile(path, corrupted, 0600); err != nil {
				t.Fatal(err)
			}

			t.Run("legacy-v3", func(t *testing.T) {
				if _, err := ReadVerifiedV3(path); err == nil {
					t.Fatal("SECURITY FAILURE: invalid TAR accepted")
				}
			})

			t.Run("disk-backed-v3", func(t *testing.T) {
				stored, err := ReadVerifiedToStoreV3(path)
				if err == nil {
					_ = stored.Close()
					t.Fatal("SECURITY FAILURE: invalid TAR accepted")
				}
			})
		})
	}
}
