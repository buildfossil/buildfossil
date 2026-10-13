#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
output_dir="${2:-dist}"

if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+-alpha\.[1-9][0-9]*$ ]]; then
  echo "release: expected version vX.Y.Z-alpha.N" >&2
  exit 2
fi

if [[ -e "$output_dir" ]]; then
  echo "release: output directory already exists: $output_dir" >&2
  exit 2
fi

mkdir -p "$output_dir"
output_dir="$(cd "$output_dir" && pwd)"

for target in darwin/arm64 linux/amd64; do
  os="${target%/*}"
  arch="${target#*/}"

  name="rew_${version}_${os}_${arch}"
  stage="$(mktemp -d)"

  echo "Building ${target}"

  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build \
    -trimpath \
    -buildvcs=false \
    -ldflags="-X main.releaseVersion=${version}" \
    -o "$stage/rew" \
    ./cmd/rew

  if [[ "$os" == "darwin" && "$(uname -s)" == "Darwin" && "$(uname -m)" == "arm64" ]] ||
     [[ "$os" == "linux" && "$(uname -s)" == "Linux" && "$(uname -m)" == "x86_64" ]]; then

    actual="$("$stage/rew" version)"

    if [[ "$actual" != "rew ${version}" ]]; then
      echo "release: wrong embedded version: $actual" >&2
      rm -rf "$stage"
      exit 1
    fi
  fi

  tar -czf "$output_dir/${name}.tar.gz" \
    -C "$stage" rew

  rm -rf "$stage"
done

(
  cd "$output_dir"

  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./*.tar.gz > SHA256SUMS
    sha256sum -c SHA256SUMS
  else
    shasum -a 256 ./*.tar.gz > SHA256SUMS
    shasum -a 256 -c SHA256SUMS
  fi
)

echo "Release artifacts ready: $output_dir"
