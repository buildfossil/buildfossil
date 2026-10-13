package capsule

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseGoSourceFilesV3SelectsMainModuleSources(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	subdir := filepath.Join(root, "internal", "worker")
	if err := os.MkdirAll(subdir, 0700); err != nil {
		t.Fatal(err)
	}

	packages := []map[string]any{
		{
			"Dir": root,
			"Module": map[string]any{
				"Main": true,
				"Dir":  root,
			},
			"GoFiles":        []string{"main.go", "platform_linux.go"},
			"IgnoredGoFiles": []string{"platform_darwin.go"},
			"TestGoFiles":    []string{"main_test.go"},
			"EmbedFiles":     []string{"assets/config.json"},
		},
		{
			"Dir": subdir,
			"Module": map[string]any{
				"Main": true,
				"Dir":  root,
			},
			"GoFiles": []string{"worker.go"},
		},
		{
			"Dir": "/external/module",
			"Module": map[string]any{
				"Main": false,
				"Dir":  "/external/module",
			},
			"GoFiles": []string{"external.go"},
		},
	}

	var input strings.Builder

	for _, pkg := range packages {
		data, err := json.Marshal(pkg)
		if err != nil {
			t.Fatal(err)
		}
		input.Write(data)
		input.WriteByte('\n')
	}

	got, err := ParseGoSourceFilesV3(
		strings.NewReader(input.String()),
		root,
	)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"internal/worker/worker.go",
		"main.go",
		"platform_linux.go",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sources = %q, want %q", got, want)
	}
}

func TestParseGoSourceFilesV3RejectsUnsafeMetadata(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	subdir := filepath.Join(root, "pkg")
	if err := os.MkdirAll(subdir, 0700); err != nil {
		t.Fatal(err)
	}

	outside := filepath.Join(filepath.Dir(root), "outside-package")

	tests := []struct {
		name     string
		packages []map[string]any
	}{
		{
			name: "parent traversal",
			packages: []map[string]any{
				{
					"Dir": root,
					"Module": map[string]any{
						"Main": true,
						"Dir":  root,
					},
					"GoFiles": []string{"../secret.go"},
				},
			},
		},
		{
			name: "absolute file",
			packages: []map[string]any{
				{
					"Dir": root,
					"Module": map[string]any{
						"Main": true,
						"Dir":  root,
					},
					"GoFiles": []string{"/tmp/secret.go"},
				},
			},
		},
		{
			name: "wrong main module root",
			packages: []map[string]any{
				{
					"Dir": root,
					"Module": map[string]any{
						"Main": true,
						"Dir":  outside,
					},
					"GoFiles": []string{"main.go"},
				},
			},
		},
		{
			name: "package outside workspace",
			packages: []map[string]any{
				{
					"Dir": outside,
					"Module": map[string]any{
						"Main": true,
						"Dir":  root,
					},
					"GoFiles": []string{"main.go"},
				},
			},
		},
		{
			name:     "no packages",
			packages: []map[string]any{},
		},
		{
			name: "only external modules",
			packages: []map[string]any{
				{
					"Dir": "/external/module",
					"Module": map[string]any{
						"Main": false,
						"Dir":  "/external/module",
					},
					"GoFiles": []string{"external.go"},
				},
			},
		},
		{
			name: "main module without sources",
			packages: []map[string]any{
				{
					"Dir": root,
					"Module": map[string]any{
						"Main": true,
						"Dir":  root,
					},
					"GoFiles": []string{},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var input strings.Builder

			for _, pkg := range tc.packages {
				data, err := json.Marshal(pkg)
				if err != nil {
					t.Fatal(err)
				}
				input.Write(data)
				input.WriteByte('\n')
			}

			got, err := ParseGoSourceFilesV3(
				strings.NewReader(input.String()),
				root,
			)
			if err == nil {
				t.Fatalf("accepted unsafe metadata: sources=%q", got)
			}
		})
	}
}

func TestParseGoSourceFilesV3RejectsSymlinkPackageDirectory(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	outside := t.TempDir()
	link := filepath.Join(root, "linked")

	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	pkg := map[string]any{
		"Dir": link,
		"Module": map[string]any{
			"Main": true,
			"Dir":  root,
		},
		"GoFiles": []string{"main.go"},
	}

	data, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}

	if got, err := ParseGoSourceFilesV3(
		strings.NewReader(string(data)),
		root,
	); err == nil {
		t.Fatalf("symlink package directory accepted: %q", got)
	}
}

