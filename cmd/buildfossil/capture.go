package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/buildfossil/buildfossil/internal/capsule"
	"github.com/buildfossil/buildfossil/internal/execution"
)

func captureCommand(argv []string, output string, saveOnSuccess bool) int {
	return captureCommandInWorkspace(argv, ".", output, saveOnSuccess)
}

func captureCommandInWorkspace(
	argv []string,
	workspaceRoot string,
	output string,
	saveOnSuccess bool,
) int {
	result, err := execution.Run(argv, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "buildfossil: execute command: %v\n", err)
		return 125
	}

	if result.ExitCode == 0 && !saveOnSuccess {
		return 0
	}

	file, err := capsule.ReadWorkspaceFile(workspaceRoot, "fixture.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "buildfossil: read workspace: %v\n", err)
		return 125
	}

	manifest := capsule.Manifest{
		SchemaVersion: capsule.SchemaVersion,
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

	if err := capsule.WriteFile(output, manifest, file); err != nil {
		fmt.Fprintf(os.Stderr, "buildfossil: write capsule: %v\n", err)
		return 125
	}

	fmt.Fprintf(os.Stderr, "buildfossil: capsule saved to %s\n", output)
	return result.ExitCode
}
