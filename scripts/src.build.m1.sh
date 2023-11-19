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
GOOS=darwin GOARCH=arm64 go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "${BDIR}/bin/mac_m1/slim"
popd

pushd ${BDIR}/cmd/slim-sensor
GOOS=linux GOARCH=arm64 go build -mod=vendor -trimpath -ldflags="${LD_FLAGS}" -a -tags 'netgo osusergo' -o "$BDIR/bin/linux_arm64/slim-sensor"
chmod a+x "$BDIR/bin/linux_arm64/slim-sensor"
popd

rm -rfv ${BDIR}/dist_mac_m1
mkdir ${BDIR}/dist_mac_m1
cp ${BDIR}/bin/mac_m1/slim ${BDIR}/dist_mac_m1/slim
cp ${BDIR}/bin/linux_arm64/slim-sensor ${BDIR}/dist_mac_m1/slim-sensor
pushd ${BDIR}/dist_mac_m1
ln -s slim docker-slim
popd
pushd ${BDIR}

if hash zip 2> /dev/null; then
	zip -r dist_mac_m1.zip dist_mac_m1 -x "*.DS_Store"
fi

rm -rfv ${BDIR}/bin
