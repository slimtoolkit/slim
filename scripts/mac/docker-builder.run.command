#!/usr/bin/env bash

here="$(dirname "$BASH_SOURCE")"
cd $here/..

if [ -z "$TARGET_PLATFORMS" ]; then
    export TARGET_PLATFORMS=darwin_$(go env GOARCH)
fi

./docker-builder.run.sh
