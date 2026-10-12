# BuildFossil alpha.3 Public Quick Start Validation

## Scope

This report records a manual end-to-end test of the published
BuildFossil v0.1.0-alpha.3 binary on October 12, 2026.

The test followed the dependency-free Quick Start documented in
the repository README.

This was a maintainer-operated test, not independent external
user validation.

## Environment

- Host: macOS ARM64
- Go: 1.26.6
- Docker Client: 29.6.2
- Docker Server: 29.6.2
- Docker engine: Linux ARM64
- BuildFossil: published v0.1.0-alpha.3 macOS ARM64 binary
- Replay target: Linux AMD64 via Docker
- Replay memory limit: 1 GiB

## Test Scenario

A new temporary Go workspace was created outside the BuildFossil
source repository.

The project contained a go.mod file and a main.go file with
an intentionally undefined function call.

Capture environment:

- CGO_ENABLED=0
- GOOS=linux
- GOARCH=amd64
- GOFLAGS=''
- GOWORK=off
- GOTOOLCHAIN=local
- GOPROXY=off
- GOSUMDB=off

The recorded command was `go build ./...`.

Capture explicitly included `go.mod` and `main.go`.

## Observed Results

| Observation | Result |
|---|---|
| Capture CLI exit code | 1 |
| Capsule created | Yes |
| Capsule size | Approximately 5.0 KB |
| Original build exit code | 1 |
| Replay build exit code | 1 |
| Replay CLI exit code | 0 |
| Replay outcome | reproduced |
| Replay memory | 1 GiB |

The intentional source error was:

    ./main.go:4:5: undefined: undefinedFunction

The replay command reported:

    Original exit code: 1
    Replay exit code:   1
    Outcome:            reproduced
    Replay memory:      1 GiB

## Interpretation

The published alpha.3 binary successfully captured and replayed
the intentional compiler failure in the tested environment.

The test exercised the public binary rather than a locally built
BuildFossil executable.

This confirms the documented Quick Start scenario for this
specific environment. It does not establish compatibility with
arbitrary Go projects or independently validate external-user
onboarding.

## Next Validation Goals

- Obtain feedback from an independent external developer.
- Test installation and Quick Start on a separate machine.
- Validate naturally occurring failures from real Go projects.
- Record installation friction and unclear error messages.
- Continue examining replay security and reproducibility limits.
