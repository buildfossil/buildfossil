package docker

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/mod/sumdb/dirhash"
)

const (
	testGoModulePath    = "example.com/buildfossil/fixture"
	testGoModuleVersion = "v1.0.0"
)

type offlineGoFixture struct {
	workspace    string
	artifactsDir string
	cacheRoot    string
}

func makeOfflineGoFixture(t *testing.T) offlineGoFixture {
	t.Helper()

	moduleGoMod := []byte(
		"module " + testGoModulePath + "\n\ngo 1.26.0\n",
	)
	moduleSource := []byte(
		"package fixture\n\nfunc Value() string { return \"offline-ok\" }\n",
	)

	artifactsDir := t.TempDir()
	zipPath := filepath.Join(artifactsDir, testGoModuleVersion+".zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	zw := zip.NewWriter(f)

	for _, entry := range []struct {
		name string
		data []byte
	}{
		{
			testGoModulePath + "@" + testGoModuleVersion + "/go.mod",
			moduleGoMod,
		},
		{
			testGoModulePath + "@" + testGoModuleVersion + "/fixture.go",
			moduleSource,
		},
	} {
		w, err := zw.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	moduleHash, err := dirhash.HashZip(zipPath, dirhash.Hash1)
	if err != nil {
		t.Fatal(err)
	}

	modHash, err := dirhash.Hash1(
		[]string{"go.mod"},
		func(name string) (io.ReadCloser, error) {
			if name != "go.mod" {
				return nil, fmt.Errorf("unexpected file: %s", name)
			}
			return io.NopCloser(bytes.NewReader(moduleGoMod)), nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	for name, content := range map[string][]byte{
		testGoModuleVersion + ".mod":  moduleGoMod,
		testGoModuleVersion + ".info": []byte(`{"Version":"v1.0.0"}`),
	} {
		if err := os.WriteFile(
			filepath.Join(artifactsDir, name),
			content,
			0644,
		); err != nil {
			t.Fatal(err)
		}
	}

	workspace := t.TempDir()

	goMod := "module example.com/offline-sdk-test\n\n" +
		"go 1.26.0\n\nrequire " +
		testGoModulePath + " " + testGoModuleVersion + "\n"

	goSum := fmt.Sprintf(
		"%s %s %s\n%s %s/go.mod %s\n",
		testGoModulePath, testGoModuleVersion, moduleHash,
		testGoModulePath, testGoModuleVersion, modHash,
	)

	mainGo := `package main

import "example.com/buildfossil/fixture"

func main() {
	if fixture.Value() != "offline-ok" {
		panic("unexpected fixture value")
	}
}
`

	for name, content := range map[string]string{
		"go.mod":  goMod,
		"go.sum":  goSum,
		"main.go": mainGo,
	} {
		if err := os.WriteFile(
			filepath.Join(workspace, name),
			[]byte(content),
			0644,
		); err != nil {
			t.Fatal(err)
		}
	}

	cacheRoot := t.TempDir()
	cacheDir := filepath.Join(
		cacheRoot,
		"cache",
		"download",
		"example.com",
		"buildfossil",
		"fixture",
		"@v",
	)

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}

	for _, ext := range []string{".zip", ".mod", ".info"} {
		data, err := os.ReadFile(
			filepath.Join(artifactsDir, testGoModuleVersion+ext),
		)
		if err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(
			filepath.Join(cacheDir, testGoModuleVersion+ext),
			data,
			0644,
		); err != nil {
			t.Fatal(err)
		}
	}

	return offlineGoFixture{
		workspace:    workspace,
		artifactsDir: artifactsDir,
		cacheRoot:    cacheRoot,
	}
}
