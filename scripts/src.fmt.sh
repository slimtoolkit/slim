#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
SDIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

source $SDIR/env.sh
cd $BDIR/apps
go fmt ./...
cd $BDIR/master
go fmt ./...
cd $BDIR/sensor
go fmt ./...
cd $BDIR/report
go fmt ./...
cd $BDIR/utils
go fmt ./...


