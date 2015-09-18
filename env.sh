#!/usr/bin/env bash
#source env.sh or . env.sh

here="$(pwd)"

export GOPATH=$here/_vendor:$here
export PATH=$PATH:$GOPATH/bin


