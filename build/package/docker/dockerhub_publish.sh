#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
BDIR="$( cd -P "$( dirname "$SOURCE" )/../../.." && pwd )"

TAG="current"
pushd $BDIR
if hash git 2>/dev/null && [ -e $BDIR/.git ]; then
  TAG="$(git describe --tags)"
fi
popd

docker tag slim dslim/slim:$TAG
docker tag slim dslim/slim
docker push dslim/slim:$TAG
docker push dslim/slim
