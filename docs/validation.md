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
