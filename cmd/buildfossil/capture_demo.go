package main

import (
	"fmt"
	"os"
)

func runCaptureDemo(args []string) int {
	if len(args) < 2 || args[0] != "--" {
		fmt.Fprintln(os.Stderr, "Usage: buildfossil capture-demo -- <command> [args...]")
		return 2
	}

	return captureCommand(args[1:], "failure.bfc", true)
}
