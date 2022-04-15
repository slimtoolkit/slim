#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
BDIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

pushd $BDIR
docker run \
	-v $(pwd):/project/docker-slim \
	-w /project/docker-slim \
	-e TARGET_PLATFORMS=${TARGET_PLATFORMS} \
	-it --rm --name="docker-slim-builder" golang:1.16 \
	make build
popd
