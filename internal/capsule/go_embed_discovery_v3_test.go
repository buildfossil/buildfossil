package capsule

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGoEmbedFilesV3ValidAndDeduplicated(t *testing.T) {
	root := t.TempDir()
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(
		filepath.Join(root, "internal", "assets"), 0700,
	); err != nil {
		t.Fatal(err)
	}

	pkg := map[string]any{
		"Dir": filepath.Join(canonicalRoot, "internal", "assets"),
		"Module": map[string]any{
			"Main": true,
			"Dir":  canonicalRoot,
		},
		"EmbedFiles": []string{
			"tests/a.toml",
			"tests/b.toml",
			"tests/a.toml",
		},
	}

	data, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseGoEmbedFilesV3(
		strings.NewReader(string(data)), root,
	)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"internal/assets/tests/a.toml",
		"internal/assets/tests/b.toml",
	}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestParseGoEmbedFilesV3RejectsUnsafePaths(t *testing.T) {
	root := t.TempDir()

	tests := []struct {
		name string
		file string
	}{
		{"parent traversal", "../secret.txt"},
		{"nested traversal", "assets/../secret.txt"},
		{"absolute path", "/etc/passwd"},
		{"dot component", "./secret.txt"},
		{"empty path", ""},
		{"backslash", `assets\secret.txt`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pkg := map[string]any{
				"Dir": filepath.Join(root, "internal"),
				"Module": map[string]any{
					"Main": true,
					"Dir":  root,
				},
				"EmbedFiles": []string{tc.file},
			}

			data, err := json.Marshal(pkg)
			if err != nil {
				t.Fatal(err)
			}

			_, err = ParseGoEmbedFilesV3(
				strings.NewReader(string(data)), root,
			)
			if err == nil {
				t.Fatalf(
					"unsafe embedded path %q was accepted",
					tc.file,
				)
			}
		})
	}
}

func TestParseGoEmbedFilesV3RejectsOutsidePackage(t *testing.T) {
	root := t.TempDir()

	pkg := map[string]any{
		"Dir": filepath.Join(root, "..", "outside"),
		"Module": map[string]any{
			"Main": true,
			"Dir":  root,
		},
		"EmbedFiles": []string{"secret.txt"},
	}

	data, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ParseGoEmbedFilesV3(
		strings.NewReader(string(data)), root,
	); err == nil {
		t.Fatal("package outside workspace accepted")
	}
}

func TestParseGoEmbedFilesV3RejectsWrongModuleRoot(t *testing.T) {
	root := t.TempDir()

	pkg := map[string]any{
		"Dir": root,
		"Module": map[string]any{
			"Main": true,
			"Dir":  filepath.Join(root, "other"),
		},
		"EmbedFiles": []string{"secret.txt"},
	}

	data, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ParseGoEmbedFilesV3(
		strings.NewReader(string(data)), root,
	); err == nil {
		t.Fatal("unexpected main module root accepted")
	}
}

func TestParseGoEmbedFilesV3RejectsIncompletePackage(t *testing.T) {
	root := t.TempDir()

	pkg := map[string]any{
		"Dir":        root,
		"Incomplete": true,
		"Module": map[string]any{
			"Main": true,
			"Dir":  root,
		},
		"EmbedFiles": []string{"data.txt"},
	}

	data, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ParseGoEmbedFilesV3(
		strings.NewReader(string(data)), root,
	); err == nil {
		t.Fatal("incomplete package accepted")
	}
}

