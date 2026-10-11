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
  buildfossil capture --v3 --include <file> ... -- go build ./...

Requirements:
  Go 1.26.6
  GOOS/GOARCH: darwin/arm64 or linux/amd64
  CGO_ENABLED=0 and empty GOFLAGS
  GOWORK=off when a Go workspace is active
  Explicitly include go.mod, go.sum, and required source files
  At most eight external Go modules, including transitive dependencies
  Active Go workspaces are unsupported

Example:
  buildfossil capture --v3 \
    --include go.mod \
    --include go.sum \
    --include main.go \
    -- go build ./...

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
