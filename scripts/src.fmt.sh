#!/usr/bin/env bash

set -e

source env.sh
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



