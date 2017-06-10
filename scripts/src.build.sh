#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
SDIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

export CGO_ENABLED=0

source $SDIR/env.sh
BDIR_GOPATH=$BDIR/_gopath/src/github.com/docker-slim/docker-slim

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
#gox -osarch="linux/arm" -output="$BDIR_GOPATH/bin/linux_arm/docker-slim"
popd
pushd $BDIR_GOPATH/apps/docker-slim-sensor
gox -osarch="linux/amd64" -output="$BDIR_GOPATH/bin/linux/docker-slim-sensor"
#gox -osarch="linux/arm" -output="$BDIR_GOPATH/bin/linux_arm/docker-slim-sensor"
popd
rm -rfv $BDIR_GOPATH/dist_mac
mkdir $BDIR_GOPATH/dist_mac
cp $BDIR_GOPATH/bin/mac/docker-slim $BDIR_GOPATH/dist_mac/docker-slim
cp $BDIR_GOPATH/bin/linux/docker-slim-sensor $BDIR_GOPATH/dist_mac/docker-slim-sensor
rm -rfv $BDIR_GOPATH/dist_linux
mkdir $BDIR_GOPATH/dist_linux
cp $BDIR_GOPATH/bin/linux/docker-slim $BDIR_GOPATH/dist_linux/docker-slim
cp $BDIR_GOPATH/bin/linux/docker-slim-sensor $BDIR_GOPATH/dist_linux/docker-slim-sensor
#rm -rfv $BDIR_GOPATH/dist_linux_arm
#mkdir $BDIR_GOPATH/dist_linux_arm
#cp $BDIR_GOPATH/bin/linux_arm/docker-slim $BDIR_GOPATH/dist_linux_arm/docker-slim
#cp $BDIR_GOPATH/bin/linux_arm/docker-slim-sensor $BDIR_GOPATH/dist_linux_arm/docker-slim-sensor
rm -rfv $BDIR_GOPATH/bin
