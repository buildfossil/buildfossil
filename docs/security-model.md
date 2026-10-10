# Replay Staging Security Model

This document describes the trust assumptions and integrity boundaries
of BuildFossil schema-v3 replay staging.

For general security policy, vulnerability reporting, and safe usage
guidance, see [SECURITY.md](../SECURITY.md).

BuildFossil remains experimental and is not a security sandbox for
arbitrary hostile workloads.

## Scope

This document covers the path from a verified schema-v3 capsule to
execution inside the Docker replay container.

The current implementation supports a constrained Go build replay
with one direct external module dependency.

## Staging Lifecycle

Schema-v3 replay currently:

1. Reads and validates the capsule.
2. Creates a private temporary staging root.
3. Restores verified workspace files.
4. Prepares a verified Go module download cache.
5. Re-verifies the restored workspace.
6. Prepares the Docker replay image and container.
7. Bind-mounts the staged workspace and module cache read-only.
8. Executes the captured build command.
9. Cleans up the temporary staging root and execution container.

## Integrity Guarantees

The implementation validates capsule structure, recorded file metadata,
workspace contents, dependency artifacts, and supported paths.

Workspace restoration uses root-relative filesystem operations and
exclusive file creation to reduce path traversal and replacement risks.

Staging verification detects unexpected files, symbolic links,
missing files, and content changes present at verification time.

These checks do not guarantee immutable host files throughout replay.

## Trust Assumptions

The security model assumes:

- The host operating system and Docker daemon are trusted.
- The replay process runs without root privileges.
- Private staging directories are not accessible to unrelated
  unprivileged operating-system users.
- Processes running under the same host UID are not considered
  mutually isolated security principals.
- Privileged host processes are outside the staging isolation boundary.

## Remaining TOCTOU Concern

There is an interval between verification of the staged workspace
and Docker consuming its contents.

Read-only bind mounts restrict writes from the container, but do not
prevent a sufficiently privileged host process from modifying the
mounted source files.

The same consideration applies to the staged Go module cache.

Repeating verification immediately before container creation would
reduce the interval but would not eliminate this class of race.

No claim of absolute staging immutability is made.

## Docker Boundary

Replay uses restricted Docker execution, including disabled container
networking, a read-only root filesystem, non-root execution, dropped
capabilities, and resource restrictions.

These protections do not make Docker equivalent to a dedicated
virtual-machine isolation boundary.

The Docker daemon, container runtime, and host kernel remain trusted
components of the execution environment.

## Out of Scope

The current design does not defend against:

- Host processes with sufficient filesystem permissions to modify
  staging concurrently.
- A compromised or malicious Docker daemon.
- Container-runtime or host-kernel vulnerabilities.
- Malicious source code merely because its archive checksums are valid.
- Arbitrary execution of untrusted capsules as a supported safe workflow.

## Future Hardening

Potential improvements require separate threat analysis and tests:

- Reducing the time between verification and execution.
- Evaluating isolation mechanisms that do not depend on mutable
  host bind-mounted inputs.
- Additional adversarial tests for staging and module-cache behavior.
- Stronger isolation for scenarios involving untrusted workloads.

Any proposed implementation must preserve offline replay,
cross-platform capture/replay behavior, and the existing supported
Go build scenario.
