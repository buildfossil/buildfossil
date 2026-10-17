package main

import (
	"context"
	"fmt"
	"os"
	"time"

	rewdocker "github.com/rewtools/rew/internal/runtime/docker"
)

func runCleanup(args []string) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Println("Usage: rew cleanup --dry-run")
		fmt.Println("Inspect Docker snapshot volume candidates without deleting resources.")
		return 0
	}

	if len(args) != 1 || args[0] != "--dry-run" {
		fmt.Fprintln(os.Stderr, "Usage: rew cleanup --dry-run")
		return 2
	}

	dockerClient, err := rewdocker.NewSDKClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "rew: initialize Docker: %v\n", err)
		return 1
	}
	defer dockerClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	candidates, err := rewdocker.ListSnapshotVolumeCandidates(ctx, dockerClient)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rew: inspect Docker snapshot volumes: %v\n", err)
		return 1
	}

	fmt.Println("REW Docker resource inspection")
	fmt.Println()
	fmt.Println("Snapshot volume candidates:")

	if len(candidates) == 0 {
		fmt.Println("  none")
	} else {
		for _, candidate := range candidates {
			fmt.Printf("  %s  %s\n", candidate.Name, candidate.Kind)
		}
	}

	fmt.Println()
	fmt.Printf("Found %d snapshot volume candidate(s).\n", len(candidates))
	fmt.Println("Container inspection is not yet supported.")
	fmt.Println("No resources were removed.")

	return 0
}
