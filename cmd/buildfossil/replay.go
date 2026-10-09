package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/buildfossil/buildfossil/internal/replay"
)

func runReplay(args []string) int {
	const usage = "Usage: buildfossil replay [--allow-command] <capsule.bfc>"

	var capsulePath string
	var options replay.Options

	switch {
	case len(args) == 1 && args[0] != "--allow-command":
		capsulePath = args[0]

	case len(args) == 2 && args[0] == "--allow-command":
		options.AllowArbitraryCommand = true
		capsulePath = args[1]

	default:
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}

	if capsulePath == "" || capsulePath == "--allow-command" {
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	result, err := replay.RunWithOptions(ctx, capsulePath, options)
	if err != nil {
		fmt.Fprintf(os.Stderr, "buildfossil: %v\n", err)
		return 125
	}

	fmt.Printf("Original exit code: %d\n", result.OriginalExitCode)
	fmt.Printf("Replay exit code:   %d\n", result.ReplayExitCode)
	fmt.Printf("Outcome:            %s\n", result.Outcome)

	switch result.Outcome {
	case replay.OutcomeReproduced:
		return 0
	case replay.OutcomePassed:
		return 10
	case replay.OutcomeDifferent:
		return 11
	case replay.OutcomeInconclusive:
		return 12
	default:
		return 125
	}
}
