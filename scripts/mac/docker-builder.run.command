#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here/..

./docker-builder.run.sh

