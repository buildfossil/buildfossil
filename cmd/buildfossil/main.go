package main

import (
	"fmt"
	"os"
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

		return captureCommand(args[2:], "failure.bfc", false)

	case "replay":
		return runReplay(args[1:])

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		return 2
	}
}
