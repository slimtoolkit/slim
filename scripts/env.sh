#!/usr/bin/env bash
#source env.sh or . env.sh

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
BDIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

export GOPATH=$BDIR/_gopath
export PATH=$PATH:$GOPATH/bin