func TestParseGoEmbedFilesV3IgnoresExternalModules(t *testing.T) {
	root := t.TempDir()
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(
		filepath.Join(root, "internal"), 0700,
	); err != nil {
		t.Fatal(err)
	}

	packages := []map[string]any{
		{
			"Dir": filepath.Join(canonicalRoot, "internal"),
			"Module": map[string]any{
				"Main": true,
				"Dir":  canonicalRoot,
			},
			"EmbedFiles": []string{"safe.txt"},
		},
		{
			"Dir": "/external/module",
			"Module": map[string]any{
				"Main": false,
				"Dir":  "/external/module",
			},
			"EmbedFiles": []string{"secret.txt"},
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

	got, err := ParseGoEmbedFilesV3(strings.NewReader(input.String()), root)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 || got[0] != "internal/safe.txt" {
		t.Fatalf("unexpected embedded files: %v", got)
	}
}

func TestParseGoEmbedFilesV3RejectsMalformedJSON(t *testing.T) {
	root := t.TempDir()

	cases := []string{
		"",
		"{",
		`{"Dir":`,
		`{"Dir":"abc"} {broken`,
	}

	for _, input := range cases {
		if _, err := ParseGoEmbedFilesV3(
			strings.NewReader(input), root,
		); err == nil {
			t.Fatalf("invalid JSON accepted: %q", input)
		}
	}
}

func TestParseGoEmbedFilesV3RejectsTooManyFiles(t *testing.T) {
	root := t.TempDir()

	names := make([]string, MaxWorkspaceV3Files+1)
	for i := range names {
		names[i] = fmt.Sprintf("file-%04d.txt", i)
	}

	pkg := map[string]any{
		"Dir": root,
		"Module": map[string]any{
			"Main": true,
			"Dir":  root,
		},
		"EmbedFiles": names,
	}

	data, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ParseGoEmbedFilesV3(
		strings.NewReader(string(data)), root,
	); err == nil {
		t.Fatal("more than 2048 embedded files accepted")
	}
}

func TestParseGoEmbedFilesV3RejectsOversizedJSON(t *testing.T) {
	root := t.TempDir()

	input := `{"Dir":"` +
		strings.Repeat("x", int(MaxGoDependencyDiagnosticsSize)) +
		`"}`

	if _, err := ParseGoEmbedFilesV3(
		strings.NewReader(input), root,
	); err == nil {
		t.Fatal("oversized JSON accepted")
	}
}

func TestParseGoEmbedFilesV3RejectsSymlinkPackageDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	link := filepath.Join(root, "linked-package")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	pkg := map[string]any{
		"Dir": link,
		"Module": map[string]any{
			"Main": true,
			"Dir":  root,
		},
		"EmbedFiles": []string{"secret.txt"},
	}

	data, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ParseGoEmbedFilesV3(
		strings.NewReader(string(data)), root,
	); err == nil {
		t.Fatal("symlink package directory accepted")
	}
}

func TestParseGoEmbedFilesV3RejectsNoMainModulePackages(t *testing.T) {
	root := t.TempDir()

	pkg := map[string]any{
		"Dir": "/external/module",
		"Module": map[string]any{
			"Main": false,
			"Dir":  "/external/module",
		},
		"EmbedFiles": []string{"data.txt"},
	}

	data, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ParseGoEmbedFilesV3(
		strings.NewReader(string(data)), root,
	); err == nil {
		t.Fatal("metadata without main-module packages accepted")
	}
}

func TestParseGoEmbedFilesV3AllowsNoEmbeddedFiles(t *testing.T) {
	root := t.TempDir()
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}

	pkg := map[string]any{
		"Dir": canonicalRoot,
		"Module": map[string]any{
			"Main": true,
			"Dir":  canonicalRoot,
		},
		"EmbedFiles": []string{},
	}

	data, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseGoEmbedFilesV3(
		strings.NewReader(string(data)), root,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 0 {
		t.Fatalf("expected no embedded files, got %v", got)
	}
}

func TestParseGoEmbedFilesV3AcceptsRootAliasRegression(t *testing.T) {
	root := t.TempDir()
	physicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}

	alias := filepath.Join(t.TempDir(), "workspace-alias")
	if err := os.Symlink(physicalRoot, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	pkg := map[string]any{
		"Dir": alias,
		"Module": map[string]any{
			"Main": true,
			"Dir":  alias,
		},
		"EmbedFiles": []string{"assets/message.txt"},
	}

	data, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseGoEmbedFilesV3(
		strings.NewReader(string(data)), physicalRoot,
	)
	if err != nil {
		t.Fatalf("valid workspace alias rejected: %v", err)
	}
	if len(got) != 1 || got[0] != "assets/message.txt" {
		t.Fatalf("unexpected paths: %q", got)
	}
}

func TestParseGoEmbedFilesV3RejectsNestedSymlinkWithRootAlias(t *testing.T) {
	root := t.TempDir()
	physicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}

	realPackage := filepath.Join(root, "real-package")
	if err := os.MkdirAll(realPackage, 0700); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(
		realPackage, filepath.Join(root, "linked-package"),
	); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	alias := filepath.Join(t.TempDir(), "workspace-alias")
	if err := os.Symlink(physicalRoot, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	pkg := map[string]any{
		"Dir": filepath.Join(alias, "linked-package"),
		"Module": map[string]any{
			"Main": true,
			"Dir":  alias,
		},
		"EmbedFiles": []string{"message.txt"},
	}

	data, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ParseGoEmbedFilesV3(
		strings.NewReader(string(data)), physicalRoot,
	); err == nil {
		t.Fatal("nested symlink accepted through workspace alias")
	}
}
