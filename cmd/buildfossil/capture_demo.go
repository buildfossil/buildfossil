package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/buildfossil/buildfossil/internal/capsule"
	"github.com/buildfossil/buildfossil/internal/execution"
)

func runCaptureDemo(args []string) int {
	if len(args) < 2 || args[0] != "--" {
		fmt.Fprintln(os.Stderr, "Usage: buildfossil capture-demo -- <command> [args...]")
		return 2
	}

	result, err := execution.Run(args[1:], os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "buildfossil: %v\n", err)
		return 125
	}

	file, err := capsule.ReadWorkspaceFile(".", "fixture.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "buildfossil: %v\n", err)
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

	if err := capsule.WriteFile("failure.bfc", manifest, file); err != nil {
		fmt.Fprintf(os.Stderr, "buildfossil: %v\n", err)
		return 125
	}

	fmt.Fprintln(os.Stderr, "buildfossil: capsule saved to failure.bfc")
	return result.ExitCode
}
