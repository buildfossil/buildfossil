package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/buildfossil/buildfossil/internal/replay"
)

func replayContext(
	parent context.Context,
	useV2 bool,
) (context.Context, context.CancelFunc) {
	if useV2 {
		return context.WithCancel(parent)
	}

	return context.WithTimeout(parent, 30*time.Second)
}

func runReplay(args []string) int {
	const usage = "Usage:\n" +
		"  buildfossil replay [--allow-command] <capsule.bfc>\n" +
		"  buildfossil replay --v2 --allow-command [--diagnose-dependencies] <capsule.bfc>\n" +
		"  buildfossil replay --v3 --allow-command <capsule.bfc>"

	var capsulePath string
	var options replay.Options
	var useV2 bool
	var useV3 bool

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

	case len(args) == 4 &&
		args[0] == "--v2" &&
		args[1] == "--allow-command" &&
		args[2] == "--diagnose-dependencies":
		useV2 = true
		options.AllowArbitraryCommand = true
		options.DiagnoseDependencies = true
		capsulePath = args[3]

	case len(args) == 3 &&
		args[0] == "--v3" &&
		args[1] == "--allow-command":
		useV3 = true
		options.AllowArbitraryCommand = true
		capsulePath = args[2]

	default:
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}

	if capsulePath == "" || strings.HasPrefix(capsulePath, "--") {
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}

	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
	)
	defer stop()

	ctx, cancel := replayContext(signalCtx, useV2 || useV3)
	defer cancel()

	var result replay.Result
	var err error

	switch {
	case useV3:
		result, err = replay.RunV3WithOptions(ctx, capsulePath, options)
	case useV2:
		result, err = replay.RunV2WithOptions(ctx, capsulePath, options)
	default:
		result, err = replay.RunWithOptions(ctx, capsulePath, options)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "buildfossil: %v\n", err)
		return 125
	}

	fmt.Printf("Original exit code: %d\n", result.OriginalExitCode)
	fmt.Printf("Replay exit code:   %d\n", result.ReplayExitCode)
	fmt.Printf("Outcome:            %s\n", result.Outcome)

	if result.GoDependencies != nil {
		fmt.Printf("Go dependencies:    %s\n", result.GoDependencies.State)

		if result.GoDependencies.Reason != "" {
			fmt.Printf("Dependency details: %s\n", result.GoDependencies.Reason)
		}
	}

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
