#!/usr/bin/env bash

set -e

#docker-machine start default
eval "$(docker-machine env default)"
docker build -t my/docker-slim-builder .



