#!/usr/bin/env bash

set -e

source env.sh
cd $BDIR
eval "$(docker-machine env default)"
docker run -v `pwd`:/go/src/github.com/docker-slim/docker-slim -w /go/src/github.com/docker-slim/docker-slim/scripts -it --rm --name="docker-slim-builder" my/docker-slim-builder ./src.deps.and.build.sh

