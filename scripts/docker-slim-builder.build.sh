#!/usr/bin/env bash

set -e

source env.sh
cd $BDIR
#docker-machine start default
eval "$(docker-machine env default)"
docker build -t my/docker-slim-builder .



