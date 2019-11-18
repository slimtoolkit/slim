#!/usr/bin/env bash

set -e

docker build --squash --rm -t docker-slim -f Dockerfile ../../..
docker image prune --filter label=build-role=ca-certs -f
docker image prune --filter label=app=docker-slim -f