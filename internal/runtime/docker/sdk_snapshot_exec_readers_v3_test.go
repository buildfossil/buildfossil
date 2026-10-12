package docker

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestSelectiveReaderExecutorRejectsNilReadersV3(t *testing.T) {
	runtime := &RuntimeSpec{
		Kind:    "go",
		Version: "1.26.6",
	}

	cases := []struct {
		name      string
		workspace io.Reader
		modules   io.Reader
	}{
		{
			name:    "nil workspace",
			modules: strings.NewReader("modules"),
		},
		{
			name:      "nil modules",
			workspace: strings.NewReader("workspace"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunWithSDKRuntimeSnapshotExecSelectiveReadersV3(
				context.Background(),
				tc.workspace,
				tc.modules,
				[]string{"go", "build", "./..."},
				nil,
				nil,
				0,
				"10001:10001",
				io.Discard,
				io.Discard,
				runtime,
			)
			if err == nil {
				t.Fatal("nil reader accepted")
			}
		})
	}
}
