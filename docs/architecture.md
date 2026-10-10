# BuildFossil Architecture

## Status

This document describes the experimental schema-v3 implementation.

It is not a specification for future releases. The archive format and implementation details may change.

## Purpose

BuildFossil attempts to reproduce certain failed Go builds using a captured subset of the original inputs.

Its architecture consists of four main stages:

1. Capture
2. Capsule verification
3. Workspace and dependency staging
4. Offline Docker replay and failure comparison

## Repository Structure

| Directory | Responsibility |
|---|---|
| `cmd/buildfossil` | CLI argument handling and capture entry points |
| `internal/capsule` | Manifest, archive writing, verification, and restoration |
| `internal/execution` | Captured command execution |
| `internal/replay` | Replay orchestration, portability checks, and result comparison |
| `internal/runtime/docker` | Docker execution, restrictions, and lifecycle management |
| `testdata` | Integration test fixtures |

## Supported v3 Execution Flow

```text
Original Go workspace
        |
        v
buildfossil capture --v3
        |
        +-- Validate supported command and runtime
        |
        +-- Inspect Go build environment
        |
        +-- Execute original Go build
        |
        +-- On failure, collect selected workspace files
        |
        +-- Read dependency artifacts from Go module cache
        |
        +-- Verify module content and checksums
        |
        v
    failure.bfc
        |
        v
buildfossil replay --v3 --allow-command
        |
        +-- Verify capsule contents
        |
        +-- Check platform and runtime compatibility
        |
        +-- Create temporary staging directories
        |
        +-- Restore and verify captured workspace
        |
        +-- Stage Go module artifacts
        |
        v
Restricted Docker container
        |
        +-- Prepare offline module cache
        |
        +-- Execute recorded Go build
        |
        +-- Capture bounded output and exit status
        |
        v
Compare original and replayed failures
        |
        v
Outcome
```

## Capture

The schema-v3 entry point is implemented in:

`cmd/buildfossil/capture_v3.go`

The CLI currently accepts only the supported command:

`go build ./...`

The capture stage inspects the Go environment before executing the original build.

If the build succeeds, no failure capsule is written.

If it fails, BuildFossil requires complete, nonempty stderr and attempts to package the recorded failure.

The user must explicitly select workspace files using `--include`.

For the current supported dependency scenario, both `go.mod` and `go.sum` must be included.

## Go Dependency Capture

The implementation currently supports exactly one direct external Go module dependency.

Dependency capture reads Go module cache artifacts:

- `.zip`
- `.mod`
- `.info`

The module path and version determine the expected artifact locations.

Captured artifacts are checked against the supported module format and the checksums recorded in `go.sum`.

This is not a general dependency resolver.

Unsupported module configurations are rejected rather than silently captured.

## Capsule Format

A `.bfc` is an archive containing a manifest, selected workspace content, and dependency artifacts.

Schema-v3 records information including:

- Schema version
- Supported runtime
- Source platform
- Go build environment
- Original command arguments
- Exit status
- Captured stdout and stderr
- Workspace integrity metadata
- Dependency identity and artifact metadata

The current format is experimental.

Do not assume compatibility with future schema versions.

## Verification

Capsule verification is handled by code under `internal/capsule`.

Validation includes structural checks, supported metadata, workspace integrity, and module artifact verification.

SHA-256 verifies recorded file content against the manifest.

Go module checksum validation provides an additional consistency check for dependency contents.

These checks detect classes of malformed or modified inputs.

They do not authenticate the capsule creator and do not establish that captured source code is safe.

## Replay Preparation

The replay orchestrator is implemented in:

`internal/replay/replay_v3.go`

Before Docker execution, the replay path:

1. Verifies the capsule.
2. Evaluates runtime and platform compatibility.
3. Requires explicit command-execution authorization.
4. Creates temporary staging directories.
5. Restores workspace files.
6. Checks staged workspace integrity.
7. Stages the required dependency artifacts.

The replay implementation does not reconstruct an entire original machine, operating system, or CI runner.

## Docker Execution

The Docker integration lives under:

`internal/runtime/docker`

Schema-v3 replay uses a pinned Go runtime image and a constrained container configuration.

The implemented controls include:

- Disabled container networking
- Read-only container root filesystem
- Non-root process execution
- Dropped Linux capabilities
- Restricted privilege escalation
- Resource limits
- Bounded output capture
- Container cleanup

The module artifacts are made available to the container, where the Go module cache is prepared before the recorded build is executed.

This pre-execution preparation is part of replay orchestration. It is not an exact reproduction of every operation originally performed on the capture host.

Docker is not a complete security boundary for malicious workloads.

## Failure Comparison

BuildFossil compares recorded and replayed failure results.

For the supported schema-v3 scenario, successful reproduction requires matching failure characteristics, including the exit status and stderr.

A `reproduced` outcome means the implemented comparison accepted the replay result.

It does not prove complete environmental equivalence.

## Portability

The verified cross-platform CI scenario is:

- Capture host: macOS ARM64
- Go build target: Linux AMD64
- CGO: disabled
- Replay host: Linux AMD64
- Replay runtime: pinned Go 1.26.6 Docker environment

The capsule is transferred between independent GitHub Actions runners.

This validates that specific supported scenario, not arbitrary cross-platform builds.

## Security Boundaries and Known Gaps

The most important trust boundaries are:

- Capture workspace to archive
- Archive to replay staging
- Staging to Docker container
- Container runtime to host kernel

Important known gaps include:

- No authenticated capsule provenance
- No security guarantee for unknown or hostile capsules
- Incomplete protection against concurrent staging changes
- Limited Go dependency graph support
- Limited command and platform support
- No complete CI environment snapshot

See `SECURITY.md` for the security policy and threat model.

## Test Coverage

The test suite contains:

- Unit tests for archive and manifest validation
- Go module checksum and artifact tests
- Workspace restoration and staging integrity tests
- Capture CLI tests
- Docker execution and cancellation tests
- End-to-end replay tests
- Cross-platform GitHub Actions jobs

Run the ordinary suite with:

```sh
go test -count=1 ./...
```

Docker integration tests require an available Docker daemon and the relevant test environment configuration.

## Design Principle

BuildFossil currently prioritizes a small, explicit, verifiable execution model over broad but weakly specified CI compatibility.

New runtime, dependency, or platform support should come with corresponding verification rules and end-to-end tests.
