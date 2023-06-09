#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
BDIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"
GOBIN=${GOBIN:=$(go env GOBIN)}

if [ -z "$GOBIN" ]; then
    GOBIN="$(go env GOPATH)/bin"
fi

GOVULNCHECK="${GOBIN}/govulncheck"

if [ ! -f "$GOVULNCHECK" ]; then
    echo "Tools: No govulncheck. Installing...."
    go install golang.org/x/vuln/cmd/govulncheck@latest
fi

pushd $BDIR
$GOVULNCHECK ./...
popd
