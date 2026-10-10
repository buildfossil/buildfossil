package replay

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/buildfossil/buildfossil/internal/capsule"
	"github.com/buildfossil/buildfossil/internal/runtime/docker"
)

func goDependencyProbeArgs() []string {
	return []string{
		"go",
		"list",
		"-deps",
		"-e",
		"-json",
		"./...",
	}
}

func shouldProbeGoDependencies(m capsule.Manifest) bool {
	return m.Runtime != nil &&
		m.Runtime.Kind == "go" &&
		m.Runtime.Version == capsule.SupportedGoVersion &&
		m.GoBuildEnv != nil
}

// probeGoDependencies runs package-loading diagnostics inside the
// isolated Docker Go runtime. It does not classify replay outcomes.
func probeGoDependencies(
	ctx context.Context,
	workspace string,
	containerUser string,
	m capsule.Manifest,
) capsule.GoDependencyStatus {
	unknown := func(reason string) capsule.GoDependencyStatus {
		return capsule.GoDependencyStatus{
			State:  capsule.GoDependenciesUnknown,
			Reason: reason,
		}
	}

	if !shouldProbeGoDependencies(m) {
		return unknown("Go dependency probe is not applicable")
	}

	if err := ctx.Err(); err != nil {
		return unknown("Go dependency probe context canceled: " + err.Error())
	}

	runtimeSpec := &docker.RuntimeSpec{
		Kind:    m.Runtime.Kind,
		Version: m.Runtime.Version,
	}

	var stdout limitedDiagnosticWriter
	var err error
	var exitCode int

	if m.Platform.OS == "darwin" {
		exitCode, err = docker.RunWithSDKRuntimeTarget(
			ctx,
			workspace,
			goDependencyProbeArgs(),
			containerUser,
			&stdout,
			io.Discard,
			runtimeSpec,
			m.GoBuildEnv.GOOS,
			m.GoBuildEnv.GOARCH,
		)
	} else {
		exitCode, err = docker.RunWithSDKRuntime(
			ctx,
			workspace,
			goDependencyProbeArgs(),
			containerUser,
			&stdout,
			io.Discard,
			runtimeSpec,
		)
	}

	if err != nil {
		return unknown(fmt.Sprintf("Docker dependency probe failed: %v", err))
	}

	if exitCode != 0 {
		return unknown(fmt.Sprintf(
			"Go dependency probe exited with code %d",
			exitCode,
		))
	}

	if stdout.exceeded {
		return unknown("Go dependency probe output exceeded 8 MiB")
	}

	status, err := capsule.InspectGoListOutput(
		bytes.NewReader(stdout.data.Bytes()),
	)
	if err != nil {
		return unknown("Go dependency probe returned invalid output")
	}

	return status
}
