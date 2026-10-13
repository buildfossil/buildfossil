package main

import "fmt"

func printGeneralHelp() {
	fmt.Println(`BuildFossil — Capture failed Go builds and replay them offline.

Experimental software. Not production-ready.

Usage:
  buildfossil <command> [options]

Commands:
  capture    Capture a failed Go build
  replay     Verify and replay a capsule
  version    Print the CLI version
  help       Show this help

Run 'buildfossil <command> --help' for command-specific help.`)
}

func printCaptureHelp() {
	fmt.Println(`Capture a failed Go build into a portable .bfc capsule.

Usage:
  buildfossil capture --v3 --include-go-sources [--include-go-embed] -- go build ./...
  buildfossil capture --v3 --include <file> ... [--include-go-embed] -- go build ./...

Requirements:
  Go 1.26.6
  GOOS/GOARCH: darwin/arm64 or linux/amd64
  CGO_ENABLED=0 and empty GOFLAGS
  GOWORK=off when a Go workspace is active
  Without --include-go-sources, explicitly include go.mod, any existing
  go.sum, and required source files
  --include-go-sources selects main-module Go sources for Linux AMD64
  and the supported capture platform, plus go.mod and any existing go.sum
  --include-go-sources may be combined with explicit --include files
  Embedded resources require --include-go-embed or manual --include
  Automatic discovery does not guarantee all non-Go build inputs are included
  At most 128 selected external Go modules; up to 64 with complete ZIP artifacts
  Optional --include-go-embed discovers embedded resources offline
  Workspace limit: 2048 archived files, including virtual go.sum
  Active Go workspaces are unsupported

Examples:
  buildfossil capture --v3 --include-go-sources -- go build ./...

  buildfossil capture --v3 \
    --include go.mod \
    --include go.sum \
    --include main.go \
    -- go build ./...

WARNING:
  Captured source files and manually included files are stored in the
  capsule. Review sensitive content before sharing the .bfc file.

The resulting capsule is saved as failure.bfc.
Existing output files are never overwritten.
Move or rename failure.bfc before another capture.
Capture supports only the exact command go build ./... in schema v3.`)
}

func printReplayHelp() {
	fmt.Println(`Verify and replay a captured failure using Docker.

Usage:
  buildfossil replay --v3 --allow-command [--memory 256m|512m|1g|2g] <capsule.bfc>

Requirements:
  Docker Engine with Linux AMD64 container execution
  Access to the pinned Go replay image

Memory:
  Default: 256 MiB
  Allowed: 256m, 512m, 1g, 2g

Examples:
  buildfossil replay --v3 --allow-command failure.bfc
  buildfossil replay --v3 --allow-command --memory 1g failure.bfc

WARNING:
  Replay executes the command recorded in the capsule.
  Only replay capsules from sources you trust.
  Docker isolation is not a complete security boundary.`)
}
