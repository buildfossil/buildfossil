package docker

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/mod/sumdb/dirhash"
)

func TestSDKOfflineGoModuleBuild(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}

	const modulePath = "example.com/buildfossil/fixture"
	const moduleVersion = "v1.0.0"

	moduleGoMod := []byte("module " + modulePath + "\n\ngo 1.26.0\n")
	moduleSource := []byte(
		"package fixture\n\nfunc Value() string { return \"offline-ok\" }\n",
	)

	artifactsDir := t.TempDir()
	zipPath := filepath.Join(artifactsDir, moduleVersion+".zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	zw := zip.NewWriter(f)

	zipFiles := map[string][]byte{
		modulePath + "@" + moduleVersion + "/go.mod":     moduleGoMod,
		modulePath + "@" + moduleVersion + "/fixture.go": moduleSource,
	}

	for _, name := range []string{
		modulePath + "@" + moduleVersion + "/go.mod",
		modulePath + "@" + moduleVersion + "/fixture.go",
	} {
		entry, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(zipFiles[name]); err != nil {
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

	for name, content := range map[string]string{
		moduleVersion + ".mod":  string(moduleGoMod),
		moduleVersion + ".info": `{"Version":"v1.0.0"}`,
	} {
		if err := os.WriteFile(
			filepath.Join(artifactsDir, name),
			[]byte(content),
			0644,
		); err != nil {
			t.Fatal(err)
		}
	}

	workspace := t.TempDir()

	goMod := "module example.com/offline-sdk-test\n\n" +
		"go 1.26.0\n\nrequire " + modulePath + " " + moduleVersion + "\n"

	goSum := fmt.Sprintf(
		"%s %s %s\n%s %s/go.mod %s\n",
		modulePath, moduleVersion, moduleHash,
		modulePath, moduleVersion, modHash,
	)

	mainGo := `package main

import "example.com/buildfossil/fixture"

func main() {
	if fixture.Value() != "offline-ok" {
		panic("unexpected fixture value")
	}
}
`

	files := map[string]string{
		"go.mod":  goMod,
		"go.sum":  goSum,
		"main.go": mainGo,
	}

	for name, content := range files {
		if err := os.WriteFile(
			filepath.Join(workspace, name),
			[]byte(content),
			0644,
		); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		150*time.Second,
	)
	defer cancel()

	command := []string{
		"sh",
		"-c",
		"mkdir -p /gomodcache/cache/download/example.com/buildfossil/fixture/@v && " +
			"cp /module-artifacts/v1.0.0.zip " +
			"/module-artifacts/v1.0.0.mod " +
			"/module-artifacts/v1.0.0.info " +
			"/gomodcache/cache/download/example.com/buildfossil/fixture/@v/ && " +
			"go build -mod=readonly -o /tmp/offline-build .",
	}

	var stdout, stderr bytes.Buffer

	exitCode, err := RunWithSDKRuntimeModules(
		ctx,
		workspace,
		command,
		fmt.Sprintf("%d:%d", os.Geteuid(), os.Getegid()),
		&stdout,
		&stderr,
		&RuntimeSpec{
			Kind:    "go",
			Version: "1.26.6",
		},
		artifactsDir,
	)

	if err != nil {
		t.Fatalf(
			"offline SDK execution: %v\nstderr:\n%s",
			err,
			stderr.String(),
		)
	}

	if exitCode != 0 {
		t.Fatalf(
			"offline build exit code = %d\nstderr:\n%s",
			exitCode,
			stderr.String(),
		)
	}

	if strings.Contains(stderr.String(), "module lookup disabled") {
		t.Fatalf("offline module lookup failed:\n%s", stderr.String())
	}

	for name, original := range files {
		actual, err := os.ReadFile(filepath.Join(workspace, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(actual) != original {
			t.Errorf("workspace file modified: %s", name)
		}
	}
}
