#!/usr/bin/env bash

set -e

docker build --squash -t docker-slim -f Dockerfile ../../..

