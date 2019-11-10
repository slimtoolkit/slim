#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here
echo `pwd`
ls -lh

docker build -t docker-slim -f Dockerfile ../../..


