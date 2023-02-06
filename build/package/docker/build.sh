#!/usr/bin/env bash

set -e

docker build --squash --rm -t slim -f Dockerfile ../../..
docker image prune --filter label=build-role=ca-certs -f
docker image prune --filter label=app=slim -f