#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
SDIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

source $SDIR/env.sh
BDIR_GOPATH=$BDIR/_gopath/src/github.com/docker-slim/docker-slim

pushd $BDIR

#used only in the builder container, so the link trick is not really necessary
rm -rf _gopath
mkdir _gopath
pushd $BDIR/_gopath
mkdir -p src/github.com/docker-slim
ln -sf $BDIR src/github.com/docker-slim/docker-slim
popd

pushd $BDIR_GOPATH/cmd/docker-slim
BUILD_TIME="$(date -u '+%Y-%m-%d_%I:%M:%S%p')"
TAG="current"
revision="current"
if hash git 2>/dev/null && [ -e $BDIR_GOPATH/.git ]; then
  TAG="$(git describe --tags)"
  revision="$(git rev-parse HEAD)"
fi

LD_FLAGS="-X github.com/docker-slim/docker-slim/utils.appVersionTag=${TAG} -X github.com/docker-slim/docker-slim/utils.appVersionRev=${REVISION} -X github.com/docker-slim/docker-slim/utils.appVersionTime=${BUILD_TIME}"

gox -osarch="linux/amd64" -ldflags "${LD_FLAGS}" -output "$BDIR_GOPATH/bin/linux/docker-slim"
gox -osarch="darwin/amd64" -ldflags "${LD_FLAGS}" -output "$BDIR_GOPATH/bin/mac/docker-slim"
popd
pushd $BDIR_GOPATH/cmd/docker-slim-sensor
gox -osarch="linux/amd64" -output="$BDIR_GOPATH/bin/linux/docker-slim-sensor"
popd
popd
