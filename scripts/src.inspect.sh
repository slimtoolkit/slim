#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
BDIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

export GOOS=linux
export GOARCH=amd64 

pushd ${BDIR}/cmd
go vet ./...
go vet -vettool=$(which shadow) ./...
golint ./...
popd
pushd ${BDIR}/pkg
go vet ./...
go vet -vettool=$(which shadow) ./...
golint ./...
popd
