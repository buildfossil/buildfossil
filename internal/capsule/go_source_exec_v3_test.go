package capsule

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoverGoSourceFilesV3BothPlatforms(t *testing.T) {
	root := t.TempDir()

	files := map[string]string{
		"go.mod":              "module example.com/source-discovery\n\ngo 1.26.0\n",
		"main.go":             "package main\nfunc main() { undefinedFunction() }\n",
		"platform_linux.go":   "package main\nvar platform = \"linux\"\n",
		"platform_darwin.go":  "package main\nvar platform = \"darwin\"\n",
		"platform_windows.go": "package main\nvar platform = \"windows\"\n",
		"main_test.go":        "package main\n",
	}

	for name, content := range files {
		if err := os.WriteFile(
			filepath.Join(root, name),
			[]byte(content),
			0600,
		); err != nil {
			t.Fatal(err)
		}
	}

	cacheCommand := exec.Command("go", "env", "GOMODCACHE")
	cacheCommand.Dir = root

	cacheOutput, err := cacheCommand.Output()
	if err != nil {
		t.Fatal(err)
	}

	moduleCacheRoot := string(cacheOutput)
	for len(moduleCacheRoot) > 0 &&
		(moduleCacheRoot[len(moduleCacheRoot)-1] == '\n' ||
			moduleCacheRoot[len(moduleCacheRoot)-1] == '\r') {
		moduleCacheRoot = moduleCacheRoot[:len(moduleCacheRoot)-1]
	}

	got, err := DiscoverGoSourceFilesV3(
		root,
		moduleCacheRoot,
		GoBuildEnvironment{
			GOOS:   "darwin",
			GOARCH: "arm64",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"main.go",
		"platform_darwin.go",
		"platform_linux.go",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discovered sources = %q, want %q", got, want)
	}
}

func TestDiscoverGoSourceFilesV3MultiplePackages(t *testing.T) {
	root := t.TempDir()

	for _, directory := range []string{
		"internal/worker",
		"pkg/config",
	} {
		if err := os.MkdirAll(
			filepath.Join(root, filepath.FromSlash(directory)),
			0700,
		); err != nil {
			t.Fatal(err)
		}
	}

	files := map[string]string{
		"go.mod": "module example.com/multipackage\n\ngo 1.26.0\n",

		"main.go": `package main

import (
	_ "example.com/multipackage/internal/worker"
	_ "example.com/multipackage/pkg/config"
)

func main() {
	missingFunction()
}
`,

		"internal/worker/worker.go": `package worker
func Work() {}
`,

		"pkg/config/config.go": `package config
func Config() {}
`,

		"pkg/config/platform_linux.go": `package config
var Platform = "linux"
`,

		"pkg/config/platform_darwin.go": `package config
var Platform = "darwin"
`,

		"pkg/config/platform_windows.go": `package config
var Platform = "windows"
`,

		"pkg/config/config_test.go": `package config
func TestOnly() {}
`,

		".env": "SECRET_SHOULD_NOT_BE_DISCOVERED=1\n",
	}

	for name, content := range files {
		if err := os.WriteFile(
			filepath.Join(root, filepath.FromSlash(name)),
			[]byte(content),
			0600,
		); err != nil {
			t.Fatal(err)
		}
	}

	cacheCommand := exec.Command("go", "env", "GOMODCACHE")
	cacheCommand.Dir = root

	cacheOutput, err := cacheCommand.Output()
	if err != nil {
		t.Fatal(err)
	}

	moduleCacheRoot := string(cacheOutput)
	for len(moduleCacheRoot) > 0 &&
		(moduleCacheRoot[len(moduleCacheRoot)-1] == '\n' ||
			moduleCacheRoot[len(moduleCacheRoot)-1] == '\r') {
		moduleCacheRoot = moduleCacheRoot[:len(moduleCacheRoot)-1]
	}

	got, err := DiscoverGoSourceFilesV3(
		root,
		moduleCacheRoot,
		GoBuildEnvironment{
			GOOS:   "darwin",
			GOARCH: "arm64",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"internal/worker/worker.go",
		"main.go",
		"pkg/config/config.go",
		"pkg/config/platform_darwin.go",
		"pkg/config/platform_linux.go",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf(
			"discovered sources = %q, want %q",
			got, want,
		)
	}
}
