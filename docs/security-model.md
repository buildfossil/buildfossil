
# Replay Snapshot Security Model

This document describes the integrity boundaries and trust assumptions
of BuildFossil schema-v3 offline replay.

For the general security policy, vulnerability reporting, and safe usage
guidance, see [SECURITY.md](../SECURITY.md).

BuildFossil remains experimental. Docker execution is not a security
sandbox for arbitrary hostile workloads.

## Scope

Schema-v3 replay currently supports a constrained Go build scenario
with zero or one direct external module dependency.

For dependency-free builds, the manifest declares no Go modules,
the capsule contains no dependency artifacts, and replay constructs
an empty module-cache TAR. For builds with one external module,
the existing ZIP, MOD, INFO, and go.sum integrity checks remain required.

This document describes the path from a verified `.bfc` capsule to
execution in a restricted Docker container.

## Snapshot Lifecycle

The current schema-v3 replay implementation:

1. Opens and verifies the `.bfc` capsule.
2. Validates workspace metadata, file contents, and Go module artifacts.
3. Builds a workspace TAR from verified capsule bytes.
4. Builds a Go module download-cache TAR from verified artifacts,
   or an empty TAR for dependency-free builds.
5. Creates two uniquely named Docker-managed volumes.
6. Uploads the TAR archives using the Docker Engine API.
7. Removes the temporary preparation containers.
8. Creates a restricted execution container with both volumes mounted
   read-only.
9. Copies the module cache into a writable container tmpfs.
10. Runs offline Go module preparation and executes the captured command.
11. Compares the replay exit code and stderr with the captured failure.
12. Removes the execution container and Docker volumes.

The active schema-v3 replay path does not restore the workspace into
host staging directories or use host bind mounts for replay inputs.

The execution container explicitly sets `GOWORK=off`, empty
`GOFLAGS`, `GOPROXY=off`, `GOSUMDB=off`, and
`GOTOOLCHAIN=local`.

Capture rejects an active Go workspace detected through
`go env GOWORK`. This check does not eliminate all possible
environment or filesystem races between inspection and execution.

## Integrity Properties

BuildFossil verifies capsule structure, recorded file metadata,
workspace contents, and dependency artifacts.

Snapshot archives are constructed from previously verified data held
by the replay process, rather than by rereading mutable host staging
files.

The snapshot builders repeat relevant content and checksum validation
before constructing TAR archives.

This removes the previous dependency on host staging directories
remaining unchanged between verification and Docker consumption.

However, this does not establish absolute snapshot immutability.

Docker-managed volumes are writable during preparation. They are
mounted read-only in the execution container, but the Docker daemon
and other sufficiently privileged actors may still alter their
contents.

## Docker Execution Restrictions

The execution container uses:

- A pinned Go replay image.
- Linux AMD64 execution.
- Disabled container networking.
- Non-root UID:GID.
- A read-only container root filesystem.
- Read-only workspace and module-cache volumes.
- No inherited host workspace bind mount.
- Dropped Linux capabilities.
- `no-new-privileges`.
- CPU, memory, and PID limits.
- Writable tmpfs mounts for temporary files and Go caches.

Both execution volume mounts use Docker's `NoCopy` option.

The preparation containers have different access requirements
because they are used to populate Docker volumes.

These controls reduce risk but do not make Docker equivalent to
a dedicated virtual-machine isolation boundary.

## Trust Assumptions

The design assumes:

- The host operating system and Docker daemon are trusted.
- The user trusts the source of the capsule.
- The replay process does not run as root.
- The container runtime and host kernel are correctly maintained.
- Privileged host processes and Docker administrators are outside
  the snapshot isolation boundary.

Processes with access to the Docker daemon may be able to alter
volumes or interfere with replay.

## Resource Cleanup

BuildFossil attempts to remove:

1. Temporary preparation containers.
2. The execution container.
3. Both Docker-managed volumes.

Cleanup uses independent timeout contexts so that cancellation of
the replay context does not automatically prevent resource removal.

Errors encountered during cleanup are reported alongside execution
errors when possible.

Cleanup is best-effort, not transactional.

If the Docker daemon becomes unavailable, a process terminates
unexpectedly, or an API operation has an indeterminate outcome,
resources may remain and require manual inspection.

Docker snapshot volumes can be inspected using:

```sh
docker volume ls --filter label=org.buildfossil.component=snapshot
```

## Remaining Risks

The snapshot architecture does not protect against:

- A compromised or malicious Docker daemon.
- Host-kernel or container-runtime vulnerabilities.
- Malicious source code with valid checksums.
- Arbitrary execution commands in untrusted capsules.
- Resource exhaustion outside configured limits.
- Every possible Docker API failure or cleanup race.
- Concurrent volume modification by sufficiently privileged actors.
- Unexpected process termination before cleanup completes.

SHA-256 provides integrity checking, not publisher authentication.

BuildFossil does not currently provide signed capsules or a
trusted publisher identity system.

## Out of Scope

The current experimental implementation is not intended for:

- Arbitrary hostile code execution.
- Untrusted third-party capsule execution.
- General-purpose dependency graph replay.
- Multi-platform container execution.
- Strong isolation from the Docker daemon or host kernel.

## Future Hardening

Areas requiring further review include:

- Linux AMD64 CI integration tests.
- Cleanup under Docker daemon disconnection.
- Resource reconciliation for orphaned volumes and containers.
- Adversarial tests for snapshot construction and extraction.
- Stronger isolation for untrusted execution.
- Independent security review before a stable release.
