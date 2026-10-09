package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/buildfossil/buildfossil/internal/replay"
)

func runReplay(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: buildfossil replay <capsule.bfc>")
		return 2
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	result, err := replay.Run(ctx, args[0])
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
