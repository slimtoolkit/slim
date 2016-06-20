#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
SDIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

source $SDIR/env.sh
pushd $BDIR/apps/docker-slim
build_time="$(date -u '+%Y-%m-%d_%I:%M:%S%p')"
tag="current"
revision="current"
if hash git 2>/dev/null && [ -e $BDIR/.git ]; then
  tag="$(git describe --tags)"
  revision="$(git rev-parse HEAD)"
fi
gox -osarch="linux/amd64" -output="$BDIR/bin/linux/docker-slim" -ldflags="-X main.appVersionTag $tag -X main.appVersionRev $revision -X main.appVersionTime $build_time"
gox -osarch="darwin/amd64" -output="$BDIR/bin/mac/docker-slim" -ldflags="-X main.appVersionTag $tag -X main.appVersionRev $revision -X main.appVersionTime $build_time"
#gox -osarch="linux/arm" -output="$BDIR/bin/linux_arm/docker-slim"
popd
pushd $BDIR/apps/docker-slim-sensor
gox -osarch="linux/amd64" -output="$BDIR/bin/linux/docker-slim-sensor"
#gox -osarch="linux/arm" -output="$BDIR/bin/linux_arm/docker-slim-sensor"
popd
rm -rfv $BDIR/dist_mac
mkdir $BDIR/dist_mac
cp $BDIR/bin/mac/docker-slim $BDIR/dist_mac/docker-slim
cp $BDIR/bin/linux/docker-slim-sensor $BDIR/dist_mac/docker-slim-sensor
rm -rfv $BDIR/dist_linux
mkdir $BDIR/dist_linux
cp $BDIR/bin/linux/docker-slim $BDIR/dist_linux/docker-slim
cp $BDIR/bin/linux/docker-slim-sensor $BDIR/dist_linux/docker-slim-sensor
#rm -rfv $BDIR/dist_linux_arm
#mkdir $BDIR/dist_linux_arm
#cp $BDIR/bin/linux_arm/docker-slim $BDIR/dist_linux_arm/docker-slim
#cp $BDIR/bin/linux_arm/docker-slim-sensor $BDIR/dist_linux_arm/docker-slim-sensor
rm -rfv $BDIR/bin
