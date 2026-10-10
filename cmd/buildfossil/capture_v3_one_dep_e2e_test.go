package main

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/buildfossil/buildfossil/internal/capsule"
	"github.com/buildfossil/buildfossil/internal/replay"
)

func TestCaptureReplayV3OneDependencyEndToEnd(t *testing.T) {
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

	// Uses existing local ZIP/MOD/INFO fixture.
	// No network downloads are needed.
	fixture := makeCaptureV3Fixture(t)

	// Materialize the local module before Capture.
	// The fixture provides the ZIP, MOD and INFO artifacts.
	// No external network access is required.
	prepare := exec.Command("go", "mod", "download")
	prepare.Dir = fixture.workspace

	if output, err := prepare.CombinedOutput(); err != nil {
		t.Fatalf(
			"prepare offline Go dependency: %v\n%s",
			err,
			output,
		)
	}

	// Empty GOFLAGS is required for portable Capture.
	t.Setenv("GOFLAGS", "")

	captureExit := captureV3FixtureCommand(fixture)
	if captureExit != 1 {
		t.Fatalf("capture exit=%d, want 1", captureExit)
	}

	verified, err := capsule.ReadVerifiedV3(fixture.output)
	if err != nil {
		t.Fatalf("verify capsule: %v", err)
	}

	if len(verified.Manifest.GoModules) != 1 {
		t.Fatalf(
			"expected one Go module, got %d",
			len(verified.Manifest.GoModules),
		)
	}

	if len(verified.Artifacts) != 3 {
		t.Fatalf(
			"expected three module artifacts, got %d",
			len(verified.Artifacts),
		)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		120*time.Second,
	)
	defer cancel()

	result, err := replay.RunV3WithOptions(
		ctx,
		fixture.output,
		replay.Options{AllowArbitraryCommand: true},
	)
	if err != nil {
		t.Fatalf("offline Docker replay: %v", err)
	}

	if result.Outcome != replay.OutcomeReproduced {
		t.Fatalf(
			"outcome=%q, original=%d, replay=%d",
			result.Outcome,
			result.OriginalExitCode,
			result.ReplayExitCode,
		)
	}

	if result.OriginalExitCode != 1 || result.ReplayExitCode != 1 {
		t.Fatalf(
			"exit codes original=%d replay=%d, want 1/1",
			result.OriginalExitCode,
			result.ReplayExitCode,
		)
	}

	t.Log("schema v3 one-dependency offline replay reproduced")
}
