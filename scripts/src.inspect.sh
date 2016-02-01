#!/usr/bin/env bash

set -e

source env.sh
cd $BDIR/apps
go tool vet .
golint ./...
cd $BDIR/master
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



