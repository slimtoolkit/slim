#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
SDIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

source $SDIR/env.sh
cd $BDIR/cmd
go tool vet .
golint ./...
cd $BDIR/consts
go tool vet .
golint ./...
cd $BDIR/master
go tool vet .
golint ./...
cd $BDIR/messages
go tool vet .
golint ./...
cd $BDIR/sensor
go tool vet .
golint ./...
cd $BDIR/report
go tool vet .
golint ./...
cd $BDIR/utils
go tool vet .
golint ./...

