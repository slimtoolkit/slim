#!/usr/bin/env bash

set -ex

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
BDIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

export CGO_ENABLED=0

pushd $BDIR

BUILD_TIME="$(date -u '+%Y-%m-%d_%I:%M:%S%p')"
TAG="current"
REVISION="current"
if hash git 2>/dev/null && [ -e $BDIR/.git ]; then
  TAG="$(git describe --tags)"
  REVISION="$(git rev-parse HEAD)"
fi

LD_FLAGS="-s -w -X github.com/docker-slim/docker-slim/pkg/version.appVersionTag=${TAG} -X github.com/docker-slim/docker-slim/pkg/version.appVersionRev=${REVISION} -X github.com/docker-slim/docker-slim/pkg/version.appVersionTime=${BUILD_TIME}"

pushd ${BDIR}/cmd/docker-slim
GOOS=linux GOARCH=amd64 go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "${BDIR}/bin/linux/docker-slim" 
GOOS=darwin GOARCH=amd64 go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "${BDIR}/bin/mac/docker-slim"
GOOS=linux GOARCH=arm go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "$BDIR/bin/linux_arm/docker-slim"
GOOS=linux GOARCH=arm64 go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "$BDIR/bin/linux_arm64/docker-slim"
popd

pushd ${BDIR}/cmd/docker-slim-sensor
GOOS=linux GOARCH=amd64 go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "${BDIR}/bin/linux/docker-slim-sensor"
GOOS=linux GOARCH=arm go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "$BDIR/bin/linux_arm/docker-slim-sensor"
GOOS=linux GOARCH=arm64 go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "$BDIR/bin/linux_arm64/docker-slim-sensor"
chmod a+x "${BDIR}/bin/linux/docker-slim-sensor"
chmod a+x "$BDIR/bin/linux_arm/docker-slim-sensor"
chmod a+x "$BDIR/bin/linux_arm64/docker-slim-sensor"
popd

rm -rfv ${BDIR}/dist_mac
mkdir ${BDIR}/dist_mac
cp ${BDIR}/bin/mac/docker-slim ${BDIR}/dist_mac/docker-slim
cp ${BDIR}/bin/linux/docker-slim-sensor ${BDIR}/dist_mac/docker-slim-sensor
pushd ${BDIR}

if hash zip 2> /dev/null; then
	zip -r dist_mac.zip dist_mac -x "*.DS_Store"
fi

popd
rm -rfv ${BDIR}/dist_linux
mkdir ${BDIR}/dist_linux
cp ${BDIR}/bin/linux/docker-slim ${BDIR}/dist_linux/docker-slim
cp ${BDIR}/bin/linux/docker-slim-sensor ${BDIR}/dist_linux/docker-slim-sensor
pushd ${BDIR}
tar -czvf dist_linux.tar.gz dist_linux
popd
rm -rfv $BDIR/dist_linux_arm
mkdir $BDIR/dist_linux_arm
cp $BDIR/bin/linux_arm/docker-slim $BDIR/dist_linux_arm/docker-slim
cp $BDIR/bin/linux_arm/docker-slim-sensor $BDIR/dist_linux_arm/docker-slim-sensor
pushd ${BDIR}
tar -czvf dist_linux_arm.tar.gz dist_linux_arm
popd
rm -rfv $BDIR/dist_linux_arm64
mkdir $BDIR/dist_linux_arm64
cp $BDIR/bin/linux_arm64/docker-slim $BDIR/dist_linux_arm64/docker-slim
cp $BDIR/bin/linux_arm64/docker-slim-sensor $BDIR/dist_linux_arm64/docker-slim-sensor
pushd ${BDIR}
tar -czvf dist_linux_arm64.tar.gz dist_linux_arm64
popd

rm -rfv ${BDIR}/bin
