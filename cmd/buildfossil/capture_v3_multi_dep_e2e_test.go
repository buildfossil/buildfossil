package main

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/buildfossil/buildfossil/internal/capsule"
	"github.com/buildfossil/buildfossil/internal/replay"
	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
)

func TestCaptureReplayV3TwoDependenciesEndToEnd(t *testing.T) {
	if os.Getenv("BUILDFOSSIL_DOCKER_TEST") != "1" {
		t.Skip("Docker integration test disabled")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("requires macOS or Linux")
	}
	if os.Geteuid() == 0 {
		t.Skip("Docker replay requires non-root execution")
	}

	t.Setenv("GOOS", "linux")
	t.Setenv("GOARCH", "amd64")
	t.Setenv("CGO_ENABLED", "0")
	t.Setenv("GOWORK", "off")
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOSUMDB", "off")
	t.Setenv("GOFLAGS", "")

	fixture := makeCaptureV3TwoDependenciesFixture(t)

	// Capture must not require network access to collect artifacts.
	code := captureV3FixtureCommand(fixture)
	if code != 1 {
		t.Fatalf("capture exit=%d, want 1", code)
	}

	verified, err := capsule.ReadVerifiedV3(fixture.output)
	if err != nil {
		t.Fatalf("read captured capsule: %v", err)
	}
	if len(verified.Manifest.GoModules) != 2 {
		t.Fatalf(
			"captured %d modules, want 2",
			len(verified.Manifest.GoModules),
		)
	}
	if len(verified.Artifacts) != 6 {
		t.Fatalf(
			"captured %d artifacts, want 6",
			len(verified.Artifacts),
		)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(), 180*time.Second,
	)
	defer cancel()

	result, err := replay.RunV3WithOptions(
		ctx,
		fixture.output,
		replay.Options{AllowArbitraryCommand: true},
	)
	if err != nil {
		t.Fatalf("Docker offline replay: %v", err)
	}
	if result.Outcome != replay.OutcomeReproduced {
		t.Fatalf(
			"outcome=%q, original=%d, replay=%d",
			result.Outcome,
			result.OriginalExitCode,
			result.ReplayExitCode,
		)
	}
	if result.OriginalExitCode != 1 ||
		result.ReplayExitCode != 1 {
		t.Fatalf(
			"exit codes=%d/%d, want 1/1",
			result.OriginalExitCode,
			result.ReplayExitCode,
		)
	}

	t.Log("TWO-MODULE CAPTURE -> DOCKER REPLAY: reproduced")
}

func makeCaptureV3TwoDependenciesFixture(t *testing.T) captureV3Fixture {
	t.Helper()
	// Existing fixture supplies the first real cached module.
	fixture := makeCaptureV3Fixture(t)

	const secondPath = "example.com/buildfossil/second"
	const version = "v1.0.0"

	modData := []byte(
		"module " + secondPath + "\n\ngo 1.26.0\n",
	)
	sourceData := []byte(
		"package second\n\nfunc Value() int { return 42 }\n",
	)

	var zipBuffer bytes.Buffer
	zw := zip.NewWriter(&zipBuffer)

	for _, entry := range []struct {
		name string
		data []byte
	}{
		{secondPath + "@" + version + "/go.mod", modData},
		{secondPath + "@" + version + "/second.go", sourceData},
	} {
		writer, err := zw.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	zipPath := filepath.Join(t.TempDir(), "second.zip")
	if err := os.WriteFile(zipPath, zipBuffer.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}

	zipHash, err := dirhash.HashZip(zipPath, dirhash.Hash1)
	if err != nil {
		t.Fatal(err)
	}

	modHash, err := dirhash.Hash1(
		[]string{"go.mod"},
		func(name string) (io.ReadCloser, error) {
			if name != "go.mod" {
				return nil, fmt.Errorf("unexpected file %q", name)
			}
			return io.NopCloser(bytes.NewReader(modData)), nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	escapedPath, err := module.EscapePath(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	escapedVersion, err := module.EscapeVersion(version)
	if err != nil {
		t.Fatal(err)
	}

	cacheDir := filepath.Join(
		fixture.cacheRoot,
		"cache", "download",
		filepath.FromSlash(escapedPath),
		"@v",
	)
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		t.Fatal(err)
	}

	for _, artifact := range []struct {
		ext  string
		data []byte
	}{
		{".zip", zipBuffer.Bytes()},
		{".mod", modData},
		{".info", []byte(`{"Version":"v1.0.0"}`)},
	} {
		name := filepath.Join(
			cacheDir, escapedVersion+artifact.ext,
		)
		if err := os.WriteFile(name, artifact.data, 0600); err != nil {
			t.Fatal(err)
		}
	}

	// Extend the first fixture with the second dependency.
	goMod := []byte(`module example.com/capture-v3-fixture

go 1.26.0

require (
	example.com/buildfossil/fixture v1.0.0
	example.com/buildfossil/second v1.0.0
)
`)
	goSum := append([]byte(nil), fixture.goSum...)
	goSum = append(goSum, []byte(fmt.Sprintf(
		"%s %s %s\n%s %s/go.mod %s\n",
		secondPath, version, zipHash,
		secondPath, version, modHash,
	))...)

	mainGo := []byte(`package main

import (
	"example.com/buildfossil/fixture"
	"example.com/buildfossil/second"
)

func main() {
	_ = fixture.Value()
	_ = second.Value()
	missingSymbol()
}
`)

	for name, data := range map[string][]byte{
		"go.mod":  goMod,
		"go.sum":  goSum,
		"main.go": mainGo,
	} {
		if err := os.WriteFile(
			filepath.Join(fixture.workspace, name),
			data, 0600,
		); err != nil {
			t.Fatal(err)
		}
	}

	// Materialize cached modules without network access.
	prepare := exec.Command("go", "mod", "download")
	prepare.Dir = fixture.workspace
	if output, err := prepare.CombinedOutput(); err != nil {
		t.Fatalf("offline module preparation: %v\n%s", err, output)
	}

	// Restore portable Capture flags after preparing the writable cache.
	t.Setenv("GOFLAGS", "")

	return fixture
}
