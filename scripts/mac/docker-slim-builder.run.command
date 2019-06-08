#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here/..

./docker-slim-builder.run.sh

