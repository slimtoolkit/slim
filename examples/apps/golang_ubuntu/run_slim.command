#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here
docker run --rm -it my/golang-app-ubuntu.slim
