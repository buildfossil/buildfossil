# BuildFossil

**Save a failed CI job. Reproduce it locally.**

BuildFossil is an experimental open-source CLI for capturing
failed CI command executions into portable artifacts and
replaying them locally inside Linux containers.

> Status: Early engineering prototype. Not ready for production use.

## Goal

Help developers investigate CI-only failures without repeatedly
pushing commits and waiting for pipelines.

BuildFossil aims to capture enough execution context to attempt
local reproduction of a failed command.

Exact reproduction is not guaranteed.

## Current Capabilities

The experimental prototype supports:

- Running a wrapped command and recording its exit code.
- Capturing stdout and stderr with bounded buffers.
- Creating a local `.bfc` archive containing a JSON manifest
  and one controlled workspace fixture.
- Recording workspace metadata and SHA-256 digests.
- Validating the supported archive structure and file integrity.
- Restoring the controlled workspace into a temporary directory.
- Executing a fixed test command inside a Linux AMD64
  Docker container.
- Comparing exit codes and stderr for a deterministic failure.

An end-to-end Linux container capture to macOS ARM64 Docker
replay has been demonstrated with a synthetic failure.

## Limitations

The current implementation:

- Only supports a fixed experimental replay command.
- Only captures one predefined workspace file.
- Uses a fixed Docker image tag.
- Does not support arbitrary CI jobs.
- Does not yet integrate with GitHub Actions.
- Does not guarantee secret-free capsules.
- Does not authenticate capsule provenance.
- Does not support untrusted capsules.
- Does not provide complete filesystem or process snapshots.
- Does not guarantee identical behavior across architectures.

The `.bfc` format is experimental and subject to change.

## Requirements

For the current development prototype:

- Go 1.26 or compatible toolchain.
- Docker Engine with Linux containers.
- Linux AMD64 container execution support.

## Development

Run unit tests:

```sh
go test ./...
```

Run Docker integration tests:

```
BUILDFOSSIL_DOCKER_TEST=1 go test ./...
```

Build the CLI:

```
go build -o buildfossil ./cmd/buildfossil
```

Show the development version:

```
./buildfossil version
```

## Security

Capsules may contain sensitive information, including command arguments, logs, environment details, and workspace files.

Do not use the current prototype with production credentials, confidential repositories, or untrusted capsule files.

Docker containers are not a complete security boundary for malicious workloads.

## Roadmap

The next engineering priorities are:

1. Remove hardcoded replay fixtures.
2. Define and validate reproducible execution environments.
3. Strengthen archive handling and workspace isolation.
4. Capture real Linux CI failures.
5. Integrate with GitHub Actions.
6. Validate cross-machine replay against real workloads.

## License

BuildFossil is licensed under the Apache License, Version 2.0.

See [LICENSE](LICENSE) for the full license text.