#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
#SDIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"
BDIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

export CGO_ENABLED=0

#source $SDIR/env.sh
#BDIR_GOPATH=$BDIR/_gopath/src/github.com/docker-slim/docker-slim
#BDIR_GOPATH=$BDIR

pushd $BDIR

#used only in the builder container, so the link trick is not really necessary
#rm -rf _gopath
#mkdir _gopath
#pushd $BDIR/_gopath
#mkdir -p src/github.com/docker-slim
#ln -sf $BDIR src/github.com/docker-slim/docker-slim
#popd

BUILD_TIME="$(date -u '+%Y-%m-%d_%I:%M:%S%p')"
TAG="current"
revision="current"
if hash git 2>/dev/null && [ -e $BDIR/.git ]; then
  TAG="$(git describe --tags)"
  revision="$(git rev-parse HEAD)"
fi

pushd $BDIR/cmd/docker-slim

LD_FLAGS="-s -w -X github.com/docker-slim/docker-slim/pkg/version.appVersionTag=${TAG} -X github.com/docker-slim/docker-slim/pkg/version.appVersionRev=${REVISION} -X github.com/docker-slim/docker-slim/pkg/version.appVersionTime=${BUILD_TIME}"

GOOS=linux GOARCH=amd64 go build -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "${BDIR}/bin/linux/docker-slim" 
GOOS=darwin GOARCH=amd64 go build -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "${BDIR}/bin/mac/docker-slim"
GOOS=linux GOARCH=arm go build -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "$BDIR/bin/linux_arm/docker-slim"

popd
pushd $BDIR/cmd/docker-slim-sensor

GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "${BDIR}/bin/linux/docker-slim-sensor"
GOOS=linux GOARCH=arm go build -ldflags="-s -w" -o "$BDIR/bin/linux_arm/docker-slim-sensor"
cp "$BDIR/bin/linux/docker-slim-sensor" "$BDIR/bin/mac/docker-slim-sensor"

popd
popd
