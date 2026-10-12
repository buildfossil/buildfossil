package capsule

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDiscoverGoEmbedFilesV3Integration(t *testing.T) {
	root := t.TempDir()

	files := map[string]string{
		"go.mod": "module example.com/embed-fixture\n\ngo 1.26\n",
		"main.go": `package main

import _ "embed"

//go:embed assets/message.txt
var message string

func main() {
	buildfossilIntentionallyUndefined()
}
`,
		"assets/message.txt": "hello from embedded resource\n",
	}

	for name, contents := range files {
		path := filepath.Join(root, filepath.FromSlash(name))

		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}

	cacheCommand := exec.Command("go", "env", "GOMODCACHE")
	cacheCommand.Dir = root

	output, err := cacheCommand.Output()
	if err != nil {
		t.Fatalf("resolve module cache: %v", err)
	}

	moduleCache := string(output)
	moduleCache = strings.TrimSpace(moduleCache)

	got, err := DiscoverGoEmbedFilesV3(root, moduleCache)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"assets/message.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("embedded files = %v, want %v", got, want)
	}
}
