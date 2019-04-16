#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
SDIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

export CGO_ENABLED=0

source ${SDIR}/env.sh
BDIR_GOPATH=${BDIR}/_gopath/src/github.com/docker-slim/docker-slim

pushd ${BDIR_GOPATH}/cmd/docker-slim
BUILD_TIME="$(date -u '+%Y-%m-%d_%I:%M:%S%p')"
TAG="current"
REVISION="current"
if hash git 2>/dev/null && [ -e ${BDIR_GOPATH}/.git ]; then
  TAG="$(git describe --tags)"
  REVISION="$(git rev-parse HEAD)"
fi

LD_FLAGS="-s -w -X github.com/docker-slim/docker-slim/pkg/version.appVersionTag=${TAG} -X github.com/docker-slim/docker-slim/pkg/version.appVersionRev=${REVISION} -X github.com/docker-slim/docker-slim/pkg/version.appVersionTime=${BUILD_TIME}"

gox -osarch="linux/amd64" -ldflags "${LD_FLAGS}" -output "${BDIR_GOPATH}/bin/linux/docker-slim" 
gox -osarch="darwin/amd64" -ldflags "${LD_FLAGS}" -output "${BDIR_GOPATH}/bin/mac/docker-slim"
gox -osarch="linux/arm" -output "$BDIR_GOPATH/bin/linux_arm/docker-slim"
popd
pushd ${BDIR_GOPATH}/cmd/docker-slim-sensor
gox -osarch="linux/amd64" -ldflags "-s -w" -output="${BDIR_GOPATH}/bin/linux/docker-slim-sensor"
gox -osarch="linux/arm" -output "$BDIR_GOPATH/bin/linux_arm/docker-slim-sensor"
popd
rm -rfv ${BDIR_GOPATH}/dist_mac
mkdir ${BDIR_GOPATH}/dist_mac
cp ${BDIR_GOPATH}/bin/mac/docker-slim ${BDIR_GOPATH}/dist_mac/docker-slim
cp ${BDIR_GOPATH}/bin/linux/docker-slim-sensor ${BDIR_GOPATH}/dist_mac/docker-slim-sensor
pushd ${BDIR_GOPATH}
zip -r dist_mac.zip dist_mac -x "*.DS_Store"
popd
rm -rfv ${BDIR_GOPATH}/dist_linux
mkdir ${BDIR_GOPATH}/dist_linux
cp ${BDIR_GOPATH}/bin/linux/docker-slim ${BDIR_GOPATH}/dist_linux/docker-slim
cp ${BDIR_GOPATH}/bin/linux/docker-slim-sensor ${BDIR_GOPATH}/dist_linux/docker-slim-sensor
rm -rfv $BDIR_GOPATH/dist_linux_arm
mkdir $BDIR_GOPATH/dist_linux_arm
cp $BDIR_GOPATH/bin/linux_arm/docker-slim $BDIR_GOPATH/dist_linux_arm/docker-slim
cp $BDIR_GOPATH/bin/linux_arm/docker-slim-sensor $BDIR_GOPATH/dist_linux_arm/docker-slim-sensor
pushd ${BDIR_GOPATH}
tar -czvf dist_linux.tar.gz dist_linux
popd
rm -rfv ${BDIR_GOPATH}/bin
