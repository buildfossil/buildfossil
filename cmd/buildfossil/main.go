package main

import (
	"fmt"
	"os"

	"github.com/buildfossil/buildfossil/internal/execution"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: buildfossil <command>")
		return 2
	}

	switch args[0] {
	case "capture-demo":
		return runCaptureDemo(args[1:])

	case "version":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Usage: buildfossil version")
			return 2
		}
		fmt.Println("buildfossil dev")
		return 0

	case "capture":
		if len(args) < 3 || args[1] != "--" {
			fmt.Fprintln(os.Stderr, "Usage: buildfossil capture -- <command> [args...]")
			return 2
		}

		result, err := execution.Run(args[2:], os.Stdout, os.Stderr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "buildfossil: %v\n", err)
			return 125
		}

		return result.ExitCode

	case "replay":
		return runReplay(args[1:])

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		return 2
	}
}
