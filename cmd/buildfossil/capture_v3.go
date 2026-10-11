package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/buildfossil/buildfossil/internal/capsule"
	"github.com/buildfossil/buildfossil/internal/execution"
)

func captureCommandV3(
	argv []string,
	workspaceRoot string,
	includes []string,
	output string,
) int {
	if len(argv) == 0 || len(includes) == 0 {
		fmt.Fprintln(os.Stderr,
			"buildfossil: v3 requires a command and --include files")
		return 2
	}

	if err := capsule.ValidateGoWorkspaceV3(workspaceRoot); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"buildfossil: Go workspace: %v\n",
			err,
		)
		return 125
	}

	detectedRuntime := capsule.DetectRuntime(argv, workspaceRoot)

	if detectedRuntime == nil ||
		detectedRuntime.Kind != "go" ||
		detectedRuntime.Version != capsule.SupportedGoVersion {
		fmt.Fprintln(os.Stderr,
			"buildfossil: v3 requires the supported Go runtime")
		return 125
	}

	goBuildEnv, err := capsule.InspectGoBuildEnvironment(workspaceRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"buildfossil: inspect Go environment: %v\n", err)
		return 125
	}

	if err := goBuildEnv.ValidatePortableGoBuild(); err != nil {
		fmt.Fprintf(os.Stderr,
			"buildfossil: unsupported portable Go environment: %v\n", err)
		return 125
	}

	if err := checkCaptureOutputAvailableV3(output); err != nil {
		fmt.Fprintf(os.Stderr,
			"buildfossil: capture output: %v\n", err)
		return 125
	}

	beforeFiles, err := capsule.ReadWorkspaceFilesV2(
		workspaceRoot,
		includes,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"buildfossil: read workspace before execution: %v\n", err)
		return 125
	}

	result, err := execution.RunInDir(
		argv,
		workspaceRoot,
		os.Stdout,
		os.Stderr,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"buildfossil: execute command: %v\n", err)
		return 125
	}

	if result.ExitCode == 0 {
		return 0
	}

	if result.Stderr == "" || result.StderrTruncated {
		fmt.Fprintln(os.Stderr,
			"buildfossil: v3 requires complete non-empty failure stderr")
		return 125
	}

	if err := verifyCapturedWorkspaceV3(
		workspaceRoot, includes, beforeFiles,
	); err != nil {
		fmt.Fprintf(os.Stderr,
			"buildfossil: workspace changed after execution: %v\n", err)
		return 125
	}

	files := beforeFiles

	var goMod []byte
	var goSum []byte
	var hasGoMod bool
	var hasGoSum bool

	for _, file := range files {
		switch file.Path {
		case "go.mod":
			goMod = file.Data
			hasGoMod = true
		case "go.sum":
			goSum = file.Data
			hasGoSum = true
		}
	}

	if !hasGoMod || !hasGoSum {
		fmt.Fprintln(os.Stderr,
			"buildfossil: v3 requires --include go.mod and --include go.sum")
		return 125
	}

	cacheCommand := exec.Command("go", "env", "GOMODCACHE")
	cacheCommand.Dir = workspaceRoot

	cacheOutput, err := cacheCommand.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"buildfossil: resolve GOMODCACHE: %v\n", err)
		return 125
	}

	moduleCacheRoot := strings.TrimSpace(string(cacheOutput))
	if moduleCacheRoot == "" {
		fmt.Fprintln(os.Stderr,
			"buildfossil: empty GOMODCACHE")
		return 125
	}

	selectedModules, err := capsule.ListGoModuleGraphV3(
		workspaceRoot,
		moduleCacheRoot,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"buildfossil: resolve offline Go module graph: %v\n", err)
		return 125
	}

	// The graph must describe the same go.mod/go.sum bytes that
	// will be embedded in the capsule.
	for _, original := range []struct {
		name string
		data []byte
	}{
		{"go.mod", goMod},
		{"go.sum", goSum},
	} {
		current, err := os.ReadFile(
			filepath.Join(workspaceRoot, original.name),
		)
		if err != nil || !bytes.Equal(current, original.data) {
			fmt.Fprintf(os.Stderr,
				"buildfossil: %s changed during module graph resolution\n",
				original.name,
			)
			return 125
		}
	}

	goModules, artifacts, err :=
		capsule.ReadGoModuleArtifactSetForCaptureV3(
			selectedModules,
			goSum,
			moduleCacheRoot,
		)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"buildfossil: capture Go module artifacts: %v\n", err)
		return 125
	}

	if err := capsule.ValidateGoModuleConsistencyV3(
		goMod, goModules,
	); err != nil {
		fmt.Fprintf(os.Stderr,
			"buildfossil: inconsistent Go module graph: %v\n", err)
		return 125
	}

	if err := verifyCapturedWorkspaceV3(
		workspaceRoot, includes, files,
	); err != nil {
		fmt.Fprintf(os.Stderr,
			"buildfossil: workspace changed before packaging: %v\n", err)
		return 125
	}

	manifest := capsule.Manifest{
		SchemaVersion: capsule.SchemaVersionV3,
		Runtime:       detectedRuntime,
		GoBuildEnv:    &goBuildEnv,
		GoModules:     goModules,
		Execution: capsule.Execution{
			Argv:            result.Argv,
			WorkingDir:      ".",
			ExitCode:        result.ExitCode,
			Stdout:          result.Stdout,
			Stderr:          result.Stderr,
			StdoutTruncated: result.StdoutTruncated,
			StderrTruncated: result.StderrTruncated,
		},
		Platform: capsule.Platform{
			OS:           runtime.GOOS,
			Architecture: runtime.GOARCH,
		},
	}

	var archive bytes.Buffer

	if err := capsule.WriteWorkspaceV3(
		&archive,
		manifest,
		files,
		artifacts,
	); err != nil {
		fmt.Fprintf(os.Stderr,
			"buildfossil: encode v3 capsule: %v\n", err)
		return 125
	}

	f, err := os.OpenFile(
		output,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		0600,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"buildfossil: create capsule: %v\n", err)
		return 125
	}

	_, writeErr := f.Write(archive.Bytes())
	closeErr := f.Close()

	if writeErr != nil || closeErr != nil {
		_ = os.Remove(output)
		fmt.Fprintf(os.Stderr,
			"buildfossil: write v3 capsule: write=%v close=%v\n",
			writeErr,
			closeErr,
		)
		return 125
	}

	fmt.Fprintf(os.Stderr,
		"buildfossil: capsule v3 saved to %s\n", output)

	return result.ExitCode
}
