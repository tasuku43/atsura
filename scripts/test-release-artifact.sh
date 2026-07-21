#!/usr/bin/env bash
# Replay the bounded transform journey against one exact native release archive.
set -euo pipefail
cd "$(dirname "$0")/.."
export GO111MODULE=on
export GOENV=off
export GOEXPERIMENT=
export GOFIPS140=off
export GOFLAGS=
export GOTOOLCHAIN=local
export GOWORK=off

if [[ $# -ne 5 ]]; then
  echo "usage: $0 <tag> <revision> <goos> <goarch> <archive>" >&2
  exit 2
fi

tag=$1
revision=$2
goos=$3
goarch=$4
archive=$5

go run ./tools/releaseversion "$tag" >/dev/null
if [[ ! $revision =~ ^[0-9a-f]{40}$ ]]; then
  echo "revision must be a full lowercase Git commit SHA" >&2
  exit 2
fi
if [[ ! -f $archive ]]; then
  echo "release archive is missing: $archive" >&2
  exit 2
fi
archive=$(cd "$(dirname "$archive")" && pwd)/$(basename "$archive")

host_os=$(go env GOHOSTOS)
host_arch=$(go env GOHOSTARCH)
if [[ $host_os != "$goos" || $host_arch != "$goarch" ]]; then
  echo "native artifact replay requires host $goos/$goarch; running on $host_os/$host_arch" >&2
  exit 1
fi

journey_root=$(mktemp -d "${TMPDIR:-/tmp}/atsura-artifact-journey.XXXXXXXX")
cleanup() { rm -rf -- "$journey_root"; }
trap cleanup EXIT

fixture=$journey_root/sourcefixture
if [[ $goos == windows ]]; then
  fixture=${fixture}.exe
fi
go build -buildvcs=false -trimpath -o "$fixture" ./tools/sourcefixture

go run ./tools/artifactjourney \
  --archive "$archive" \
  --source "$fixture" \
  --tag "$tag" \
  --revision "$revision" \
  --goos "$goos" \
  --goarch "$goarch"
