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

LD_FLAGS="-s -w -X github.com/docker-slim/docker-slim/pkg/version.appVersionTag=${TAG} -X github.com/docker-slim/docker-slim/pkg/version.appVersionRev=${REVISION} -X github.com/docker-slim/docker-slim/pkg/version.appVersionTime=${BUILD_TIME}"

mkdir -p ${BDIR}/dist_linux/
rm -f ${BDIR}/dist_linux/*

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="${LD_FLAGS}" -mod=vendor -o "${BDIR}/dist_linux/docker-slim" "${BDIR}/cmd/docker-slim/main.go"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="${LD_FLAGS}" -mod=vendor -o "${BDIR}/dist_linux/docker-slim-sensor" "${BDIR}/cmd/docker-slim-sensor/main.go"
