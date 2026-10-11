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
- Up to sixteen selected external Go modules, including transitive dependencies.
- Selected regular workspace files.
- Verified `.zip`, `.mod`, and `.info` artifacts for every selected external module.
- SHA-256 and Go module checksum verification.
- Captured stdout, stderr, exit code, and build environment metadata.
- Offline replay inside a Linux AMD64 Docker container.
- Comparison of original and replayed failures.
- An explicit opt-in for replaying captured commands.

BuildFossil also retains older experimental capture/replay modes.

The schema-v3 format is experimental and may change.

Projects with no external Go dependencies are supported. For these
projects, the capsule contains no dependency artifacts.

Capture requires including `go.mod` and necessary source files.
An existing `go.sum` must also be included. Dependency-free Go projects
without `go.sum` are supported: an empty file is embedded in the capsule.
Active Go workspaces (`go.work`) are unsupported; use `GOWORK=off`.

## Requirements

For development and the supported schema-v3 workflow:

- Go 1.26.6.
- Docker Engine or Docker Desktop with Linux containers.
- Linux AMD64 container execution support.
- Access to the pinned replay image when it is not already cached.

Replay containers run without network access. The Docker host may still need network access to obtain the pinned image.

## Install

The current public alpha is [v0.1.0-alpha.2](https://github.com/buildfossil/buildfossil/releases/tag/v0.1.0-alpha.2).

Prebuilt binaries are available for macOS ARM64 and Linux AMD64.
These binaries are experimental and not production-ready.

Download the archive for your platform and SHA256SUMS from the release page.
Verify the downloaded archive against its SHA-256 checksum before extracting it.

macOS ARM64:

    grep 'buildfossil_v0.1.0-alpha.2_darwin_arm64.tar.gz' SHA256SUMS | shasum -a 256 -c -
    tar -xzf buildfossil_v0.1.0-alpha.2_darwin_arm64.tar.gz
    ./buildfossil version

Linux AMD64:

    grep 'buildfossil_v0.1.0-alpha.2_linux_amd64.tar.gz' SHA256SUMS | sha256sum -c -
    tar -xzf buildfossil_v0.1.0-alpha.2_linux_amd64.tar.gz
    ./buildfossil version

The checksum verification must report OK before extracting the archive.
The expected version is `buildfossil v0.1.0-alpha.2`.

Set the CLI path before following Quick Start:

    export BUILDFOSSIL_BIN="$(pwd)/buildfossil"

The macOS binary is not Developer ID signed or notarized.
macOS Gatekeeper may require explicit approval.
Do not run binaries from untrusted sources.

## Install from Source

Clone the repository and build the CLI:

```sh
git clone https://github.com/buildfossil/buildfossil.git
cd buildfossil

go build -o buildfossil ./cmd/buildfossil
BUILDFOSSIL_BIN="$(pwd)/buildfossil"
./buildfossil version
```

Locally built development binaries normally report `buildfossil dev`.
Binaries installed from a tagged Go module report the module version.
Official release binaries can embed their version at build time.

Public alpha binaries are available; no stable release is available yet.

## Command-Line Help

After building BuildFossil, you can inspect its available commands:

    ./buildfossil --help
    ./buildfossil capture --help
    ./buildfossil replay --help
    ./buildfossil --version

The `capture` and `replay` help commands describe the supported schema-v3
workflow. Older experimental modes remain available but are not the
recommended starting point.

## Quick Start: Reproduce a Failed Build

This example requires Go 1.26.6, Docker with Linux AMD64
support, and the BuildFossil CLI installed above.

Create a dependency-free Go project:

```sh
mkdir buildfossil-example
cd buildfossil-example

cat > go.mod <<'MOD'
module example.com/buildfossil-quickstart

go 1.26.0
MOD

cat > main.go <<'GO'
package main

func main() {
    undefinedFunction()
}
GO
```

Capture the intentional compiler failure:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOFLAGS='' \
GOWORK=off GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off \
"$BUILDFOSSIL_BIN" capture --v3 \
  --include go.mod \
  --include main.go \
  -- go build ./...
```

The build fails intentionally and BuildFossil saves `failure.bfc`.
A successful capture normally returns the original build's
nonzero exit code.

No `go.sum` is required for this dependency-free example.
BuildFossil does not create `go.sum` in the source project.

Replay the failure using Docker:

```sh
"$BUILDFOSSIL_BIN" replay --v3 --allow-command --memory 1g failure.bfc
```

Expected output:

```text
Original exit code: 1
Replay exit code:   1
Outcome:            reproduced
Replay memory:      1 GiB
```

Only replay capsules from trusted sources. Replay executes
the command recorded inside the capsule.

## Understanding Replay Results

Make the `.bfc` capsule available on a machine with a supported Docker environment.

Run:

```sh
"$BUILDFOSSIL_BIN" replay --v3 --allow-command failure.bfc
```

When Replay v3 returns `different_failure` (exit code `11`), it also
reports the category of the mismatch:

- `exit codes differ`: the original and replayed commands exited with different codes.
- `stderr differs`: exit codes match, but the captured and replayed standard error differ.

For example:

    Original exit code: 1
    Replay exit code:   1
    Outcome:            different_failure
    Difference:         stderr differs

BuildFossil does not print the full stderr difference. If the captured or
replayed stderr was truncated, the comparison is `inconclusive` rather than
claiming a complete mismatch.

Schema-v3 replay uses 256 MiB of container memory by default.

For memory-intensive builds or AMD64 emulation on ARM64, use:

    "$BUILDFOSSIL_BIN" replay --v3 --allow-command --memory 1g failure.bfc

Supported values: 256m, 512m, 1g, 2g. Maximum: 2 GiB.
This option applies only to schema-v3 replay.

Changing memory limits can change the replay outcome.
Other Docker security restrictions remain unchanged.

For rootless Docker, set DOCKER_HOST to the rootless Docker socket
if automatic connection fails.

A successfully reproduced failure reports output similar to:

```text
Original exit code: 1
Replay exit code:   1
Outcome:            reproduced
```

`reproduced` means BuildFossil's current comparison logic considers the captured and replayed failures equivalent.

It does not prove that every aspect of the original execution environment was reconstructed.

**Only replay capsules from sources you trust.** The `--allow-command` option explicitly permits execution of the command recorded in the capsule.

## Advanced Example: External Go Dependency

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

Run the BuildFossil executable using the `BUILDFOSSIL_BIN` variable defined during installation:

```sh
"$BUILDFOSSIL_BIN" capture --v3 \
  --include go.mod \
  --include go.sum \
  --include main.go \
  -- go build ./...
```

Capture v3 checks the portable Go environment before executing the build.
Supported source targets are `darwin/arm64` and `linux/amd64`, with
`CGO_ENABLED=0` and empty `GOFLAGS`.

If the environment is unsupported, Capture exits with code `125`.
For example, on macOS where CGO is enabled by default, set
`CGO_ENABLED=0` before capture.

The output file `failure.bfc` is never overwritten. If it already exists,
move or rename it before repeating Capture. BuildFossil checks this before
executing the build and retains exclusive file creation protection.

The build is expected to fail because `undefinedFunction` does not exist.

BuildFossil saves `failure.bfc` in the current working directory when the failed build satisfies the supported capture requirements.

A successfully captured compiler failure normally preserves the original nonzero build exit code.

If capture itself fails, BuildFossil may exit with code `125` and will not produce a valid new capsule.


## What Is Inside a Capsule?

A schema-v3 `.bfc` capsule contains:

- A JSON manifest describing the captured execution.
- Selected workspace files and their integrity metadata.
- Captured build environment information.
- Zero dependency artifacts for dependency-free builds, or three
  verified artifacts per selected external Go module (up to sixteen modules).
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
- Supports up to sixteen selected external Go modules, including transitive dependencies.
- Requires explicit inclusion of `go.mod` and necessary source files.
  Existing `go.sum` files must be included; dependency-free projects
  without `go.sum` are supported.
- Does not support active Go workspaces (`go.work`), `replace`,
  or `exclude` directives.
- Requires the selected module graph and all module download artifacts
  to be available locally during offline capture.
- Rejects module graphs containing more than sixteen selected external modules.
- Captures up to 64 additional graph-only `.mod` files, with a combined
  size limit of 1 MiB, when older module versions are needed for offline
  dependency graph reconstruction.
- Requires those additional `.mod` files to be present in the local Go
  module cache and verified against `/go.mod` entries in `go.sum`.
- Does not support arbitrary-sized dependency graphs.
- Does not support general-purpose CI job capture.
- Requires workspace files to be explicitly included.
- Does not create a complete filesystem or process snapshot.
- Does not guarantee reproduction across all host and target platforms.
- Does not authenticate capsule origin.
- Is not approved for executing untrusted capsules.
- Uses non-atomic workspace consistency checks during capture;
  concurrent changes may still escape detection.
- Has unresolved security-hardening work, including filesystem race considerations.

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

Public alpha releases are available for early testing. Further security review, documentation, installation testing, and compatibility validation remain necessary before a stable release.

The next priorities include:

1. Documenting the threat model and trust boundaries.
2. Hardening the replay staging and execution path.
3. Expanding reproducibility tests.
4. Improving failure diagnostics.
5. Evaluating support for module graphs beyond eight external dependencies.

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