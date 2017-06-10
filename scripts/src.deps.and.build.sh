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

pushd $BDIR_GOPATH/apps/docker-slim
build_time="$(date -u '+%Y-%m-%d_%I:%M:%S%p')"
tag="current"
revision="current"
if hash git 2>/dev/null && [ -e $BDIR_GOPATH/.git ]; then
  tag="$(git describe --tags)"
  revision="$(git rev-parse HEAD)"
fi
gox -osarch="linux/amd64" -output="$BDIR_GOPATH/bin/linux/docker-slim" -ldflags="-X utils.appVersionTag=$tag -X utils.appVersionRev=$revision -X utils.appVersionTime=$build_time"
gox -osarch="darwin/amd64" -output="$BDIR_GOPATH/bin/mac/docker-slim" -ldflags="-X utils.appVersionTag=$tag -X utils.appVersionRev=$revision -X utils.appVersionTime=$build_time"
popd
pushd $BDIR_GOPATH/apps/docker-slim-sensor
gox -osarch="linux/amd64" -output="$BDIR_GOPATH/bin/linux/docker-slim-sensor"
popd
popd
