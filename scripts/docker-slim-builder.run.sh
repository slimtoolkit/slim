#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
BDIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

cd $BDIR
docker run -v `pwd`:/go/src/github.com/docker-slim/docker-slim -w /go/src/github.com/docker-slim/docker-slim -it --rm --name="docker-slim-builder" golang:1.13 ./scripts/src.deps.and.build.sh

