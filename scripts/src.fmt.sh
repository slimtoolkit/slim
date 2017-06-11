#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
SDIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

source $SDIR/env.sh
cd $BDIR/apps
gofmt -l -w -s .
cd $BDIR/master
gofmt -l -w -s .
cd $BDIR/messages
gofmt -l -w -s .
cd $BDIR/sensor
gofmt -l -w -s .
cd $BDIR/report
gofmt -l -w -s .
cd $BDIR/utils
gofmt -l -w -s .
cd $BDIR/consts
gofmt -l -w -s .

