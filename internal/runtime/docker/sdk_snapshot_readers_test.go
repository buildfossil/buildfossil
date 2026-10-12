package docker

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestSnapshotReadersRejectNilInputs(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		workspace io.Reader
		modules   io.Reader
	}{
		{"nil workspace", nil, strings.NewReader("module")},
		{"nil modules", strings.NewReader("workspace"), nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CreateSnapshotVolumesFromReaders(
				ctx, nil, "image", tc.workspace, tc.modules,
			)
			if err == nil {
				t.Fatal("expected input validation error")
			}
		})
	}
}

func TestSnapshotReadersDoNotRequireByteSlices(t *testing.T) {
	reader := io.MultiReader(
		strings.NewReader("first"),
		bytes.NewBufferString("-second"),
	)
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "first-second" {
		t.Fatalf("unexpected stream: %q", data)
	}
}
