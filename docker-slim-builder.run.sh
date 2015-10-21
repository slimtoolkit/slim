#!/usr/bin/env bash

set -e

eval "$(docker-machine env default)"
docker run -v `pwd`:/go/src/github.com/cloudimmunity/docker-slim -w /go/src/github.com/cloudimmunity/docker-slim -it --rm --name="docker-slim-builder" my/docker-slim-builder ./src.deps.and.build.sh

