# BuildFossil Validation Report

This document records manually executed validation scenarios for
BuildFossil v0.1.0-alpha.2.

These results describe specific tested configurations. They do not
establish general build reproducibility or production readiness.

## Release Under Test

- Version: v0.1.0-alpha.2
- Source commit: f084207468d2b14b2ce0d4516f00a9f7320b2a0e
- Supported capture runtime: Go 1.26.6
- Replay target: Linux AMD64 Docker
- Release: https://github.com/buildfossil/buildfossil/releases/tag/v0.1.0-alpha.2

The official release binaries were used for the manual scenarios.

## Test 1: google/uuid

Repository: https://github.com/google/uuid

Source commit:

2d3c2a9cc518326daf99a383f07c4d3c44317e4d

Test configuration:

- Original project built successfully before modification.
- No external Go modules.
- No source go.sum file.
- An intentional undefined symbol was introduced into a copy.
- Capture ran on macOS ARM64.
- Replay ran locally through Docker.

Results:

- Capture preserved the original compilation failure.
- A 56 KB capsule was created.
- No source go.sum file was created.
- Replay outcome: reproduced.
- Replay CLI exit code: 0.

## Test 2: spf13/cobra

Repository: https://github.com/spf13/cobra

Source commit:

adbc8813901bba65827259daa8e22ff94ec1f30e

Test configuration:

- Original project built successfully before modification.
- Six external Go modules in the module graph.
- Existing go.mod and go.sum.
- An intentional undefined symbol was introduced into a copy.
- Capture ran on macOS ARM64 using Go 1.26.6.

Results:

- Capture exit code: 1.
- Capsule size: 643072 bytes.
- Local Docker replay outcome: reproduced.
- Replay CLI exit code: 0.

The local replay was also repeated with an empty client-side
GOMODCACHE and GOPROXY=off.

The failure remained reproducible.

## Cross-Machine Validation

The original Cobra capsule was transferred from macOS ARM64
to an independent GitHub Actions Ubuntu 24.04 AMD64 runner.

The runner downloaded the official Linux AMD64 binary
for BuildFossil v0.1.0-alpha.2.

The binary and transferred capsule were checked against
their expected SHA-256 hashes before replay.

Capsule SHA-256:

5d0c73497f5390d16552f8c7774ec7f5936b228db7f1cae7a3e5faf02db61467

Observed results:

- Original exit code: 1
- Replay exit code: 1
- Outcome: reproduced
- BuildFossil replay exit code: 0
- GitHub Actions conclusion: success

The test ran in a separate private repository.
Its CI logs are not publicly accessible.

## Limitations

These tests do not establish:

- Reproduction of naturally occurring CI failures.
- Compatibility with arbitrary Go projects.
- Support for more than eight external modules.
- General cross-platform build reproducibility.
- Complete reconstruction of the original environment.
- Safety when replaying untrusted capsules.
- Reproducibility across arbitrary Docker and host configurations.

The compilation failures in both projects were intentionally introduced.

## Next Validation Priorities

1. Reproduce a naturally occurring Go build failure.
2. Test additional supported dependency graphs.
3. Investigate failures that produce different replay outcomes.
4. Evaluate capsule portability under additional clean environments.
5. Continue security and reproducibility hardening.

---

## Development Validation — 16 Real Go Modules and Offline Graph Metadata

This section documents local development-branch validation performed on
October 11, 2026. It is not a validation claim for the published
v0.1.0-alpha.2 release.

Configuration:

- Capture host: macOS ARM64.
- Go toolchain: Go 1.26.6.
- Build target: Linux AMD64, `CGO_ENABLED=0`.
- Command: `go build ./...`.
- Selected external Go modules: 16.
- Additional graph-only `.mod` files: 3, totalling 170 bytes.
- Intentional failure: undefined `buildFossilMissingSymbol`.
- Replay: Docker, with Go module proxy access disabled.

The three additional graph-only module versions were:

- `github.com/mattn/go-isatty@v0.0.16`
- `golang.org/x/sys@v0.0.0-20220811171246-fbc7d0a398ab`
- `golang.org/x/sys@v0.6.0`

All three `.mod` artifacts were read from the local Go module cache
and verified against their recorded module metadata checksums.

The recorded and replayed errors matched byte-for-byte:

    # example.com/buildfossil-real-modules-16
    ./main.go:23:2: undefined: buildFossilMissingSymbol

Observed results:

| Replay memory limit | Outcome | CLI exit code |
|---|---|---|
| 256 MiB | reproduced | 0 |
| 512 MiB | reproduced | 0 |
| 1 GiB | reproduced | 0 |

The current development reader also successfully replayed a
v0.1.0-alpha.2 Cobra capsule containing six external Go modules.

The full Docker-gated Go test suite passed on the development branch using:

    BUILDFOSSIL_DOCKER_TEST=1 go test ./... -count=1 -timeout=10m

These results confirm the tested configuration only. They do not
establish general reproducibility across arbitrary Go builds,
cross-machine environments, or naturally occurring CI failures.
