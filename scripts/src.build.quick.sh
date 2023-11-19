#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
BDIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

BUILD_TIME="$(date -u '+%Y-%m-%d_%I:%M:%S%p')"
TAG="current"
REVISION="current"
if hash git 2>/dev/null && [ -e $BDIR/.git ]; then
  TAG="$(git describe --tags)"
  REVISION="$(git rev-parse HEAD)"
fi

LD_FLAGS="-s -w -X github.com/slimtoolkit/slim/pkg/version.appVersionTag=${TAG} -X github.com/slimtoolkit/slim/pkg/version.appVersionRev=${REVISION} -X github.com/slimtoolkit/slim/pkg/version.appVersionTime=${BUILD_TIME}"

go generate github.com/slimtoolkit/slim/pkg/appbom

BINDIR="${BDIR}/bin"
mkdir -p "$BINDIR"
rm -rf "${BINDIR}/"*

CGO_ENABLED=0 go build -ldflags="${LD_FLAGS}" -mod=vendor -o "${BINDIR}/slim" "${BDIR}/cmd/slim/main.go"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="${LD_FLAGS}" -mod=vendor -o "${BINDIR}/slim-sensor" "${BDIR}/cmd/slim-sensor/main.go"
