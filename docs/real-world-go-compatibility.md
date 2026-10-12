# Real-World Go Compatibility — Development Validation

## BuildFossil v0.1.0-alpha.3 Validation

The larger-project implementation was integrated through
[Pull Request #37](https://github.com/buildfossil/buildfossil/pull/37)
and released from commit `7c47578978e109e60ae2c20b955442c365601cf4`.

The schema-v3 implementation supports:

- Up to 128 selected external Go modules.
- Up to 64 modules with complete ZIP artifacts.
- A combined 64 MiB module artifact budget.
- A maximum 96 MiB schema-v3 capsule.
- Up to 256 explicitly included workspace files.

### Go-Task v3.40.1 — Successful Offline Reproduction

A local validation used the upstream Go-Task v3.40.1 source at
`32fa3a01561b16aee9c87ecf0b49be5b733bb3d1`.

Observed results:

- Workspace: 96 included files.
- A failed `go build ./...` was captured into a schema-v3 capsule.
- The capsule was replayed using the pinned Linux AMD64 Go runtime
  in Docker with a 2 GiB container memory limit.
- Original exit code: `1`.
- Replay exit code: `1`.
- Replay outcome: `reproduced`.

This establishes reproduction of the tested failure, not general
compatibility with every Go-Task build or Go project.

### Release Verification

The alpha.3 release candidate passed the tagged GitHub Actions
validation workflow, including Go tests, static analysis, Docker E2E,
archive creation, and SHA-256 verification.

The public macOS ARM64 and Linux AMD64 release archives were downloaded
from GitHub Releases and successfully verified against `SHA256SUMS`.
The macOS ARM64 binary reported `buildfossil v0.1.0-alpha.3`.

Release:
https://github.com/buildfossil/buildfossil/releases/tag/v0.1.0-alpha.3

Tagged validation:
https://github.com/buildfossil/buildfossil/actions/runs/38202436931

These results do not establish support for arbitrary Go projects,
complete CI environments, or execution of untrusted capsules.

---

## Scope

This report records historical local compatibility checks performed
against BuildFossil development commit `39ee427` in October 2026.

The original results below describe the earlier implementation, not the
published `v0.1.0-alpha.3` binaries. See the alpha.3 validation section
for a separately verified larger-project reproduction.

The tested implementation supports:

- Go 1.26.6.
- Capture command: `go build ./...`.
- Linux AMD64 build target with `CGO_ENABLED=0`.
- Explicit workspace file inclusion.
- Up to 64 selected external modules.
- Up to 16 full module ZIP artifacts.
- A 16 MiB maximum capsule size.
- Offline Docker replay with explicit command authorization.

## Results

| Project | Pinned revision | Dependency findings | Result |
|---|---|---|---|
| Sonic v1.14.1 | `53b981bdd59c34d4ee4bd40915101c5c5057dd04` | 17 selected; 5 full ZIP | Reproduced |
| Cobra v1.10.2 | `88b30ab89da2d0d0abb153818746c5a2d30eccec` | 6 selected; 4 full ZIP | Reproduced (intentional failure) |
| Caddy v2.10.2 | Tag `2800fc8b6f56bcfe7200692bb1b94706771338ed` | 423 selected; 140 build modules | Capacity preflight failed |
| Hugo v0.145.0 | `666444f0a52132f9fec9f71cf25b441cc6a4f355` | 387 selected | Capacity preflight failed |
| Restic v0.18.0 | Commit `d401ad6c1ec93a8fbcfcd9f6855a7a4468878ec2` | 274 selected | Capacity preflight failed |
| Go-Task v3.40.1 | `32fa3a01561b16aee9c87ecf0b49be5b733bb3d1` | 82 selected; 48 build modules | Capacity preflight failed |
| Lazygit v0.45.2 | `c03b89227092b852d50015d289a7c6d8c69811c5` | 155 selected; vendor directory present | Capacity preflight failed |

The Restic annotated tag object is
`7f723603e56655a6fc383dc20e5ee6655e7ed149`.

## Successful Reproductions

### Sonic v1.14.1

The original checkout was not modified.

A naturally occurring Go compilation failure involving
`GoMapIterator` symbols was captured and reproduced.

Observed results:

- Selected modules: 17.
- Full ZIP modules: 5.
- Metadata-only selected modules: 12.
- Original exit code: 1.
- Replay exit code: 1.
- Replay outcome: `reproduced`.

The recorded Sonic capsule was also exercised using the
Docker-gated regression test.

### Cobra v1.10.2

The pinned original project built successfully and remained clean.

A separate fixture copy introduced one undefined symbol:
`undefinedBuildFossilSymbol`.

Observed results:

- Selected modules: 6.
- Full ZIP modules: 4.
- Metadata-only selected modules: 2.
- Capsule size: 916992 bytes.
- Original exit code: 1.
- Replay exit code: 1.
- Replay outcome: `reproduced`.
- Replay CLI exit code: 0.

This is a controlled failure, not an upstream Cobra defect.

## Capacity Preflight

The remaining projects were inspected using Go module and/or
package dependency listing.

These projects were **not** subjected to full BuildFossil
Capture → Docker Replay.

Their unsupported-capacity classification follows from
observed module counts exceeding the current implementation
limits, not from an observed BuildFossil capture failure.

Build-module counts obtained using `go list -deps -e -json ./...`
are estimates of required full module artifacts; they are
not measurements from completed BuildFossil captures.

The Lazygit repository contains a vendor directory.
Its module graph was inspected using `go list -mod=mod -m all`;
vendor-mode Capture compatibility was not established.

## Interpretation

A `reproduced` outcome means the recorded and replayed failures
matched BuildFossil's current comparison criteria in that test.

It does not establish:

- Compatibility with arbitrary Go projects.
- Reproduction of complete CI environments.
- Bit-for-bit reproducibility of successful builds.
- Safe execution of untrusted capsules.
- Support for arbitrary Go toolchains or target platforms.

## Future Engineering

Large-project support requires investigation of:

- Selected module graph capacity.
- Full ZIP artifact capacity and total size budgets.
- Streaming and bounded temporary-file processing.
- Capsule verification memory and CPU costs.
- ZIP decompression and entry-count limits.
- Offline Docker replay resource budgets.
- Backward compatibility with existing schema-v3 capsules.

Increasing numerical limits alone is not sufficient.

See `docs/limitations.md` and `docs/security-model.md`
for broader scope and security information.
