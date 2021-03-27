#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
BDIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

pushd ${BDIR}/cmd
gofmt -l -w -s .
popd
pushd ${BDIR}/pkg
gofmt -l -w -s .
popd
