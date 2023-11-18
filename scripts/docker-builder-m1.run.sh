#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
BDIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

pushd $BDIR
docker run -v $(pwd):/go/src/github.com/slimtoolkit/slim -w /go/src/github.com/slimtoolkit/slim -it --rm --name="slim-builder" golang:1.21 make build_m1

if [ ! -f dist_mac_m1.zip ]; then
if hash zip 2> /dev/null; then
	zip -r dist_mac_m1.zip dist_mac_m1 -x "*.DS_Store"
fi
fi
