package main

import (
	"bytes"
	"fmt"
	"os"
	"runtime"

	"github.com/buildfossil/buildfossil/internal/capsule"
	"github.com/buildfossil/buildfossil/internal/execution"
)

func captureCommandV2(
	argv []string,
	workspaceRoot string,
	includes []string,
	output string,
) int {
	if len(includes) == 0 {
		fmt.Fprintln(os.Stderr, "buildfossil: at least one --include is required")
		return 2
	}

	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "buildfossil: missing command")
		return 2
	}

	result, err := execution.RunInDir(
		argv,
		workspaceRoot,
		os.Stdout,
		os.Stderr,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "buildfossil: execute command: %v\n", err)
		return 125
	}

	if result.ExitCode == 0 {
		return 0
	}

	files, err := capsule.ReadWorkspaceFilesV2(workspaceRoot, includes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "buildfossil: read workspace: %v\n", err)
		return 125
	}

	manifest := capsule.Manifest{
		SchemaVersion: capsule.SchemaVersionV2,
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

	// Construct the complete archive before creating the output file.
	// This prevents validation failures from leaving a partial capsule.
	var archive bytes.Buffer

	if err := capsule.WriteWorkspaceV2(&archive, manifest, files); err != nil {
		fmt.Fprintf(os.Stderr, "buildfossil: encode capsule: %v\n", err)
		return 125
	}

	f, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "buildfossil: create capsule: %v\n", err)
		return 125
	}

	_, writeErr := f.Write(archive.Bytes())
	closeErr := f.Close()

	if writeErr != nil || closeErr != nil {
		_ = os.Remove(output)
		fmt.Fprintf(
			os.Stderr,
			"buildfossil: write capsule: write=%v close=%v\n",
			writeErr,
			closeErr,
		)
		return 125
	}

	fmt.Fprintf(os.Stderr, "buildfossil: capsule v2 saved to %s\n", output)
	return result.ExitCode
}
