# BuildFossil — Supported Scope and Limitations

## Status

This document describes the current experimental schema-v3 implementation.

BuildFossil is not production-ready. Supported scenarios are intentionally narrow, and successful reproduction is not guaranteed outside the tested configurations.

## Support Matrix

| Component | Current support |
|---|---|
| Capture language | Go |
| Go toolchain | Go 1.26.6 |
| Captured command | `go build ./...` |
| External modules | Exactly one direct require entry in go.mod; transitive dependency graphs are not supported |
| Dependency artifacts | `.zip`, `.mod`, `.info` |
| Dependency verification | Go module h1 checksums and artifact integrity |
| Workspace capture | Explicitly included regular files |
| Capsule format | Experimental schema-v3 `.bfc` |
| Replay target | Linux AMD64 |
| Replay runtime | Pinned Go Docker image |
| Container networking | Disabled |
| Replay authorization | Explicit `--allow-command` |
| Failure comparison | Exit code and stderr |
| Capsule provenance authentication | Not supported |
| Untrusted capsule execution | Not supported as a safe use case |

## Verified Cross-Platform Scenario

GitHub Actions currently validates:

1. Capture execution on a macOS ARM64 runner.
2. Compilation targeting Linux AMD64.
3. `CGO_ENABLED=0`.
4. One external Go module dependency.
5. Capsule transfer through a GitHub Actions artifact.
6. Replay inside a Linux AMD64 Docker environment.
7. A `reproduced` outcome.

This is a specific cross-host reproduction scenario.

It does not establish universal cross-platform reproducibility.

## Command Restrictions

Schema-v3 currently accepts only:

```sh
go build ./...
```

Other commands are outside the supported scope, including:

- `go test ./...`
- `go run .`
- `go generate`
- `make`
- Shell scripts and shell wrappers
- Arbitrary compiler invocations
- Go build commands with additional flags

These restrictions allow the implementation to make more explicit assumptions about the execution environment.

## Go Dependency Restrictions

The current prototype requires exactly one direct external module dependency.

Unsupported or unverified configurations include:

- Multiple direct dependencies
- General transitive dependency graphs
- Workspace configurations using `go.work`
- Local replacements
- Unsupported `replace` or `exclude` directives
- Private module authentication workflows
- Vendor-only builds
- Arbitrary dependency cache formats

Dependency content is verified, but verification does not establish whether the dependency source code is trustworthy.

## Workspace Restrictions

The user selects files through `--include`.

The resulting capsule does not automatically contain the entire repository or working environment.

Consequently:

- Omitted source files can prevent reproduction.
- Generated files are not automatically reconstructed.
- External filesystem paths are not fully captured.
- Local caches outside the supported dependency mechanism are not captured.
- Processes and background services are not snapshotted.
- The original operating system is not reproduced.

For a supported v3 capture, `go.mod` and `go.sum` must be included.

## Go Environment Restrictions

The supported portability configuration is deliberately constrained.

The tested cross-platform scenario uses:

```sh
GOOS=linux
GOARCH=amd64
CGO_ENABLED=0
GOFLAGS=
GOTOOLCHAIN=local
```

Different compiler flags, build tags, architecture-specific sources, and environment-sensitive build behavior may produce different results.

The current implementation does not reproduce every property of the original CI runner.

## Replay Restrictions

Replay requires a working Docker environment capable of executing Linux AMD64 containers.

The Docker image is pinned for reproducibility.

The replay container has network access disabled. However, the host may require network access to fetch the replay image.

The replay process may perform preparation steps, including offline module-cache setup, before running the captured build.

Therefore, replay should not be interpreted as an exact record-and-reexecute mechanism for every operation originally performed by Go.

## Failure Comparison

A reproduced outcome indicates agreement according to BuildFossil's implemented comparison criteria.

For the supported deterministic compiler-failure scenario, exit code and stderr are central to this comparison.

Known sources of divergence include:

- Non-deterministic diagnostics
- Environment-sensitive compiler output
- Missing selected source files
- Different build targets
- Unsupported module configurations
- Differences between host and container execution environments

A different result is not necessarily a BuildFossil defect; it can indicate that the captured environment was insufficient to reproduce the original failure.

## Security Limitations

BuildFossil is not currently a safe executor for arbitrary untrusted capsules.

Important limitations include:

- No authenticated capsule provenance.
- No guarantee that included source code is harmless.
- No comprehensive secret redaction.
- No complete protection against concurrent staging modification.
- No guarantee of isolation against container-runtime or kernel vulnerabilities.
- No complete protection against all resource-exhaustion attacks.

See `SECURITY.md` for the complete experimental security policy.

## Unsupported Claims

The current implementation does not claim:

- Complete CI job reproduction.
- General-purpose build-system virtualization.
- Reproduction of arbitrary Docker builds.
- Bit-for-bit rebuilding of successful binaries.
- Support for Kubernetes workloads.
- Arbitrary operating-system and architecture compatibility.
- Support for every Go module configuration.
- Safe execution of unknown third-party capsules.

These capabilities require separate engineering and validation.

## Before Public Alpha

The following work remains necessary before treating BuildFossil as a public alpha candidate:

1. Security review of capsule parsing and restoration.
2. Hardening staging and Docker execution boundaries.
3. Additional negative tests and failure-injection coverage.
4. Improved installation and release procedures.
5. Testing clean installations on supported platforms.
6. Versioned release artifacts and checksums.
7. Clear vulnerability reporting and maintenance procedures.

The current project should be treated as an experimental, limited-scope developer tool.
