package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/buildfossil/buildfossil/internal/replay"
)

func runReplay(args []string) int {
	const usage = "Usage: buildfossil replay [--v2] [--allow-command] <capsule.bfc>"

	var capsulePath string
	var options replay.Options
	var useV2 bool

	switch {
	case len(args) == 1:
		capsulePath = args[0]

	case len(args) == 2 && args[0] == "--allow-command":
		options.AllowArbitraryCommand = true
		capsulePath = args[1]

	case len(args) == 3 &&
		args[0] == "--v2" &&
		args[1] == "--allow-command":
		useV2 = true
		options.AllowArbitraryCommand = true
		capsulePath = args[2]

	default:
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}

	if capsulePath == "" ||
		capsulePath == "--allow-command" ||
		capsulePath == "--v2" {
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
	)
	defer stop()

	var result replay.Result
	var err error

	if useV2 {
		result, err = replay.RunV2WithOptions(ctx, capsulePath, options)
	} else {
		result, err = replay.RunWithOptions(ctx, capsulePath, options)
	}
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
