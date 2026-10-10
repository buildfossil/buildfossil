<div align="center">

  <img src="assets/img/logo.svg" alt="BuildFossil" width="340">

  <h3>Capture a failed Go build. Replay the failure offline.</h3>

  <p>
    <img src="https://img.shields.io/badge/status-experimental-orange" alt="Experimental">
    &nbsp;
    <img src="https://img.shields.io/badge/license-Apache%202.0-blue" alt="Apache 2.0">
    &nbsp;
    <img src="https://img.shields.io/badge/open%20source-yes-brightgreen" alt="Open Source">
  </p>

  <p>
    Portable build failure capsules for reproducible debugging.
  </p>

</div>

BuildFossil is an experimental open-source CLI that packages a failed Go build, selected workspace files, and verified dependency artifacts into a portable `.bfc` capsule.

The capsule can then be replayed inside a constrained Linux Docker container without network access from the container.

> **Status:** Experimental engineering prototype. Not production-ready.

## Why BuildFossil?

A build can fail in one environment and be difficult to reproduce in another.

BuildFossil explores a different debugging workflow:

1. Run a build through BuildFossil.
2. Capture a failed execution and its required inputs.
3. Transfer the resulting `.bfc` capsule.
4. Verify the capsule and replay the build in Docker.
5. Compare the original and replayed failure.

The goal is to make certain build failures easier to investigate without repeatedly recreating the original environment.

BuildFossil does not guarantee that every build failure can be reproduced.

## Current Support

The schema-v3 prototype supports:

- Go 1.26.6.
- The exact command `go build ./...`.
- Zero or one direct external Go module dependency.
- Selected regular workspace files.
- Verified Go module `.zip`, `.mod`, and `.info` artifacts when a dependency is present.
- SHA-256 and Go module checksum verification.
- Captured stdout, stderr, exit code, and build environment metadata.
- Offline replay inside a Linux AMD64 Docker container.
- Comparison of original and replayed failures.
- An explicit opt-in for replaying captured commands.

BuildFossil also retains older experimental capture/replay modes.

The schema-v3 format is experimental and may change.

Projects with no external Go dependencies are supported. For these
projects, the capsule contains no dependency artifacts.

Capture requires explicitly including both `go.mod` and `go.sum`,
even when `go.sum` is empty. Active Go workspaces (`go.work`) are
not supported; use `GOWORK=off`.

## Requirements

For development and the supported schema-v3 workflow:

- Go 1.26.6.
- Docker Engine or Docker Desktop with Linux containers.
- Linux AMD64 container execution support.
- Access to the pinned replay image when it is not already cached.

Replay containers run without network access. The Docker host may still need network access to obtain the pinned image.

## Install from Source

Clone the repository and build the CLI:

```sh
git clone https://github.com/buildfossil/buildfossil.git
cd buildfossil

go build -o buildfossil ./cmd/buildfossil
BUILDFOSSIL_BIN="$(pwd)/buildfossil"
./buildfossil version
```

The current development version reports `buildfossil dev`.

No stable release or binary distribution is provided yet.

## Quick Start: Capture a Go Build Failure

The following example creates a small Go project with one external dependency and an intentional compiler error.

Create the project:

```sh
mkdir buildfossil-example
cd buildfossil-example

cat > go.mod <<'MOD'
module example.com/buildfossil-example

go 1.26.0

require github.com/google/uuid v1.6.0
MOD

cat > main.go <<'GO'
package main

import (
    "fmt"

    "github.com/google/uuid"
)

func main() {
    fmt.Println(uuid.NewString())
    undefinedFunction()
}
GO
```

Download and verify the dependency before capture:

```sh
go mod download github.com/google/uuid@v1.6.0
go mod download
```

Make sure `go.sum` contains both the module and its `/go.mod` checksum entries.

For a portable Linux AMD64 build target, set:

```sh
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0
export GOFLAGS=
export GOTOOLCHAIN=local
```

Disable module proxy and checksum database access during capture:

```sh
export GOPROXY=off
export GOSUMDB=off
```

Ensure no external Go workspace is active:

```sh
export GOWORK=off
```

Run the BuildFossil executable built in the previous step, replacing `"$BUILDFOSSIL_BIN"` with its actual absolute path:

```sh
/path/to/buildfossil capture --v3 \
  --include go.mod \
  --include go.sum \
  --include main.go \
  -- go build ./...
```

The build is expected to fail because `undefinedFunction` does not exist.

BuildFossil saves `failure.bfc` in the current working directory when the failed build satisfies the supported capture requirements.

A successfully captured compiler failure normally preserves the original nonzero build exit code.

If capture itself fails, BuildFossil may exit with code `125` and will not produce a valid new capsule.

## Replay the Failure

Make the `.bfc` capsule available on a machine with a supported Docker environment.

Run:

