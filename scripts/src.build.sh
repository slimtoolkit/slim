#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
BDIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

export CGO_ENABLED=0

TARGET_PLATFORMS=${TARGET_PLATFORMS:-"linux_amd64 linux_arm64 linux_arm darwin_amd64 darwin_arm64"}
REBUILD=${REBUILD:-}

pushd $BDIR

BUILD_TIME="$(date -u '+%Y-%m-%d_%I:%M:%S%p')"
TAG="current"
REVISION="current"
if hash git 2>/dev/null && [ -e $BDIR/.git ]; then
  TAG="$(git describe --tags)"
  REVISION="$(git rev-parse HEAD)"
fi

LD_FLAGS="-s -w -X github.com/docker-slim/docker-slim/pkg/version.appVersionTag=${TAG} -X github.com/docker-slim/docker-slim/pkg/version.appVersionRev=${REVISION} -X github.com/docker-slim/docker-slim/pkg/version.appVersionTime=${BUILD_TIME}"
BUILD_FLAGS="-mod=vendor -trimpath"
if [ -n "$REBUILD" ]; then
  BUILD_FLAGS="$BUILD_FLAGS -a"
fi

for platform in $TARGET_PLATFORMS; do

  os="$(echo $platform | awk -F_ '{print $1}')"
  arch="$(echo $platform | awk -F_ '{print $2}')"

  echo "building target ${os}/${arch}"

  pushd ${BDIR}/cmd/docker-slim >/dev/null
  GOOS=$os GOARCH=$arch go build $BUILD_FLAGS -tags 'netgo osusergo' -ldflags="${LD_FLAGS}" -o "${BDIR}/bin/${platform}/docker-slim"
  popd >/dev/null

  if [ ! -f "bin/linux_${arch}/docker-slim-sensor" ]; then
    pushd "${BDIR}/cmd/docker-slim-sensor" >/dev/null
    GOOS=linux GOARCH=$arch go build $BUILD_FLAGS -tags 'netgo osusergo' -ldflags="${LD_FLAGS}" -o "${BDIR}/bin/linux_${arch}/docker-slim-sensor"
    chmod a+x "${BDIR}/bin/linux_${arch}/docker-slim-sensor"
    popd >/dev/null
  fi

  rm -rf dist_${platform}
  mkdir dist_${platform}
  cp bin/${platform}/docker-slim dist_${platform}/docker-slim

  if [ "$os" = "darwin" ]; then
    # Use the linux arch since linux OS will be emulated but not arcsh.
    cp bin/linux_${arch}/docker-slim-sensor dist_${platform}/docker-slim-sensor
    if which zip >/dev/null 2>&1; then
      zip -r dist_${platform}.zip dist_${platform} -x "*.DS_Store"
    else
      echo "zip binary not found, skipping zip archiving of dist_${platform}"
    fi
  else
    cp bin/${platform}/docker-slim-sensor dist_${platform}/docker-slim-sensor
    tar --exclude="*.DS_Store" -czvf dist_${platform}.tar.gz dist_${platform}
  fi

done

rm -rf ./bin

popd >/dev/null
