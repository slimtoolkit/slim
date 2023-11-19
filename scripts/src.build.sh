#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
BDIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

export CGO_ENABLED=0

pushd $BDIR

BUILD_TIME="$(date -u '+%Y-%m-%d_%I:%M:%S%p')"
TAG="current"
REVISION="current"
if hash git 2>/dev/null && [ -e $BDIR/.git ]; then
  TAG="$(git describe --tags --always)"
  REVISION="$(git rev-parse HEAD)"
fi

LD_FLAGS="-s -w -X github.com/slimtoolkit/slim/pkg/version.appVersionTag=${TAG} -X github.com/slimtoolkit/slim/pkg/version.appVersionRev=${REVISION} -X github.com/slimtoolkit/slim/pkg/version.appVersionTime=${BUILD_TIME}"

go generate github.com/slimtoolkit/slim/pkg/appbom

pushd ${BDIR}/cmd/slim
GOOS=linux GOARCH=amd64 go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "${BDIR}/bin/linux/slim" 
GOOS=darwin GOARCH=amd64 go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "${BDIR}/bin/mac/slim"
GOOS=linux GOARCH=arm go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "$BDIR/bin/linux_arm/slim"
GOOS=linux GOARCH=arm64 go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "$BDIR/bin/linux_arm64/slim"
popd

pushd ${BDIR}/cmd/slim-sensor
GOOS=linux GOARCH=amd64 go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "${BDIR}/bin/linux/slim-sensor"
GOOS=linux GOARCH=arm go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "$BDIR/bin/linux_arm/slim-sensor"
GOOS=linux GOARCH=arm64 go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "$BDIR/bin/linux_arm64/slim-sensor"
chmod a+x "${BDIR}/bin/linux/slim-sensor"
chmod a+x "$BDIR/bin/linux_arm/slim-sensor"
chmod a+x "$BDIR/bin/linux_arm64/slim-sensor"
popd

rm -rfv ${BDIR}/dist_mac
mkdir ${BDIR}/dist_mac
cp ${BDIR}/bin/mac/slim ${BDIR}/dist_mac/slim
cp ${BDIR}/bin/linux/slim-sensor ${BDIR}/dist_mac/slim-sensor
pushd ${BDIR}/dist_mac
ln -s slim docker-slim
popd
pushd ${BDIR}
if hash zip 2> /dev/null; then
	zip -r dist_mac.zip dist_mac -x "*.DS_Store"
fi
popd

rm -rfv ${BDIR}/dist_linux
mkdir ${BDIR}/dist_linux
cp ${BDIR}/bin/linux/slim ${BDIR}/dist_linux/slim
cp ${BDIR}/bin/linux/slim-sensor ${BDIR}/dist_linux/slim-sensor
pushd ${BDIR}/dist_linux
ln -s slim docker-slim
popd
pushd ${BDIR}
tar -czvf dist_linux.tar.gz dist_linux
popd

rm -rfv $BDIR/dist_linux_arm
mkdir $BDIR/dist_linux_arm
cp $BDIR/bin/linux_arm/slim $BDIR/dist_linux_arm/slim
cp $BDIR/bin/linux_arm/slim-sensor $BDIR/dist_linux_arm/slim-sensor
pushd ${BDIR}/dist_linux_arm
ln -s slim docker-slim
popd
pushd ${BDIR}
tar -czvf dist_linux_arm.tar.gz dist_linux_arm
popd

rm -rfv $BDIR/dist_linux_arm64
mkdir $BDIR/dist_linux_arm64
cp $BDIR/bin/linux_arm64/slim $BDIR/dist_linux_arm64/slim
cp $BDIR/bin/linux_arm64/slim-sensor $BDIR/dist_linux_arm64/slim-sensor
pushd ${BDIR}/dist_linux_arm64
ln -s slim docker-slim
popd
pushd ${BDIR}
tar -czvf dist_linux_arm64.tar.gz dist_linux_arm64
popd

rm -rfv ${BDIR}/bin