func TestParseGoSourceFilesV3IncompleteMetadata(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		pkg     map[string]any
		wantErr bool
	}{
		{
			name: "missing import with known source",
			pkg: map[string]any{
				"Dir": root,
				"Module": map[string]any{
					"Main": true,
					"Dir":  root,
				},
				"GoFiles":    []string{"main.go"},
				"Incomplete": true,
				"DepsErrors": []map[string]any{
					{
						"Err": "cannot find module providing package",
					},
				},
			},
		},
		{
			name: "package discovery error",
			pkg: map[string]any{
				"Dir": root,
				"Module": map[string]any{
					"Main": true,
					"Dir":  root,
				},
				"GoFiles": []string{"main.go"},
				"Error": map[string]any{
					"Err": "cannot determine package files",
				},
			},
			wantErr: true,
		},
		{
			name: "incomplete package without source selection",
			pkg: map[string]any{
				"Dir": root,
				"Module": map[string]any{
					"Main": true,
					"Dir":  root,
				},
				"Incomplete": true,
				"DepsErrors": []map[string]any{
					{
						"Err": "dependency unavailable",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.pkg)
			if err != nil {
				t.Fatal(err)
			}

			got, err := ParseGoSourceFilesV3(
				strings.NewReader(string(data)),
				root,
			)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("accepted incomplete source metadata: %q", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("rejected known sources: %v", err)
			}

			if !reflect.DeepEqual(got, []string{"main.go"}) {
				t.Fatalf("sources = %q", got)
			}
		})
	}
}

func TestParseGoSourceFilesV3RejectsOversizedMetadata(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	pkg := map[string]any{
		"Dir": root,
		"Module": map[string]any{
			"Main": true,
			"Dir":  root,
		},
		"GoFiles": []string{"main.go"},
		"Unused":  strings.Repeat("x", int(MaxGoDependencyDiagnosticsSize)),
	}

	data, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ParseGoSourceFilesV3(
		strings.NewReader(string(data)), root,
	); err == nil {
		t.Fatal("oversized Go source metadata accepted")
	}
}

func TestParseGoSourceFilesV3AcceptsRootAlias(t *testing.T) {
	originalRoot := t.TempDir()
	physicalRoot, err := filepath.EvalSymlinks(originalRoot)
	if err != nil {
		t.Fatal(err)
	}

	// On macOS, a temporary directory can be accessible through
	// both /var and /private/var. The package metadata may report
	// either spelling of the same physical directory.
	aliasRoot := originalRoot
	if aliasRoot == physicalRoot {
		t.Skip("temporary root has no path alias")
	}

	packageDir := filepath.Join(aliasRoot, "pkg")
	if err := os.MkdirAll(packageDir, 0700); err != nil {
		t.Fatal(err)
	}

	pkg := map[string]any{
		"Dir": packageDir,
		"Module": map[string]any{
			"Main": true,
			"Dir":  aliasRoot,
		},
		"GoFiles": []string{"worker.go"},
	}

	data, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseGoSourceFilesV3(
		strings.NewReader(string(data)), physicalRoot,
	)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"pkg/worker.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sources = %q, want %q", got, want)
	}
}

func TestParseGoSourceFilesV3RejectsPartiallyDiscoveredMainModule(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	subdir := filepath.Join(root, "internal", "worker")
	if err := os.MkdirAll(subdir, 0700); err != nil {
		t.Fatal(err)
	}

	packages := []map[string]any{
		{
			"Dir": root,
			"Module": map[string]any{
				"Main": true,
				"Dir":  root,
			},
			"GoFiles": []string{"main.go"},
		},
		{
			"Dir": subdir,
			"Module": map[string]any{
				"Main": true,
				"Dir":  root,
			},
			"Incomplete": true,
			"DepsErrors": []map[string]any{
				{
					"Err": "package metadata unavailable",
				},
			},
		},
	}

	var input strings.Builder

	for _, pkg := range packages {
		data, err := json.Marshal(pkg)
		if err != nil {
			t.Fatal(err)
		}
		input.Write(data)
		input.WriteByte('\n')
	}

	got, err := ParseGoSourceFilesV3(
		strings.NewReader(input.String()),
		root,
	)

	if err == nil {
		t.Fatalf(
			"accepted partially discovered main module: sources=%q",
			got,
		)
	}
}