```sh
/path/to/buildfossil replay --v3 --allow-command failure.bfc
```

A successfully reproduced failure reports output similar to:

```text
Original exit code: 1
Replay exit code:   1
Outcome:            reproduced
```

`reproduced` means BuildFossil's current comparison logic considers the captured and replayed failures equivalent.

It does not prove that every aspect of the original execution environment was reconstructed.

**Only replay capsules from sources you trust.** The `--allow-command` option explicitly permits execution of the command recorded in the capsule.

## What Is Inside a Capsule?

A schema-v3 `.bfc` capsule contains:

- A JSON manifest describing the captured execution.
- Selected workspace files and their integrity metadata.
- Captured build environment information.
- Zero dependency artifacts for dependency-free builds, or three
  verified Go module artifacts for one supported external dependency.
- Recorded command output and exit status.

The archive is verified before replay. Workspace and module-cache
snapshots are constructed from verified capsule contents and uploaded
to Docker-managed volumes, without host bind mounts for replay inputs.

Integrity verification is not the same as authenticating who created a capsule.

## Security

BuildFossil replays commands from captured artifacts. It must not be treated as a safe executor for arbitrary, untrusted code.

The replay implementation applies container restrictions, including disabled container networking, but Docker isolation is not a complete security boundary.

Capsules may contain:

- Source code and other included files.
- Command arguments.
- Build output and diagnostics.
- Environment metadata.
- Dependency artifacts.

Do not capture confidential projects or production credentials without carefully reviewing the information that may be embedded in the capsule.

A capsule can pass integrity checks without being trustworthy.

For the schema-v3 staging trust assumptions and remaining TOCTOU concerns, see [Replay Staging Security Model](docs/security-model.md).

## Known Limitations

The current schema-v3 implementation:

- Supports only the exact command `go build ./...`.
- Requires Go 1.26.6.
- Supports zero or one direct external Go module dependency.
- Requires explicit inclusion of `go.mod` and `go.sum`, even for
  dependency-free builds.
- Does not support active Go workspaces (`go.work`), `replace`,
  `exclude`, or multiple external Go modules.
- Does not support indirect-only dependency configurations.
- Does not support arbitrary dependency graphs.
- Does not support general-purpose CI job capture.
- Requires workspace files to be explicitly included.
- Does not create a complete filesystem or process snapshot.
- Does not guarantee reproduction across all host and target platforms.
- Does not authenticate capsule origin.
- Is not approved for executing untrusted capsules.
- Has unresolved security-hardening work, including staging race considerations.

Replaying a captured failure can produce a different result or fail when the supported reproducibility assumptions are not met.

Replay runs with `GOWORK=off`, empty `GOFLAGS`, `GOPROXY=off`,
`GOSUMDB=off`, and `GOTOOLCHAIN=local` in the restricted Docker
execution container.

A failure is reported as reproduced only when the replay exit code
and complete stderr match the captured failure exactly.

## Tested Environments

The GitHub Actions CI suite exercises:

- Go unit tests and static analysis.
- Linux AMD64 CLI compilation.
- Docker offline replay integration tests.
- macOS ARM64 Go capture.
- Linux Docker replay of a schema-v3 capsule created on macOS ARM64.
- Existing legacy capture/replay compatibility tests.

The schema-v3 cross-platform CI scenario targets Linux AMD64 during capture, transfers the capsule between independent runners, and replays it on Linux AMD64.

This does not establish general compatibility with every Go build target.

## Development

Run tests:

```sh
go test -count=1 ./...
```

Run static analysis:

```sh
go vet ./...
```

Run Docker integration tests with a working Docker daemon:

```sh
BUILDFOSSIL_DOCKER_TEST=1 go test -count=1 -timeout=8m ./...
```

Build the CLI:

```sh
go build -o buildfossil ./cmd/buildfossil
```

## Project Status

BuildFossil is in active experimental development.

Before a public alpha release, the project requires additional security review, documentation, installation testing, and compatibility validation.

The next priorities include:

1. Documenting the threat model and trust boundaries.
2. Hardening the replay staging and execution path.
3. Expanding reproducibility tests.
4. Improving failure diagnostics.
5. Evaluating support for larger Go dependency graphs.

Features outside the documented supported scenarios should not be assumed to work.

## Contributing

BuildFossil welcomes early contributors interested in Go, DevOps, build reproducibility, container infrastructure, and developer tools.

Contributions can include bug reports, tests, documentation, code improvements, and technical discussions.

Looking for a place to start? Explore our [open issues](https://github.com/buildfossil/buildfossil/issues), especially those labeled `good first issue` or `help wanted`.

For larger changes, please open an Issue to discuss the approach before submitting a Pull Request.

Read [CONTRIBUTING.md](CONTRIBUTING.md) to get started.

## License

Licensed under the Apache License, Version 2.0.

See [LICENSE](LICENSE).