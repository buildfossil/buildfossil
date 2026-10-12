package capsule

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDiscoverGoEmbedFilesV3BothPlatforms(t *testing.T) {
	root := t.TempDir()

	files := map[string]string{
		"go.mod":  "module example.com/embed-platform\n\ngo 1.26\n",
		"main.go": "package main\nfunc main() {}\n",
		"embed_darwin.go": `package main
import _ "embed"
//go:embed assets/darwin.txt
var darwinResource string
`,
		"embed_linux.go": `package main
import _ "embed"
//go:embed assets/linux.txt
var linuxResource string
`,
		"assets/darwin.txt": "macOS resource\n",
		"assets/linux.txt":  "Linux resource\n",
	}

	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command("go", "env", "GOMODCACHE")
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}

	cache := strings.TrimSpace(string(output))

	got, err := DiscoverGoEmbedFilesV3(
		root,
		cache,
		GoBuildEnvironment{GOOS: "darwin", GOARCH: "arm64"},
	)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"assets/darwin.txt",
		"assets/linux.txt",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("embedded files = %q, want %q", got, want)
	}
}
