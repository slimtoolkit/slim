#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
SDIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

source $SDIR/env.sh

echo "installing bom tool (note: requires Go 1.8+)"
go get -v -u github.com/cloudimmunity/license-bill-of-materials

license-bill-of-materials github.com/docker-slim/docker-slim/apps/docker-slim > $SDIR/../lib-licenses.json
